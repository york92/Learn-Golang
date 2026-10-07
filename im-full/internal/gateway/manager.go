package gateway

import (
	"sync"
	"sync/atomic"
)

const shardCount = 64

type shard struct {
	mu sync.RWMutex
	m  map[int64]map[string]*Conn // uid -> deviceID -> conn
}

// Manager 管理本机所有已认证连接。
// 分 64 个 shard，按 uid 取模，降低读写锁竞争（百万连接下全局锁会成为瓶颈）。
type Manager struct {
	shards [shardCount]shard
	count  atomic.Int64
}

func NewManager() *Manager {
	m := &Manager{}
	for i := range m.shards {
		m.shards[i].m = make(map[int64]map[string]*Conn)
	}
	return m
}

func (m *Manager) shardOf(uid int64) *shard { return &m.shards[uint64(uid)%shardCount] }

// Add 注册连接；若同 uid+device 已有旧连接则返回它，由调用方踢掉（"同端互踢"）。
func (m *Manager) Add(c *Conn) (old *Conn) {
	sh := m.shardOf(c.uid)
	sh.mu.Lock()
	defer sh.mu.Unlock()
	devs := sh.m[c.uid]
	if devs == nil {
		devs = make(map[string]*Conn, 2)
		sh.m[c.uid] = devs
	}
	old = devs[c.deviceID]
	devs[c.deviceID] = c
	if old == nil {
		m.count.Add(1)
	}
	return old
}

// Remove 只在 map 里存的就是 c 本身时才删除。
// 这一点很关键：客户端快速重连时，新连接可能已经顶替了旧连接，
// 旧连接的清理逻辑迟到执行，不能把新连接误删。
func (m *Manager) Remove(c *Conn) bool {
	sh := m.shardOf(c.uid)
	sh.mu.Lock()
	defer sh.mu.Unlock()
	devs := sh.m[c.uid]
	if devs == nil || devs[c.deviceID] != c {
		return false
	}
	delete(devs, c.deviceID)
	if len(devs) == 0 {
		delete(sh.m, c.uid)
	}
	m.count.Add(-1)
	return true
}

// Conns 返回 uid 当前所有在线设备连接的快照。
func (m *Manager) Conns(uid int64) []*Conn {
	sh := m.shardOf(uid)
	sh.mu.RLock()
	defer sh.mu.RUnlock()
	devs := sh.m[uid]
	out := make([]*Conn, 0, len(devs))
	for _, c := range devs {
		out = append(out, c)
	}
	return out
}

// Snapshot 返回全部连接快照（用于优雅停机）。
func (m *Manager) Snapshot() []*Conn {
	var out []*Conn
	for i := range m.shards {
		sh := &m.shards[i]
		sh.mu.RLock()
		for _, devs := range sh.m {
			for _, c := range devs {
				out = append(out, c)
			}
		}
		sh.mu.RUnlock()
	}
	return out
}

func (m *Manager) Count() int64 { return m.count.Load() }
