// Package store 是零依赖的持久化存储：内存索引 + 追加写日志（WAL）。
//
// 所有写操作遵循同一个模式：校验 → 生成 record → 先写 WAL → 再 apply 到内存。
// 启动时按顺序回放 WAL，用同一个 apply 函数重建状态，因此内存与磁盘永远一致。
//
// 适用范围：单机、中小规模（消息全量驻留内存）。想换成 MySQL/PostgreSQL，
// 只需实现相同方法集（service 层只依赖这些方法），详见 docs/ROADMAP.md。
package store

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"sync"
	"time"
	"unicode/utf8"
)

type Options struct {
	// FsyncInterval：后台 fsync 间隔。>0 为组提交（断电最多丢这段时间的数据，
	// 进程崩溃不丢）；0 表示每次写都 fsync（最安全，最慢）。
	FsyncInterval time.Duration
}

type tokenRec struct {
	Token string `json:"token"`
	UID   int64  `json:"uid"`
	Exp   int64  `json:"exp"` // unix 秒
}

type readRec struct {
	UID  int64  `json:"uid"`
	Conv string `json:"conv"`
	Seq  int64  `json:"seq"`
}

type record struct {
	T     string    `json:"t"`
	User  *User     `json:"u,omitempty"`
	Token *tokenRec `json:"k,omitempty"`
	A     int64     `json:"a,omitempty"`
	B     int64     `json:"b,omitempty"`
	Group *Group    `json:"g,omitempty"` // 群的完整快照（创建/加人/退群都写快照）
	Msg   *Message  `json:"m,omitempty"`
	Rcpt  []int64   `json:"r,omitempty"`
	Read  *readRec  `json:"d,omitempty"`
}

type Store struct {
	mu   sync.RWMutex
	f    *os.File
	opts Options

	dirty     bool
	closeOnce sync.Once
	closeErr  error
	closeCh   chan struct{}
	wg        sync.WaitGroup

	nextUID, nextGID, nextMsgID int64

	users     map[int64]*User
	byName    map[string]int64
	tokens    map[string]tokenRec
	friends   map[int64]map[int64]struct{}
	groups    map[int64]*Group
	convMsgs  map[string][]*Message // seq-1 即下标，seq 连续
	inbox     map[int64][]*Message  // user_seq-1 即下标
	userConvs map[int64]map[string]struct{}
	reads     map[int64]map[string]int64
	dedup     map[string]*Message // "uid|clientMsgID" -> 已落库消息
}

func Open(dir string, opts Options) (*Store, error) {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, err
	}
	path := filepath.Join(dir, "wal.jsonl")
	f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0o600) // 含密码哈希与令牌，仅属主可读写
	if err != nil {
		return nil, err
	}
	s := &Store{
		f: f, opts: opts, closeCh: make(chan struct{}),
		nextUID: 1000, nextGID: 1, nextMsgID: 1,
		users: map[int64]*User{}, byName: map[string]int64{}, tokens: map[string]tokenRec{},
		friends: map[int64]map[int64]struct{}{}, groups: map[int64]*Group{},
		convMsgs: map[string][]*Message{}, inbox: map[int64][]*Message{},
		userConvs: map[int64]map[string]struct{}{}, reads: map[int64]map[string]int64{},
		dedup: map[string]*Message{},
	}
	if err := s.replay(); err != nil {
		f.Close()
		return nil, err
	}
	if opts.FsyncInterval > 0 {
		s.wg.Add(1)
		go s.fsyncLoop()
	}
	return s, nil
}

// replay 回放 WAL。若末尾有半行（崩溃时写了一半），截断丢弃，保证后续追加不被污染。
func (s *Store) replay() error {
	r := bufio.NewReaderSize(s.f, 1<<20)
	var valid int64
	for {
		line, err := r.ReadBytes('\n')
		if len(line) > 0 && line[len(line)-1] == '\n' {
			var rec record
			if jerr := json.Unmarshal(bytes.TrimSpace(line), &rec); jerr != nil {
				return fmt.Errorf("wal corrupted at offset %d: %w", valid, jerr)
			}
			s.apply(&rec)
			valid += int64(len(line))
		}
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
	}
	if st, err := s.f.Stat(); err == nil && st.Size() != valid {
		if err := s.f.Truncate(valid); err != nil {
			return err
		}
	}
	_, err := s.f.Seek(valid, io.SeekStart)
	return err
}

