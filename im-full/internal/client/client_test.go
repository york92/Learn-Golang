package client

import (
	"sync"
	"testing"

	"example.com/im/internal/protocol"
)

// 游标逻辑：乱序、重复、空洞。
func TestDeliverCursorGapAndDup(t *testing.T) {
	var mu sync.Mutex
	var got []int64
	c := New(Options{OnPush: func(p *protocol.Push) { mu.Lock(); got = append(got, p.UserSeq); mu.Unlock() }})
	push := func(seq, id int64) bool { return c.deliver(&protocol.Push{UserSeq: seq, MsgID: id}) }

	if push(1, 101) {
		t.Fatal("连续消息不应触发 sync")
	}
	if !push(3, 103) { // 跳过了 2 → 出现空洞
		t.Fatal("出现空洞必须触发 sync")
	}
	if c.Cursor() != 1 {
		t.Fatalf("空洞存在时游标不能越过空洞: %d", c.Cursor())
	}
	push(2, 102) // 空洞被补上，游标应一次性吸收到 3
	if c.Cursor() != 3 {
		t.Fatalf("补齐后游标应为 3, got %d", c.Cursor())
	}
	push(2, 102) // 重复（<= 游标）
	push(3, 103)
	mu.Lock()
	defer mu.Unlock()
	if len(got) != 3 {
		t.Fatalf("每条消息只能交付一次, got %v", got)
	}
}
