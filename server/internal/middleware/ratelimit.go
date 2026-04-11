package middleware

import (
	"net"
	"net/http"
	"sync"
	"time"
)

type ipLimiter struct {
	mu      sync.Mutex
	buckets map[string]*bucket
	rate    float64
	burst   int
}

type bucket struct {
	tokens   float64
	lastSeen time.Time
}

func newIPLimiter(perMinute int) *ipLimiter {
	l := &ipLimiter{
		buckets: make(map[string]*bucket),
		rate:    float64(perMinute) / 60.0,
		burst:   perMinute,
	}
	go l.cleanup()
	return l
}

func (l *ipLimiter) allow(ip string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()

	b, ok := l.buckets[ip]
	now := time.Now()
	if !ok {
		l.buckets[ip] = &bucket{tokens: float64(l.burst) - 1, lastSeen: now}
		return true
	}

	elapsed := now.Sub(b.lastSeen).Seconds()
	b.tokens += elapsed * l.rate
	if b.tokens > float64(l.burst) {
		b.tokens = float64(l.burst)
	}
	b.lastSeen = now

	if b.tokens < 1 {
		return false
	}
	b.tokens--
	return true
}

func (l *ipLimiter) cleanup() {
	for {
		time.Sleep(5 * time.Minute)
		l.mu.Lock()
		cutoff := time.Now().Add(-10 * time.Minute)
		for ip, b := range l.buckets {
			if b.lastSeen.Before(cutoff) {
				delete(l.buckets, ip)
			}
		}
		l.mu.Unlock()
	}
}

func clientIP(r *http.Request) string {
	if fwd := r.Header.Get("X-Forwarded-For"); fwd != "" {
		return fwd
	}
	host, _, _ := net.SplitHostPort(r.RemoteAddr)
	return host
}

func RateLimit(perMinute int, next http.HandlerFunc) http.HandlerFunc {
	limiter := newIPLimiter(perMinute)
	return func(w http.ResponseWriter, r *http.Request) {
		if !limiter.allow(clientIP(r)) {
			http.Error(w, `{"error":"too many requests"}`, http.StatusTooManyRequests)
			return
		}
		next(w, r)
	}
}