func (s *Store) fsyncLoop() {
	defer s.wg.Done()
	t := time.NewTicker(s.opts.FsyncInterval)
	defer t.Stop()
	for {
		select {
		case <-t.C:
			s.mu.Lock()
			d := s.dirty
			s.dirty = false
			s.mu.Unlock()
			if d {
				_ = s.f.Sync()
			}
		case <-s.closeCh:
			return
		}
	}
}

// Close 幂等：可安全重复调用。
func (s *Store) Close() error {
	s.closeOnce.Do(func() {
		close(s.closeCh)
		s.wg.Wait()
		s.mu.Lock()
		defer s.mu.Unlock()
		_ = s.f.Sync()
		s.closeErr = s.f.Close()
	})
	return s.closeErr
}

// commit：先写 WAL 再 apply。调用方必须持有写锁。
func (s *Store) commit(rec *record) error {
	b, err := json.Marshal(rec)
	if err != nil {
		return err
	}
	b = append(b, '\n')
	if _, err := s.f.Write(b); err != nil {
		return err
	}
	if s.opts.FsyncInterval <= 0 {
		if err := s.f.Sync(); err != nil {
			return err
		}
	} else {
		s.dirty = true
	}
	s.apply(rec)
	return nil
}

// apply 是唯一修改内存状态的地方（运行时与回放共用）。
func (s *Store) apply(r *record) {
	switch r.T {
	case "user":
		u := *r.User
		s.users[u.UID] = &u
		s.byName[u.Username] = u.UID
		if u.UID >= s.nextUID {
			s.nextUID = u.UID + 1
		}
	case "token":
		s.tokens[r.Token.Token] = *r.Token
	case "tokrev":
		delete(s.tokens, r.Token.Token)
	case "friend":
		s.addFriendLocked(r.A, r.B)
		s.addFriendLocked(r.B, r.A)
	case "group":
		g := r.Group.clone()
		old := s.groups[g.ID]
		s.groups[g.ID] = g
		if g.ID >= s.nextGID {
			s.nextGID = g.ID + 1
		}
		conv := GroupConvID(g.ID)
		for _, m := range g.Members {
			s.addConvLocked(m, conv)
		}
		if old != nil { // 退出群的人，会话列表里移除
			for _, m := range old.Members {
				if !g.HasMember(m) {
					delete(s.userConvs[m], conv)
				}
			}
		}
	case "msg":
		m := *r.Msg
		s.convMsgs[m.ConvID] = append(s.convMsgs[m.ConvID], &m)
		for _, uid := range r.Rcpt {
			s.inbox[uid] = append(s.inbox[uid], &m)
			s.addConvLocked(uid, m.ConvID)
		}
		if m.ClientMsgID != "" {
			s.dedup[dedupKey(m.From, m.ClientMsgID)] = &m
		}
		if m.MsgID >= s.nextMsgID {
			s.nextMsgID = m.MsgID + 1
		}
	case "read":
		s.setReadLocked(r.Read.UID, r.Read.Conv, r.Read.Seq)
	}
}

func (s *Store) addFriendLocked(a, b int64) {
	if s.friends[a] == nil {
		s.friends[a] = map[int64]struct{}{}
	}
	s.friends[a][b] = struct{}{}
}

func (s *Store) addConvLocked(uid int64, conv string) {
	if s.userConvs[uid] == nil {
		s.userConvs[uid] = map[string]struct{}{}
	}
	s.userConvs[uid][conv] = struct{}{}
}

