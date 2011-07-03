package main

import (
	"fmt"
	"http"
	"log"
	"os"
	"rpc"
	"strings"
	"sync"
)

const listenAddr = ":5001"

type Master struct {
	blocked map[string]bool
	mu sync.RWMutex
}

func (m *Master) Validate(b []byte, ok *bool) os.Error {
	m.mu.RLock()
	defer m.mu.RUnlock()
	s := string(b)
	*ok = true
	for domain := range m.blocked {
		if strings.HasSuffix(domain, s) {
			*ok = false
		}
	}
	log.Println(*ok, s)
	return nil
}

func (m *Master) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintf(w, "Blocked hosts: %v", m.blocked)
}

func main() {
	m := &Master{blocked: make(map[string]bool)}
	m.blocked["webkinz.com"] = true
	rpc.Register(m)
	rpc.HandleHTTP()
	http.Handle("/", m)
	log.Fatal(http.ListenAndServe(listenAddr, nil))
}
