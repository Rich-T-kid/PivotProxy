package main

import (
	"PivotProxy/internal"
	"fmt"
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
		workerChannel <- internal.RawRequest{W: w, R: r}
	}

	http.HandleFunc("/", http.HandlerFunc(onlyHandler))
	http.HandleFunc("/stats", internal.StatsHandler)

	fmt.Printf("running on port %s\n", server.Proxy_port)
	http.ListenAndServe(":"+server.Proxy_port, internal.RateLimter(http.DefaultServeMux))

}