func (s *Store) setReadLocked(uid int64, conv string, seq int64) bool {
	if s.reads[uid] == nil {
		s.reads[uid] = map[string]int64{}
	}
	if seq <= s.reads[uid][conv] {
		return false
	}
	s.reads[uid][conv] = seq
	return true
}

func dedupKey(uid int64, cmid string) string { return fmt.Sprintf("%d|%s", uid, cmid) }

// ================================================================ 用户 / 令牌

var usernameRe = regexp.MustCompile(`^[A-Za-z0-9_]{3,32}$`)

func (s *Store) CreateUser(username, nickname, password string) (*User, error) {
	if !usernameRe.MatchString(username) {
		return nil, fmt.Errorf("%w: username must be 3-32 chars of letters, digits or _", ErrInvalid)
	}
	if len(password) < 6 || len(password) > 128 {
		return nil, fmt.Errorf("%w: password must be 6-128 chars", ErrInvalid)
	}
	if nickname == "" {
		nickname = username
	}
	if utf8.RuneCountInString(nickname) > 32 {
		return nil, fmt.Errorf("%w: nickname too long", ErrInvalid)
	}
	salt, hash := hashPassword(password) // 耗时操作放在锁外
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.byName[username]; ok {
		return nil, ErrUserExists
	}
	u := &User{UID: s.nextUID, Username: username, Nickname: nickname, Salt: salt, Hash: hash, Created: time.Now().UnixMilli()}
	if err := s.commit(&record{T: "user", User: u}); err != nil {
		return nil, err
	}
	pub := u.Public()
	return &pub, nil
}

func (s *Store) Verify(username, password string) (*User, error) {
	s.mu.RLock()
	uid, ok := s.byName[username]
	var u User
	if ok {
		u = *s.users[uid]
	}
	s.mu.RUnlock()
	if !ok {
		// 用户不存在也做一次哈希，避免通过响应时间探测用户名是否存在。
		checkPassword(password, "00", "00")
		_, _ = hashPassword(password)
		return nil, ErrBadCredentials
	}
	if !checkPassword(password, u.Salt, u.Hash) {
		return nil, ErrBadCredentials
	}
	pub := u.Public()
	return &pub, nil
}

func (s *Store) GetUser(uid int64) (User, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	u, ok := s.users[uid]
	if !ok {
		return User{}, false
	}
	return u.Public(), true
}

func (s *Store) FindUser(username string) (User, bool) {
	s.mu.RLock()
	uid, ok := s.byName[username]
	s.mu.RUnlock()
	if !ok {
		return User{}, false
	}
	return s.GetUser(uid)
}

func (s *Store) IssueToken(uid int64, ttl time.Duration) (token string, exp time.Time, err error) {
	exp = time.Now().Add(ttl)
	tok := newToken()
	s.mu.Lock()
	defer s.mu.Unlock()
	if err = s.commit(&record{T: "token", Token: &tokenRec{Token: tok, UID: uid, Exp: exp.Unix()}}); err != nil {
		return "", time.Time{}, err
	}
	return tok, exp, nil
}

func (s *Store) ResolveToken(token string) (int64, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	t, ok := s.tokens[token]
	if !ok || time.Now().Unix() > t.Exp {
		return 0, false
	}
	return t.UID, true
}

func (s *Store) RevokeToken(token string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.tokens[token]; !ok {
		return nil
	}
	return s.commit(&record{T: "tokrev", Token: &tokenRec{Token: token}})
}

// ================================================================ 好友 / 群

func (s *Store) AddFriend(a, b int64) error {
	if a == b {
		return fmt.Errorf("%w: cannot add yourself", ErrInvalid)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.users[a] == nil || s.users[b] == nil {
		return ErrNotFound
	}
	if _, ok := s.friends[a][b]; ok {
		return nil // 幂等
	}
	return s.commit(&record{T: "friend", A: a, B: b})
}

func (s *Store) AreFriends(a, b int64) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	_, ok := s.friends[a][b]
	return ok
}

