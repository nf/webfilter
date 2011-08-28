package main

import (
	"bufio"
	"flag"
	"log"
	"os"
	"rpc"
)

var masterAddr = flag.String("master", "127.0.0.1:5001", "master server address")

func main() {
	flag.Parse()
	client, err := rpc.DialHTTP("tcp", *masterAddr)
	if err != nil {
		log.Fatal("Dial:", err)
	}
	in := bufio.NewReader(os.Stdin)
	for {
		host, _, err := in.ReadLine()
		if err != nil {
			log.Fatal("Read:", err)
		}
		var ok bool
		err = client.Call("Master.Validate", host, &ok)
		if err != nil {
			log.Fatal("Call:", err)
		}
		if ok {
			os.Stdout.Write([]byte("OK\n"))
		} else {
			os.Stdout.Write([]byte("ERR\n"))
		}
	}
}
