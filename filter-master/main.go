package main

import (
	"flag"
	"http"
	"json"
	"log"
	"os"
	"rpc"
	"strconv"
	"strings"
	"sync"
	"template"
	"time"
)

var (
	listenAddr = flag.String("addr", ":5001", "HTTP/RPC listen address")
	logFile    = flag.String("log", "/var/log/webfilter.log",
		"log file path")
	configFile = flag.String("config", "/usr/local/etc/webfilter.conf",
		"configuration file")
)

func main() {
	flag.Parse()

	// Log to the specified file.
	f, err := os.OpenFile(*logFile, os.O_WRONLY|os.O_APPEND|os.O_CREATE, 0644)
	if err != nil {
		log.Fatal(err)
	}
	log.SetOutput(f)

	// Set up RPC and HTTP servers.
	m := &Master{}
	m.loadConfig()
	rpc.RegisterName("Master", RPCMaster{m})
	rpc.HandleHTTP()
	http.Handle("/admin/", m)
	log.Fatal(http.ListenAndServe(*listenAddr, nil))
}

type RPCMaster struct {
	m *Master
}

func (m RPCMaster) Validate(b []byte, ok *bool) os.Error {
	return m.m.Validate(b, ok)
}

type Host struct {
	Suffix    string
	CloseTime int64
}

func (h *Host) Match(host string) bool {
	return strings.HasSuffix(host, h.Suffix)
}

func (h *Host) Closed() bool {
	return h.CloseTime < time.Nanoseconds()
}

func (h *Host) MinsRemaining() int {
	return int((h.CloseTime - time.Nanoseconds()) / 60e9)
}

type Master struct {
	Hosts []*Host
	mu    sync.RWMutex
}

func (m *Master) Validate(b []byte, ok *bool) os.Error {
	m.mu.RLock()
	defer m.mu.RUnlock()
	s := string(b)
	*ok = true
	for _, host := range m.Hosts {
		if host.Match(s) && host.Closed() {
			*ok = false
			break
		}
	}
	log.Println(*ok, s)
	return nil
}

func (m *Master) add(suffix string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.Hosts = append(m.Hosts, &Host{Suffix: suffix})
	m.saveConfig()
}

func (m *Master) open(suffix string, mins int) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, h := range m.Hosts {
		if h.Suffix == suffix {
			log.Println("open", h.Suffix)
			h.CloseTime = time.Nanoseconds() + int64(mins)*60e9
			m.saveConfig()
			return
		}
	}
}

func (m *Master) close(suffix string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, h := range m.Hosts {
		if h.Suffix == suffix {
			log.Println("close", h.Suffix)
			h.CloseTime = 0
			m.saveConfig()
			return
		}
	}
}

func (m *Master) saveConfig() {
	f, err := os.Create(*configFile)
	if err != nil {
		log.Println(err)
		return
	}
	defer f.Close()
	err = json.NewEncoder(f).Encode(m.Hosts)
	if err != nil {
		log.Println(err)
	}
}

func (m *Master) loadConfig() {
	f, err := os.Open(*configFile)
	if err != nil {
		log.Println(err)
		return
	}
	defer f.Close()
	err = json.NewDecoder(f).Decode(&m.Hosts)
	if err != nil {
		log.Println(err)
	}
}

var tmpl = template.MustParse(html, nil)

func (m *Master) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s := r.FormValue("suffix")
	switch r.URL.Path[len("/admin"):] {
	case "/":
		m.mu.RLock()
		defer m.mu.RUnlock()
		err := tmpl.Execute(w, m)
		if err != nil {
			http.Error(w, err.String(), 500)
		}
		return
	case "/add":
		m.add(s)
	case "/open":
		mins, err := strconv.Atoi(r.FormValue("mins"))
		if err != nil {
			http.Error(w, err.String(), 300)
		}
		m.open(s, mins)
	case "/close":
		m.close(s)
	default:
		http.Error(w, "not found", 404)
		return
	}
	http.Redirect(w, r, "/admin/", http.StatusFound)
}

const html = `
<form method="POST" action="/admin/add">
<input type="text" name="suffix">
<input type="submit" value="Add">
</form>

<table>
<tr><th>Host</th><th>State</th><th>Action</th></tr>
{.repeated section Hosts}
<tr>
	<td>{Suffix|html}</td>
	<td>{.section Closed}Closed{.or}Open ({MinsRemaining} mins){.end}</td>
	<td>
	{.section Closed}
		<form method="POST" action="/admin/open">
		<input type="hidden" name="suffix" value="{Suffix|html}">
		<input type="input" name="mins" value="30">
		<input type="submit" value="Open">
		</form>
	{.or}
		<form method="POST" action="/admin/close">
		<input type="hidden" name="suffix" value="{Suffix|html}">
		<input type="submit" value="Close">
		</form>
	{.end}
	</td>
</tr>
{.end}
</table>
`
