package main

import (
	"bytes"
	"fmt"
	"hash/maphash"
	"io"
	"math/rand"
	"net/http"
	"os"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

const (
	communicationPort = "8989"
)

type ApplicationsServers struct {
	Url    string  `yaml:"ip_address"`
	Name   string  `yaml:"name"`
	weight float64 `yaml:"weight"` // scale 1-10
}

func testApplicationServer(url, name string) *ApplicationsServers {
	return &ApplicationsServers{
		Url:  url,
		Name: name,
	}
}

type coreRequest struct {
	url     string
	headers http.Header
	body    []byte
}

func newRequest(url string, headers http.Header, body []byte) coreRequest {
	return coreRequest{
		url:     "http://" + url + ":" + communicationPort,
		headers: headers,
		body:    body,
	}
}

// (tbd) this should be more of an enum kind of struct/object
type status struct {
	issue string
}

func newHttpClient() *http.Client {
	return &http.Client{
		Timeout: time.Second * 2,
	}
}
func (a *ApplicationsServers) sendRequest(clonedRequest coreRequest) (coreRequest, status) {
	reader := bytes.NewReader(clonedRequest.body)
	req, err := http.NewRequest("GET", clonedRequest.url, reader)
	if err != nil {
		return coreRequest{}, status{issue: err.Error()}
	}
	for k, v := range clonedRequest.headers {
		for _, v := range v {
			req.Header.Add(k, v)

		}
	}
	client := newHttpClient()
	resp, err := client.Do(req)
	if err != nil {
		return coreRequest{}, status{issue: err.Error()}
	}
	content, err := io.ReadAll(resp.Body)
	if err != nil {
		return coreRequest{}, status{issue: err.Error()}
	}
	return newRequest(resp.Request.URL.String(), resp.Header, content), status{issue: "no issue"}

}

type serverConfig struct {
	Servers      []ApplicationsServers `yaml:"servers"`
	LoadBalencer string                `yaml:"algorithm"`
}

func (s *serverConfig) parseAlgorithm() loadbalencer {
	switch strings.ToLower(s.LoadBalencer) {
	case "round robin":
		return &roundRobin{}
	case "weighted round robin":
		return &weightedRobin{}
	case "url hash":
		return &urlHash{}
	case "random":
		return &random{}
	case "least connections":
		return &leastConnections{}
	case "weighted least connections":
		return &weightedLC{}
	case "least response time":
		return &responseTime{}
	default:
		fmt.Printf("unknown load balancing algorithm %s, defaulting to round robin\n", s.LoadBalencer)
		return &roundRobin{}
	}
}

func initApplicationServers() (*serverConfig, loadbalencer) {
	f, err := os.ReadFile("config/servers.yaml")
	if err != nil {
		panic(err)
	}
	var s serverConfig
	if err := yaml.Unmarshal(f, &s); err != nil {
		panic(err)
	}
	return &s, s.parseAlgorithm()

}

type rawRequest struct {
	w http.ResponseWriter
	r *http.Request
}

func writeResponse(w http.ResponseWriter, resp coreRequest) {
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(resp.body)
	for k, v := range resp.headers {
		for _, v := range v {
			w.Header().Add(k, v)
		}
	}
}
func unwrapBody(r *io.ReadCloser) []byte {
	defer (*r).Close()
	v, _ := io.ReadAll(*r)
	return v
}

type loadbalencer interface {
	balence(request chan rawRequest, servers *serverConfig)
	name() string
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

func (r *roundRobin) balence(request chan rawRequest, servers *serverConfig) {
	var i int
	for req := range request {
		i++
		idx := i % len(servers.Servers)
		proxyServer := servers.Servers[idx]
		fmt.Printf("%s is attempting to proccess a request %v\n ", proxyServer.Name, req)
		fwRequest := newRequest(proxyServer.Url, req.r.Header, unwrapBody(&req.r.Body))
		resp, _ := proxyServer.sendRequest(fwRequest)
		fmt.Printf("Application servers responded with %v", string(resp.body))
		writeResponse(req.w, resp)
	}

}
func (r *roundRobin) name() string {
	return "round robin"
}

type weightedRobin struct{}

func (w *weightedRobin) balence(request chan rawRequest, servers *serverConfig) {}
func (w *weightedRobin) name() string {
	return "weighted round robin"
}

type urlHash struct{}

func (u *urlHash) balence(request chan rawRequest, servers *serverConfig) {
	var h maphash.Hash
	for req := range request {
		h.WriteString(req.r.URL.String())
		hashVal := h.Sum64()
		idx := hashVal % uint64(len(servers.Servers))
		proxyServer := servers.Servers[idx]
		fwRequest := newRequest(proxyServer.Url, req.r.Header, unwrapBody(&req.r.Body))
		resp, _ := proxyServer.sendRequest(fwRequest)
		fmt.Printf("Application server %s responded with %v", proxyServer.Name, string(resp.body))
		writeResponse(req.w, resp)

	}
}
func (u *urlHash) name() string {
	return "url hash"
}

type random struct{}

func (r *random) balence(request chan rawRequest, servers *serverConfig) {
	for req := range request {
		n := rand.Intn(100)
		idx := n % len(servers.Servers)
		proxyServer := servers.Servers[idx]
		fwRequest := newRequest(proxyServer.Url, req.r.Header, unwrapBody(&req.r.Body))
		resp, _ := proxyServer.sendRequest(fwRequest)
		fmt.Printf("Application server %s responded with %v", proxyServer.Name, string(resp.body))
		writeResponse(req.w, resp)

	}
}
func (r *random) name() string {
	return "random"
}

// dynamic algorithms
type leastConnections struct{}

func (lc *leastConnections) balence(request chan rawRequest, servers *serverConfig) {}
func (lc *leastConnections) name() string {
	return "least connections"
}

type weightedLC struct{}

func (wlc *weightedLC) balence(request chan rawRequest, servers *serverConfig) {}
func (wlc *weightedLC) name() string {
	return "weighted least connections"
}

type responseTime struct{}

func (rp *responseTime) balence(request chan rawRequest, servers *serverConfig) {}
func (rp *responseTime) name() string {
	return "response time"
}
func handleStream(lb loadbalencer, requestStream chan rawRequest, servers *serverConfig) {
	lb.balence(requestStream, servers)
}
func main() {
	// this is a catch all route
	workerChannel := make(chan rawRequest, 1024)
	onlyHandler := func(w http.ResponseWriter, r *http.Request) {
		workerChannel <- rawRequest{w: w, r: r}
	}
	http.HandleFunc("/", onlyHandler)
	s, algo := initApplicationServers()
	fmt.Printf("working with the %+v load balancing algorithm\n", algo.name())
	go handleStream(algo, workerChannel, s)

	fmt.Printf("running on port :79 \n ")
	panic("")
	http.ListenAndServe(":79", nil)

}
