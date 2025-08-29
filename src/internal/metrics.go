package internal

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strconv"
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

// discuss with team what info we want out of this endpoint
func (m *metricsLogger) stats() interface{} {
	return 0
}
