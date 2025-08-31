package internal

import (
	"bytes"
	"fmt"
	"io"
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
	Url  string `yaml:"ip_address"`
	Name string `yaml:"name"`
}

type coreRequest struct {
	url     string
	headers http.Header
	body    []byte
}

func toProperURL(url string) string {
	return "http://" + url + ":" + communicationPort
}
func newRequest(url string, headers http.Header, body []byte) coreRequest {
	return coreRequest{
		url:     toProperURL(url) + "/process",
		headers: headers,
		body:    body,
	}
}

func newHttpClient(seconds float64) *http.Client {
	return &http.Client{
		Timeout: time.Second * time.Duration(seconds),
	}
}
func (a *ApplicationsServers) sendRequest(clonedRequest coreRequest, metLog *metricsLogger) (coreRequest, float64, error) {
	reader := bytes.NewReader(clonedRequest.body)
	req, err := http.NewRequest("POST", clonedRequest.url, reader)
	if err != nil {
		return coreRequest{}, 0, err
	}
	for k, v := range clonedRequest.headers {
		for _, v := range v {
			req.Header.Add(k, v)

		}
	}
	client := newHttpClient(2)
	start := time.Now()
	metLog.addConnection(a.Name) //log
	resp, err := client.Do(req)
	responseTime := time.Since(start).Seconds() * 1000 // in ms
	metLog.removeConnection(a.Name)                    //log
	if err != nil {
		return coreRequest{}, 0, err
	}
	content, err := io.ReadAll(resp.Body)
	if err != nil {
		return coreRequest{}, 0, err
	}
	return newRequest(resp.Request.URL.String(), resp.Header, content), responseTime, nil

}

type serverConfig struct {
	Proxy_ip           string                `yaml:"Proxy_ip"`
	Proxy_port         string                `yaml:"Proxy_port"`
	Servers            []ApplicationsServers `yaml:"servers"`
	LoadBalencer       string                `yaml:"algorithm"`
	Communication_port string                `yaml:"Communication_port"`
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
	case "least response time":
		return &responseTime{}
	default:
		fmt.Printf("unknown load balancing algorithm %s, defaulting to round robin\n", s.LoadBalencer)
		return &roundRobin{}
	}
}

func InitApplicationServers() (*serverConfig, loadbalencer) {
	f, err := os.ReadFile("config/servers.yaml")
	if err != nil {
		panic(err)
	}
	var s serverConfig
	if err := yaml.Unmarshal(f, &s); err != nil {
		panic(err)
	}
	fmt.Println("there are " + fmt.Sprint(len(s.Servers)) + " servers configured")
	fmt.Printf("loaded server configuration: %+v\n", s.Servers)
	return &s, s.parseAlgorithm()

}

type RawRequest struct {
	W        http.ResponseWriter
	R        *http.Request
	Content  []byte
	KillChan chan struct{}
}

func writeResponse(w http.ResponseWriter, resp coreRequest) {
	for k, v := range resp.headers {
		for _, v := range v {
			w.Header().Add(k, v)
		}
	}
	_, _ = w.Write(resp.body)
}
func unwrapBody(r io.ReadCloser) []byte {
	defer r.Close()
	v, _ := io.ReadAll(r)
	fmt.Printf("recieved this from client -> %s\n", string(v))
	return v
}

func PingServer(url string) bool {
	url = toProperURL(url)
	fmt.Println("pinging server at " + url)
	client := newHttpClient(0.5)
	resp, err := client.Get(url)
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	return resp.StatusCode == http.StatusOK
}
