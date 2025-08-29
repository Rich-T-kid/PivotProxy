package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"hash/maphash"
	"io"
	"math/rand"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/joho/godotenv"
	"github.com/redis/go-redis/v9"
	"gopkg.in/yaml.v3"
)

const (
	communicationPort = "8989"
)

var (
	cache *redis.Client
	once  sync.Once
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
	Port         string                `yaml:"port"`
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
func stats(w http.ResponseWriter, r *http.Request) {
	// tbd implement stats endpoint
	metrics := newMetricsLogger()
	for {
		// just read from redis instance and transform to json and transmit it
		w.Write([]byte("stats endpoint is under construction"))
		v := metrics.stats()
		err := json.NewEncoder(w).Encode(v)
		if err != nil {
			fmt.Println(err)
		}
		time.Sleep(5 * time.Second) // Reduce this very heavily

	}
}

// need to add wrapper methods on this for easier logging
func cacheClient() *redis.Client {

	once.Do(func() {
		ctx := context.Background()
		rdb := redis.NewClient(&redis.Options{
			Addr:     os.Getenv("REDIS_ADDR"),
			Username: os.Getenv("REDIS_USER"),
			Password: os.Getenv("REDIS_PASSWORD"),
			DB:       0,
		})
		if _, err := rdb.Ping(ctx).Result(); err != nil {
			panic(err)
		}
		cache = rdb
	})
	return cache
}

type metricsLogger struct {
	mu      sync.RWMutex
	storage *redis.Client
}

func newMetricsLogger() *metricsLogger {
	return &metricsLogger{
		storage: cacheClient(),
	}
}

// server_1 INCR
func (m *metricsLogger) requestHit(name string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	ctx := context.Background()
	m.storage.Incr(ctx, fmt.Sprintf("%s:requestCount", name))
}

// time taken for server_1 to complete the request and send results back
func (m *metricsLogger) responseTimeLog(name string, time float64) {
	m.mu.Lock()
	defer m.mu.Unlock()
	ctx := context.Background()
	m.storage.LPush(ctx, fmt.Sprintf("%s:responseTimes", name), time)
}

// number of connections that the current server is working with
func (m *metricsLogger) activeConnections(name string) uint {
	m.mu.RLock()
	defer m.mu.RUnlock()
	ctx := context.Background()
	val, _ := m.storage.Get(ctx, fmt.Sprintf("%s:activeConnections", name)).Uint64()
	return uint(val)
}

// increment current connection count
func (m *metricsLogger) addConnection(name string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	ctx := context.Background()
	m.storage.Incr(ctx, fmt.Sprintf("%s:activeConnections", name))
}

// remove connection from current count to keep it updated
func (m *metricsLogger) removeConnection(name string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	ctx := context.Background()
	m.storage.Decr(ctx, fmt.Sprintf("%s:activeConnections", name))
}
func (m *metricsLogger) averageResponseTime(name string) float64 {
	m.mu.RLock()
	defer m.mu.RUnlock()
	ctx := context.Background()
	val, _ := m.storage.LRange(ctx, fmt.Sprintf("%s:responseTimes", name), 0, -1).Result()
	var total float64
	for _, v := range val {
		if f, err := strconv.ParseFloat(v, 64); err == nil {
			total += f
		}
	}
	return total / float64(len(val))
}
func (m *metricsLogger) stats() interface{} {
	return 0
}
func init() {
	godotenv.Load()
}
func main() {
	cacheClient()
	// this is a catch all route
	/*workerChannel := make(chan rawRequest, 1024)
	onlyHandler := func(w http.ResponseWriter, r *http.Request) {
		workerChannel <- rawRequest{w: w, r: r}
	}*/
	// (1)
	// X tbd set up redis instance and manage the server state
	// Xread from redis instance for server state
	// Xset up an endpoint to showcase the server stats

	// (2)
	// X add logging for everything from, who sent what request
	// X what server is handeling what request
	// X just basic stats that can be used wih INCR and its vice versa
	// X we can run aggreagtaions on this stuff later if we want to

	// (3)
	// make all the IP addresses that this proxy uses the private IP addrerss on the aws VPS

	// (4)
	// update python test script to use multi theading

	// (5)
	// write the python server application code that runs in docker, should be simple
	// and needs to frequenly update redis stats

	// (6)
	// end to end test,
	// send request to proxy, proxy -> server , server -> proxy, proxy -> client
	// latency is track, each servers latency/connections/ect is tracked and updated frequently in redis
	// < 15 ms of added latency total
	//
	/*http.HandleFunc("/stats", stats)

	http.HandleFunc("/", onlyHandler)

	s, algo := initApplicationServers()
	fmt.Printf("working with the %+v load balancing algorithm\n", algo.name())
	go handleStream(algo, workerChannel, s)

	fmt.Printf("running on port :79 \n ")
	http.ListenAndServe(":79", nil)*/

}
