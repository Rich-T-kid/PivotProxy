package main

import (
	"PivotProxy/internal"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/joho/godotenv"
)

func init() {
	godotenv.Load()
}
func main() {
	workerChannel := make(chan internal.RawRequest, 1024)

	server, lb := internal.InitApplicationServers()
	go internal.HandleStream(lb, workerChannel, server)

	onlyHandler := func(w http.ResponseWriter, r *http.Request) {
		content, _ := io.ReadAll(r.Body)
		killChan := make(chan struct{})
		workerChannel <- internal.RawRequest{W: w, R: r, Content: content, KillChan: killChan}
		<-killChan // wait for signal to return from functions
		print("Exiting now")
	}
	healthHandler := func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}
	serverLive := func(w http.ResponseWriter, r *http.Request) {
		table := make(map[int]bool, 0)
		for i, s := range server.Servers {
			if internal.PingServer(s.Url) {
				table[i+1] = true
			} else {
				table[i+1] = false
			}
		}
		json.NewEncoder(w).Encode(table)
	}

	http.HandleFunc("/", http.HandlerFunc(onlyHandler))
	http.HandleFunc("/stats", internal.StatsHandler)
	http.HandleFunc("/health", healthHandler)
	http.HandleFunc("/serversHealth", serverLive)
	fmt.Printf("running on port %s\n", server.Proxy_port)
	http.ListenAndServe(":"+server.Proxy_port, internal.RateLimter(http.DefaultServeMux))
}
