package main

import (
	"fmt"
	"log"
	"net/http"
	"strconv"
	"sync"
)

var (
	serverNum = 5
	port      = 23000
	probePort = 43000
)

func main() {
	var wg sync.WaitGroup
	for i := 0; i < serverNum; i++ {
		addr := "127.0.0.1:" + strconv.Itoa(port)
		probe := "127.0.0.1:" + strconv.Itoa(probePort)
		port++
		probePort++
		wg.Add(2)
		// Start Server
		go func(addr, probe string) {
			fmt.Printf("start server %s, probe addr:%s\n", addr, probe)
			err := http.ListenAndServe(addr, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				fmt.Printf("server %s, receive request from:%s\n", addr, r.RemoteAddr)
				w.WriteHeader(http.StatusOK)
				_, err := w.Write([]byte("this is server:" + addr))
				if err != nil {
					log.Fatal(err)
				}
			}))
			wg.Done()
			if err != nil {
				log.Fatal(err)
			}
		}(addr, probe)

		// Start probe
		go func(addr, probe string) {
			err := http.ListenAndServe(probe, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				fmt.Printf("server %s, receive health check...\n", addr)
				w.WriteHeader(http.StatusOK)
			}))
			wg.Done()
			if err != nil {
				log.Fatal(err)
			}
		}(addr, probe)
	}
	wg.Wait()
}
