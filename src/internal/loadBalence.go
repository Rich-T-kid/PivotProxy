package internal

import (
	"fmt"
	"hash/maphash"
	"math/rand"
	"net/http"
	"time"
)

type loadbalencer interface {
	balence(request chan RawRequest, servers *serverConfig, metLog *metricsLogger)
	Name() string
}

/*
Static algorithms
	round robin
	weighted round robin
	URL Hash
	random select


Dynamic algorithms
	Least connections
		weighted least connections
	least response time

*/
// static algorithms
type roundRobin struct{}

func (r *roundRobin) balence(request chan RawRequest, servers *serverConfig, metLog *metricsLogger) {
	var i int
	for req := range request {
		proxystart := time.Now()
		i++
		idx := i % len(servers.Servers)
		proxyServer := servers.Servers[idx]
		fmt.Printf("(Round Robin) %s is attempting to proccess a request %v\n ", proxyServer.Name, req)
		fwRequest := newRequest(proxyServer.Url, req.R.Header, unwrapBody(&req.R.Body))
		resp, serverlatency, _ := proxyServer.sendRequest(fwRequest, metLog)
		writeResponse(req.W, resp)
		metLog.collectMetrics(proxyServer.Name, serverlatency, time.Since(proxystart).Seconds()*1000)
	}

}
func (r *roundRobin) Name() string {
	return "round robin"
}

type weightedRobin struct{}

func (w *weightedRobin) balence(request chan RawRequest, servers *serverConfig, metLog *metricsLogger) {
}
func (w *weightedRobin) Name() string {
	return "weighted round robin"
}

type urlHash struct{}

func (u *urlHash) balence(request chan RawRequest, servers *serverConfig, metLog *metricsLogger) {
	var h maphash.Hash
	for req := range request {
		proxystart := time.Now()
		h.WriteString(req.R.URL.String())
		hashVal := h.Sum64()
		idx := hashVal % uint64(len(servers.Servers))
		proxyServer := servers.Servers[idx]
		fwRequest := newRequest(proxyServer.Url, req.R.Header, unwrapBody(&req.R.Body))
		resp, serverlatency, _ := proxyServer.sendRequest(fwRequest, metLog)
		writeResponse(req.W, resp)
		metLog.collectMetrics(proxyServer.Name, serverlatency, time.Since(proxystart).Seconds()*1000)

	}
}
func (u *urlHash) Name() string {
	return "url hash"
}

type random struct{}

func (r *random) balence(request chan RawRequest, servers *serverConfig, metLog *metricsLogger) {
	for req := range request {
		proxystart := time.Now()
		n := rand.Intn(100)
		idx := n % len(servers.Servers)
		proxyServer := servers.Servers[idx]
		fwRequest := newRequest(proxyServer.Url, req.R.Header, unwrapBody(&req.R.Body))
		resp, serverlatency, _ := proxyServer.sendRequest(fwRequest, metLog)
		fmt.Printf("(Random) Application server %s responded with %v", proxyServer.Name, string(resp.body))
		writeResponse(req.W, resp)
		metLog.collectMetrics(proxyServer.Name, serverlatency, time.Since(proxystart).Seconds()*1000)

	}
}
func (r *random) Name() string {
	return "random"
}

// dynamic algorithms
type leastConnections struct{}

func (lc *leastConnections) balence(request chan RawRequest, servers *serverConfig, metLog *metricsLogger) {
}
func (lc *leastConnections) Name() string {
	return "least connections"
}

type weightedLC struct{}

func (wlc *weightedLC) balence(request chan RawRequest, servers *serverConfig, metLog *metricsLogger) {
}
func (wlc *weightedLC) Name() string {
	return "weighted least connections"
}

type responseTime struct{}

func (rp *responseTime) balence(request chan RawRequest, servers *serverConfig, metLog *metricsLogger) {
}
func (rp *responseTime) Name() string {
	return "response time"
}

func HandleStream(lb loadbalencer, workerChannel chan RawRequest, servers *serverConfig) {
	fmt.Printf("working with the %+v load balancing algorithm\n", lb.Name())
	lb.balence(workerChannel, servers, newMetricsLogger())
}

// sliding window log for rate limiting
func RateLimter(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// check for IP, headers, ect gotta check with team
		fmt.Println("rate limiting check passed")
		next.ServeHTTP(w, r)
	})

}
