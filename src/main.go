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

	http.HandleFunc("/", onlyHandler)
	http.HandleFunc("/stats", internal.StatsHandler)

	fmt.Printf("running on port %s\n", server.Port)
	http.ListenAndServe(":"+server.Port, nil)

	// (4) (marco/tyler)
	// update python test script to use multi theading

	// (5) (marco/tyler)
	// write the python server application code that runs in docker, should be simple
	// and needs to frequenly update redis stats

	// (6)
	// end to end test,
	// send request to proxy, proxy -> server , server -> proxy, proxy -> client
	// latency is track, each servers latency/connections/ect is tracked and updated frequently in redis
	// < 15 ms of added latency total

}
