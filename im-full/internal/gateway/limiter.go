package gateway

import "time"

// limiter 是令牌桶。每个连接的 readLoop 单协程使用，因此无需加锁。
type limiter struct {
	rate   float64 // 每秒补充令牌数
	burst  float64
	tokens float64
	last   time.Time
}

func newLimiter(rate float64, burst int) *limiter {
	return &limiter{rate: rate, burst: float64(burst), tokens: float64(burst), last: time.Now()}
}

func (l *limiter) Allow() bool {
	now := time.Now()
	l.tokens += now.Sub(l.last).Seconds() * l.rate
	if l.tokens > l.burst {
		l.tokens = l.burst
	}
	l.last = now
	if l.tokens < 1 {
		return false
	}
	l.tokens--
	return true
}
