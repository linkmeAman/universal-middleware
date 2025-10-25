package loadbalancer

import (
	"net/http"
	"sync"
	"sync/atomic"
	"time"

	"go.uber.org/zap"
)

// Backend represents a backend server instance
type Backend struct {
	URL           string
	Healthy       bool
	LastChecked   time.Time
	FailureCount  int
	ResponseTime  time.Duration
	MaxFailures   int
	CheckInterval time.Duration
}

// LoadBalancer implements load balancing functionality
type LoadBalancer struct {
	backends     []*Backend
	current      uint64
	mu           sync.RWMutex
	logger       *zap.Logger
	healthyCount int32
}

// New creates a new load balancer instance
func New(logger *zap.Logger) *LoadBalancer {
	lb := &LoadBalancer{
		backends: make([]*Backend, 0),
		logger:   logger,
	}
	go lb.healthCheck()
	return lb
}

// AddBackend adds a new backend server to the load balancer
func (lb *LoadBalancer) AddBackend(url string) {
	lb.mu.Lock()
	defer lb.mu.Unlock()

	backend := &Backend{
		URL:           url,
		Healthy:       true,
		MaxFailures:   3,
		CheckInterval: 10 * time.Second,
	}
	lb.backends = append(lb.backends, backend)
	atomic.AddInt32(&lb.healthyCount, 1)
}

// RemoveBackend removes a backend server from the load balancer
func (lb *LoadBalancer) RemoveBackend(url string) {
	lb.mu.Lock()
	defer lb.mu.Unlock()

	for i, backend := range lb.backends {
		if backend.URL == url {
			if backend.Healthy {
				atomic.AddInt32(&lb.healthyCount, -1)
			}
			lb.backends = append(lb.backends[:i], lb.backends[i+1:]...)
			return
		}
	}
}

// NextBackend returns the next available backend using round-robin algorithm
func (lb *LoadBalancer) NextBackend() *Backend {
	lb.mu.RLock()
	defer lb.mu.RUnlock()

	if len(lb.backends) == 0 {
		return nil
	}

	next := atomic.AddUint64(&lb.current, 1)
	idx := next % uint64(len(lb.backends))
	backend := lb.backends[idx]

	// If the chosen backend is unhealthy, try to find a healthy one
	if !backend.Healthy {
		for i := 0; i < len(lb.backends); i++ {
			idx = (idx + 1) % uint64(len(lb.backends))
			if lb.backends[idx].Healthy {
				return lb.backends[idx]
			}
		}
		// If no healthy backend is found, return nil
		return nil
	}

	return backend
}

// healthCheck performs periodic health checks on all backends
func (lb *LoadBalancer) healthCheck() {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		lb.mu.Lock()
		for _, backend := range lb.backends {
			go lb.checkBackendHealth(backend)
		}
		lb.mu.Unlock()
	}
}

// checkBackendHealth checks the health of a single backend
func (lb *LoadBalancer) checkBackendHealth(backend *Backend) {
	start := time.Now()
	resp, err := http.Get(backend.URL + "/health")

	lb.mu.Lock()
	defer lb.mu.Unlock()

	if err != nil || resp.StatusCode != http.StatusOK {
		backend.FailureCount++
		if backend.FailureCount >= backend.MaxFailures && backend.Healthy {
			backend.Healthy = false
			atomic.AddInt32(&lb.healthyCount, -1)
			lb.logger.Warn("Backend marked as unhealthy",
				zap.String("url", backend.URL),
				zap.Int("failures", backend.FailureCount))
		}
	} else {
		if !backend.Healthy {
			backend.Healthy = true
			atomic.AddInt32(&lb.healthyCount, 1)
			lb.logger.Info("Backend marked as healthy",
				zap.String("url", backend.URL))
		}
		backend.FailureCount = 0
		backend.ResponseTime = time.Since(start)
	}

	if resp != nil {
		resp.Body.Close()
	}
	backend.LastChecked = time.Now()
}

// HealthyBackendCount returns the number of healthy backends
func (lb *LoadBalancer) HealthyBackendCount() int32 {
	return atomic.LoadInt32(&lb.healthyCount)
}