func (s *Store) Friends(uid int64) []User {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]User, 0, len(s.friends[uid]))
	for f := range s.friends[uid] {
		if u := s.users[f]; u != nil {
			out = append(out, u.Public())
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].UID < out[j].UID })
	return out
}

func (s *Store) CreateGroup(owner int64, name string, members []int64) (*Group, error) {
	if l := utf8.RuneCountInString(name); l == 0 || l > 64 {
		return nil, fmt.Errorf("%w: group name must be 1-64 chars", ErrInvalid)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	set := map[int64]bool{owner: true}
	list := []int64{owner}
	for _, m := range members {
		if s.users[m] == nil {
			return nil, fmt.Errorf("%w: user %d", ErrNotFound, m)
		}
		if !set[m] {
			set[m] = true
			list = append(list, m)
		}
	}
	if len(list) > 500 {
		return nil, fmt.Errorf("%w: too many members (max 500)", ErrInvalid)
	}
	g := &Group{ID: s.nextGID, Name: name, Owner: owner, Members: list, Created: time.Now().UnixMilli()}
	if err := s.commit(&record{T: "group", Group: g}); err != nil {
		return nil, err
	}
	return g.clone(), nil
}

func (s *Store) GetGroup(id int64) (*Group, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	g, ok := s.groups[id]
	if !ok {
		return nil, false
	}
	return g.clone(), true
}

func (s *Store) GroupsOf(uid int64) []*Group {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var out []*Group
	for _, g := range s.groups {
		if g.HasMember(uid) {
			out = append(out, g.clone())
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

// AddGroupMember 仅群主可拉人。
func (s *Store) AddGroupMember(gid, operator, uid int64) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	g := s.groups[gid]
	if g == nil || s.users[uid] == nil {
		return ErrNotFound
	}
	if g.Owner != operator {
		return ErrForbidden
	}
	if g.HasMember(uid) {
		return nil
	}
	if len(g.Members) >= 500 {
		return fmt.Errorf("%w: group is full", ErrInvalid)
	}
	ng := g.clone()
	ng.Members = append(ng.Members, uid)
	return s.commit(&record{T: "group", Group: ng})
}

// LeaveGroup 退群。群主不能直接退群（演示版未实现转让/解散）。
func (s *Store) LeaveGroup(gid, uid int64) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	g := s.groups[gid]
	if g == nil || !g.HasMember(uid) {
		return ErrNotFound
	}
	if g.Owner == uid {
		return fmt.Errorf("%w: owner cannot leave the group", ErrForbidden)
	}
	ng := g.clone()
	ng.Members = ng.Members[:0]
	for _, m := range g.Members {
		if m != uid {
			ng.Members = append(ng.Members, m)
		}
	}
	return s.commit(&record{T: "group", Group: ng})
}

// ================================================================ 消息

// AppendMessage 是写消息的唯一入口，在一把锁内完成：
// 幂等检查 → 分配 msg_id / 会话 seq → 写 WAL → 写入各收件人的收件箱。
func (s *Store) AppendMessage(in NewMessage) (AppendResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if in.ClientMsgID != "" {
		if m, ok := s.dedup[dedupKey(in.From, in.ClientMsgID)]; ok {
			return AppendResult{Msg: *m, Dup: true}, nil
		}
	}
	m := &Message{
		MsgID: s.nextMsgID, ConvID: in.ConvID, Seq: int64(len(s.convMsgs[in.ConvID])) + 1,
		From: in.From, Type: in.Type, Content: in.Content, ClientMsgID: in.ClientMsgID,
		Time: time.Now().UnixMilli(),
	}
	if err := s.commit(&record{T: "msg", Msg: m, Rcpt: in.Recipients}); err != nil {
		return AppendResult{}, err
	}
	// 发送即视为已读到这条（避免自己发的消息算未读）
	if s.setReadLocked(in.From, in.ConvID, m.Seq) {
		_ = s.commit(&record{T: "read", Read: &readRec{UID: in.From, Conv: in.ConvID, Seq: m.Seq}})
	}
	res := AppendResult{Msg: *m, UserSeqs: make(map[int64]int64, len(in.Recipients))}
	for _, uid := range in.Recipients {
		res.UserSeqs[uid] = int64(len(s.inbox[uid]))
	}
	return res, nil
}

// History 返回会话中 seq < beforeSeq 的最近 limit 条（beforeSeq<=0 表示从最新开始），按 seq 升序。
func (s *Store) History(conv string, beforeSeq int64, limit int) []Message {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	msgs := s.convMsgs[conv]
	end := len(msgs)
	if beforeSeq > 0 && int(beforeSeq-1) < end {
		end = int(beforeSeq - 1)
	}
	start := end - limit
	if start < 0 {
		start = 0
	}
	out := make([]Message, 0, end-start)
	for _, m := range msgs[start:end] {
		out = append(out, *m)
	}
	return out
}

// Sync 返回 user_seq 在 (since, since+limit] 内的收件箱消息。
func (s *Store) Sync(uid, since int64, limit int) (items []InboxItem, latest int64, hasMore bool) {
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	if since < 0 {
		since = 0
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	box := s.inbox[uid]
	latest = int64(len(box))
	if since >= latest {
		return nil, latest, false
	}
	end := since + int64(limit)
	if end > latest {
		end = latest
	}
	for i := since; i < end; i++ {
		items = append(items, InboxItem{UserSeq: i + 1, Msg: box[i]})
	}
	return items, latest, end < latest
}

func (s *Store) LatestUserSeq(uid int64) int64 {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return int64(len(s.inbox[uid]))
}

func (s *Store) InConv(uid int64, conv string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	_, ok := s.userConvs[uid][conv]
	return ok
}

// MarkRead 前移已读位置；返回生效后的位置以及是否真的发生了变化。
func (s *Store) MarkRead(uid int64, conv string, seq int64) (int64, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.userConvs[uid][conv]; !ok {
		return 0, false, ErrForbidden
	}
	if last := int64(len(s.convMsgs[conv])); seq > last {
		seq = last
	}
	if seq <= s.reads[uid][conv] {
		return s.reads[uid][conv], false, nil
	}
	if err := s.commit(&record{T: "read", Read: &readRec{UID: uid, Conv: conv, Seq: seq}}); err != nil {
		return 0, false, err
	}
	return seq, true, nil
}

func (s *Store) ReadSeq(uid int64, conv string) int64 {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.reads[uid][conv]
}

// Conversations 返回会话摘要（按最后消息时间倒序）以及与之一致的最新 user_seq。
// 二者在同一把读锁内取得，客户端据此"快照 + 增量"：先拿快照，再从 latest 之后 sync。
func (s *Store) Conversations(uid int64) ([]ConvSummary, int64) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]ConvSummary, 0, len(s.userConvs[uid]))
	for conv := range s.userConvs[uid] {
		msgs := s.convMsgs[conv]
		cs := ConvSummary{ConvID: conv, ReadSeq: s.reads[uid][conv]}
		if n := len(msgs); n > 0 {
			m := *msgs[n-1]
			cs.Last = &m
			cs.Unread = int64(n) - cs.ReadSeq
			if cs.Unread < 0 {
				cs.Unread = 0
			}
		}
		out = append(out, cs)
	}
	t := func(c ConvSummary) int64 {
		if c.Last != nil {
			return c.Last.Time
		}
		return 0
	}
	sort.Slice(out, func(i, j int) bool { return t(out[i]) > t(out[j]) })
	return out, int64(len(s.inbox[uid]))
}

type Stats struct {
	Users    int `json:"users"`
	Groups   int `json:"groups"`
	Messages int `json:"messages"`
}

func (s *Store) Stats() Stats {
	s.mu.RLock()
	defer s.mu.RUnlock()
	n := 0
	for _, m := range s.convMsgs {
		n += len(m)
	}
	return Stats{Users: len(s.users), Groups: len(s.groups), Messages: n}
}
