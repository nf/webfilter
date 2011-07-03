package main

import (
	"http"
	"log"
	"os"
	"rpc"
	"strconv"
	"strings"
	"sync"
	"template"
	"time"
)

const listenAddr = ":5001"

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

func (m *Master) Add(suffix string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.Hosts = append(m.Hosts, &Host{Suffix: suffix})
}

func (m *Master) Open(suffix string, mins int) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, h := range m.Hosts {
		if h.Suffix == suffix {
			log.Println("open", h.Suffix)
			h.CloseTime = time.Nanoseconds() + int64(mins)*60e9
			return
		}
	}
}

func (m *Master) Close(suffix string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, h := range m.Hosts {
		if h.Suffix == suffix {
			log.Println("close", h.Suffix)
			h.CloseTime = 0
			return
		}
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
		m.Add(s)
	case "/open":
		mins, err := strconv.Atoi(r.FormValue("mins"))
		if err != nil {
			http.Error(w, err.String(), 300)
		}
		m.Open(s, mins)
	case "/close":
		m.Close(s)
	default:
		http.Error(w, "not found", 404)
		return
	}
	http.Redirect(w, r, "/admin/", http.StatusFound)
}

func main() {
	m := &Master{}
	rpc.Register(m)
	rpc.HandleHTTP()
	http.Handle("/admin/", m)
	log.Fatal(http.ListenAndServe(listenAddr, nil))
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
