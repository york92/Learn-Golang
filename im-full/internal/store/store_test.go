package store

import (
	"encoding/hex"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestPBKDF2Vector(t *testing.T) {
	// RFC 7914 / 常用测试向量：PBKDF2-HMAC-SHA256("password","salt",c=1,dkLen=32)
	got := hex.EncodeToString(pbkdf2SHA256([]byte("password"), []byte("salt"), 1, 32))
	want := "120fb6cffcf8b32c43e7225256c4f837a86548c92ccc35480805987cb70be17b"
	if got != want {
		t.Fatalf("pbkdf2 mismatch:\n got %s\nwant %s", got, want)
	}
}

func open(t *testing.T, dir string) *Store {
	t.Helper()
	s, err := Open(dir, Options{FsyncInterval: 50 * time.Millisecond})
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func TestUserAuth(t *testing.T) {
	s := open(t, t.TempDir())
	defer s.Close()
	u, err := s.CreateUser("alice", "", "secret1")
	if err != nil || u.Nickname != "alice" || u.Hash != "" {
		t.Fatalf("create: %v %+v", err, u)
	}
	if _, err := s.CreateUser("alice", "", "secret1"); err != ErrUserExists {
		t.Fatalf("want ErrUserExists, got %v", err)
	}
	if _, err := s.CreateUser("a!", "", "secret1"); err == nil {
		t.Fatal("invalid username accepted")
	}
	if _, err := s.Verify("alice", "wrong"); err != ErrBadCredentials {
		t.Fatalf("want bad credentials, got %v", err)
	}
	if _, err := s.Verify("alice", "secret1"); err != nil {
		t.Fatal(err)
	}
	tok, _, _ := s.IssueToken(u.UID, time.Hour)
	if uid, ok := s.ResolveToken(tok); !ok || uid != u.UID {
		t.Fatal("token should resolve")
	}
	s.RevokeToken(tok)
	if _, ok := s.ResolveToken(tok); ok {
		t.Fatal("revoked token still valid")
	}
	exp, _, _ := s.IssueToken(u.UID, -time.Second)
	if _, ok := s.ResolveToken(exp); ok {
		t.Fatal("expired token still valid")
	}
}

func TestMessagesSeqDedupSyncRead(t *testing.T) {
	s := open(t, t.TempDir())
	defer s.Close()
	a, _ := s.CreateUser("alice", "", "secret1")
	b, _ := s.CreateUser("bob", "", "secret1")
	conv := SingleConvID(a.UID, b.UID)
	send := func(from int64, cmid, text string) AppendResult {
		r, err := s.AppendMessage(NewMessage{ConvID: conv, From: from, Recipients: []int64{a.UID, b.UID}, Type: 1, Content: text, ClientMsgID: cmid})
		if err != nil {
			t.Fatal(err)
		}
		return r
	}
	r1 := send(a.UID, "c1", "one")
	r2 := send(b.UID, "c2", "two")
	dup := send(a.UID, "c1", "one")
	if r1.Msg.Seq != 1 || r2.Msg.Seq != 2 {
		t.Fatalf("会话 seq 必须连续递增: %d %d", r1.Msg.Seq, r2.Msg.Seq)
	}
	if !dup.Dup || dup.Msg.MsgID != r1.Msg.MsgID {
		t.Fatalf("重复的 client_msg_id 必须命中幂等: %+v", dup)
	}
	if got := s.History(conv, 0, 10); len(got) != 2 {
		t.Fatalf("幂等命中后不应多写一条, got %d", len(got))
	}
	if r2.UserSeqs[a.UID] != 2 || r2.UserSeqs[b.UID] != 2 {
		t.Fatalf("user_seq: %+v", r2.UserSeqs)
	}

	items, latest, more := s.Sync(a.UID, 1, 10)
	if len(items) != 1 || items[0].UserSeq != 2 || latest != 2 || more {
		t.Fatalf("sync: %+v %d %v", items, latest, more)
	}
	if items, _, more := s.Sync(a.UID, 0, 1); len(items) != 1 || !more {
		t.Fatalf("sync limit/has_more: %+v %v", items, more)
	}

	// alice 发了 seq1，bob 发了 seq2 → alice 未读 1（bob 的），bob 未读 0（他最后发的是 seq2）
	convs, _ := s.Conversations(a.UID)
	if len(convs) != 1 || convs[0].Unread != 1 {
		t.Fatalf("alice unread want 1, got %+v", convs)
	}
	seq, changed, err := s.MarkRead(a.UID, conv, 99) // 超出范围自动夹紧
	if err != nil || !changed || seq != 2 {
		t.Fatalf("markread: %d %v %v", seq, changed, err)
	}
	if _, changed, _ := s.MarkRead(a.UID, conv, 1); changed {
		t.Fatal("已读位置不能回退")
	}
	if _, _, err := s.MarkRead(999, conv, 1); err != ErrForbidden {
		t.Fatalf("非会话成员不能标记已读, got %v", err)
	}
	// 分页历史
	for i := 0; i < 5; i++ {
		send(a.UID, "", "x")
	}
	page := s.History(conv, 0, 3)
	if len(page) != 3 || page[2].Seq != 7 {
		t.Fatalf("history latest page: %+v", page)
	}
	older := s.History(conv, page[0].Seq, 3)
	if len(older) != 3 || older[2].Seq != page[0].Seq-1 {
		t.Fatalf("history older page: %+v", older)
	}
}

func TestGroupLifecycle(t *testing.T) {
	s := open(t, t.TempDir())
	defer s.Close()
	a, _ := s.CreateUser("alice", "", "secret1")
	b, _ := s.CreateUser("bob", "", "secret1")
	c, _ := s.CreateUser("carol", "", "secret1")
	g, err := s.CreateGroup(a.UID, "team", []int64{b.UID, b.UID})
	if err != nil || len(g.Members) != 2 {
		t.Fatalf("create group: %v %+v", err, g)
	}
	if err := s.AddGroupMember(g.ID, b.UID, c.UID); err != ErrForbidden {
		t.Fatalf("only owner can add, got %v", err)
	}
	if err := s.AddGroupMember(g.ID, a.UID, c.UID); err != nil {
		t.Fatal(err)
	}
	if !s.InConv(c.UID, GroupConvID(g.ID)) {
		t.Fatal("new member should see the group conversation")
	}
	if err := s.LeaveGroup(g.ID, a.UID); err == nil {
		t.Fatal("owner must not be able to leave")
	}
	if err := s.LeaveGroup(g.ID, c.UID); err != nil {
		t.Fatal(err)
	}
	if s.InConv(c.UID, GroupConvID(g.ID)) {
		t.Fatal("left member should lose the conversation")
	}
}

// 核心保证：重启后回放 WAL，状态完全一致；并且能容忍崩溃时写了一半的尾行。
func TestReplayAndTornTail(t *testing.T) {
	dir := t.TempDir()
	s := open(t, dir)
	a, _ := s.CreateUser("alice", "Alice", "secret1")
	b, _ := s.CreateUser("bob", "", "secret1")
	s.AddFriend(a.UID, b.UID)
	g, _ := s.CreateGroup(a.UID, "g", []int64{b.UID})
	conv := SingleConvID(a.UID, b.UID)
	for i := 0; i < 3; i++ {
		s.AppendMessage(NewMessage{ConvID: conv, From: a.UID, Recipients: []int64{a.UID, b.UID}, Type: 1, Content: "m", ClientMsgID: string(rune('a' + i))})
	}
	s.MarkRead(b.UID, conv, 2)
	tok, _, _ := s.IssueToken(a.UID, time.Hour)
	s.Close()

	// 模拟崩溃：尾部追加半行垃圾
	f, _ := os.OpenFile(filepath.Join(dir, "wal.jsonl"), os.O_APPEND|os.O_WRONLY, 0o644)
	f.WriteString(`{"t":"msg","m":{"msg_id":99,"conv`)
	f.Close()

	s2 := open(t, dir)
	defer s2.Close()
	if _, err := s2.Verify("alice", "secret1"); err != nil {
		t.Fatal("user lost after replay:", err)
	}
	if !s2.AreFriends(a.UID, b.UID) {
		t.Fatal("friendship lost")
	}
	if gg, ok := s2.GetGroup(g.ID); !ok || len(gg.Members) != 2 {
		t.Fatal("group lost")
	}
	if got := s2.History(conv, 0, 10); len(got) != 3 {
		t.Fatalf("messages lost: %d", len(got))
	}
	if s2.ReadSeq(b.UID, conv) != 2 {
		t.Fatal("read pointer lost")
	}
	if _, ok := s2.ResolveToken(tok); !ok {
		t.Fatal("token lost")
	}
	// 回放后幂等表也要恢复：重发同一 client_msg_id 不能重复入库
	r, _ := s2.AppendMessage(NewMessage{ConvID: conv, From: a.UID, Recipients: []int64{a.UID, b.UID}, Type: 1, Content: "m", ClientMsgID: "a"})
	if !r.Dup {
		t.Fatal("dedup table not rebuilt")
	}
	// 回放后新消息的 id/seq 必须接着往后排
	r, _ = s2.AppendMessage(NewMessage{ConvID: conv, From: b.UID, Recipients: []int64{a.UID, b.UID}, Type: 1, Content: "new", ClientMsgID: "z"})
	if r.Msg.Seq != 4 || r.Msg.MsgID != 4 {
		t.Fatalf("counters after replay: seq=%d id=%d", r.Msg.Seq, r.Msg.MsgID)
	}
	// 截断后文件应保持可再次打开
	s2.Close()
	s3 := open(t, dir)
	s3.Close()
}
