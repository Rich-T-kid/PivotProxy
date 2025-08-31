package internal

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
)

var (
	cache *redis.Client
	once  sync.Once
)

func StatsHandler(w http.ResponseWriter, r *http.Request) {
	// tbd implement stats endpoint
	metrics := newMetricsLogger()
	// just read from redis instance and transform to json and transmit it
	v := metrics.stats()
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(v)

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
func (m *metricsLogger) proxyLatency(time float64) {
	m.mu.Lock()
	defer m.mu.Unlock()
	ctx := context.Background()
	m.storage.LPush(ctx, "proxyLatencies", time)
}
func (m *metricsLogger) collectMetrics(name string, serverTime float64, proxyLatency float64) {
	m.requestHit(name)
	m.responseTimeLog(name, serverTime)
	m.proxyLatency(serverTime)
}

func (m *metricsLogger) avgListFloat(key string) float64 {
	mu := 0.0
	ctx := context.Background()
	vals, err := m.storage.LRange(ctx, key, 0, -1).Result()
	if err != nil || len(vals) == 0 {
		return 0
	}
	for _, v := range vals {
		if f, err := strconv.ParseFloat(v, 64); err == nil {
			mu += f
		}
	}
	return mu / float64(len(vals))
}

func (m *metricsLogger) serverNames(ctx context.Context) []string {
	// Discover servers by scanning requestCount keys: serverN:requestCount
	var names []string
	iter := m.storage.Scan(ctx, 0, "server*:requestCount", 0).Iterator()
	for iter.Next(ctx) {
		k := iter.Val() // e.g., "server1:requestCount"
		if idx := strings.IndexByte(k, ':'); idx > 0 {
			names = append(names, k[:idx])
		}
	}
	// Optionally also scan activeConnections in case requestCount doesn't exist yet
	iter2 := m.storage.Scan(ctx, 0, "server*:activeConnections", 0).Iterator()
	for iter2.Next(ctx) {
		k := iter2.Val()
		if idx := strings.IndexByte(k, ':'); idx > 0 {
			name := k[:idx]
			found := false
			for _, n := range names {
				if n == name {
					found = true
					break
				}
			}
			if !found {
				names = append(names, name)
			}
		}
	}
	return names
}

func (m *metricsLogger) stats() map[string]interface{} {
	m.mu.RLock()
	defer m.mu.RUnlock()

	ctx := context.Background()

	names := m.serverNames(ctx)

	type perServer struct {
		RequestCount      int64    `json:"request_count"`
		RequestsPerSecond *float64 `json:"requests_per_second"` // nil until we add rolling counters
		AvgLatencyMs      float64  `json:"avg_latency_ms"`
		ActiveConnections int64    `json:"active_connections"`
		Inflight          int64    `json:"inflight"`
	}

	servers := make(map[string]perServer, len(names))
	activeServers := 0

	// Use a pipeline for GETs
	pipe := m.storage.Pipeline()
	reqGets := make(map[string]*redis.StringCmd, len(names))
	actGets := make(map[string]*redis.StringCmd, len(names))

	for _, name := range names {
		reqKey := fmt.Sprintf("%s:requestCount", name)
		actKey := fmt.Sprintf("%s:activeConnections", name)
		reqGets[name] = pipe.Get(ctx, reqKey)
		actGets[name] = pipe.Get(ctx, actKey)
	}

	_, _ = pipe.Exec(ctx)

	for _, name := range names {
		var rc, ac int64

		if v, err := reqGets[name].Int64(); err == nil {
			rc = v
		}
		if v, err := actGets[name].Int64(); err == nil {
			ac = v
		}

		if ac > 0 {
			activeServers++
		}

		avg := m.avgListFloat(fmt.Sprintf("%s:responseTimes", name))

		servers[name] = perServer{
			RequestCount:      rc,
			RequestsPerSecond: nil, // TODO: requires rolling time-bucketed counters
			AvgLatencyMs:      avg,
			ActiveConnections: ac,
			Inflight:          ac,
		}
	}

	proxyAvg := m.avgListFloat("proxyLatencies")

	out := map[string]interface{}{
		"active_servers":             activeServers,
		"servers":                    servers,
		"proxy_avg_added_latency_ms": proxyAvg,
		// Optional: include a timestamp
		"generated_at_unix_ms": time.Now().UnixMilli(),
	}
	return out
}
