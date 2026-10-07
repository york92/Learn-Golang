package api

import (
	"sync"
	"time"
)

// ipLimiter：按来源 IP 的简单令牌桶，用于登录/注册防暴力破解。
type ipLimiter struct {
	mu      sync.Mutex
	rate    float64 // 令牌/秒
	burst   float64
	buckets map[string]*bucket
}

type bucket struct {
	tokens float64
	last   time.Time
}

func newIPLimiter(perMinute int, burst int) *ipLimiter {
	return &ipLimiter{rate: float64(perMinute) / 60, burst: float64(burst), buckets: map[string]*bucket{}}
}

// Peek 只检查、不消耗令牌：用于"登录前"判断该 IP 是否已因失败过多被限制。
func (l *ipLimiter) Peek(ip string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	b := l.buckets[ip]
	if b == nil {
		return true
	}
	return b.tokens+time.Since(b.last).Seconds()*l.rate >= 1
}

// Allow 检查并消耗一个令牌。
func (l *ipLimiter) Allow(ip string) bool {
	now := time.Now()
	l.mu.Lock()
	defer l.mu.Unlock()
	if len(l.buckets) > 50000 { // 防内存被大量不同 IP 撑爆：粗暴清空
		l.buckets = map[string]*bucket{}
	}
	b := l.buckets[ip]
	if b == nil {
		b = &bucket{tokens: l.burst, last: now}
		l.buckets[ip] = b
	}
	b.tokens += now.Sub(b.last).Seconds() * l.rate
	if b.tokens > l.burst {
		b.tokens = l.burst
	}
	b.last = now
	if b.tokens < 1 {
		return false
	}
	b.tokens--
	return true
}
