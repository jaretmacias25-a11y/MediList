package httpapi

import (
	"sync"
	"time"
)

type rateWindow struct {
	count int
	reset time.Time
}

// rateLimiter implementa una ventana fija en memoria por clave (IP).
type rateLimiter struct {
	mu      sync.Mutex
	limit   int
	window  time.Duration
	clients map[string]*rateWindow
}

func newRateLimiter(limit int, window time.Duration) *rateLimiter {
	return &rateLimiter{
		limit:   limit,
		window:  window,
		clients: make(map[string]*rateWindow),
	}
}

// Allow indica si la clave puede realizar otra solicitud y, si no, cuánto esperar.
func (l *rateLimiter) Allow(key string) (bool, time.Duration) {
	now := time.Now()
	l.mu.Lock()
	defer l.mu.Unlock()

	if len(l.clients) > 5000 {
		for k, v := range l.clients {
			if now.After(v.reset) {
				delete(l.clients, k)
			}
		}
	}

	entry, ok := l.clients[key]
	if !ok || now.After(entry.reset) {
		l.clients[key] = &rateWindow{count: 1, reset: now.Add(l.window)}
		return true, 0
	}
	if entry.count >= l.limit {
		return false, time.Until(entry.reset)
	}
	entry.count++
	return true, 0
}
