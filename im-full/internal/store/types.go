package store

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
)

var (
	ErrUserExists     = errors.New("username already exists")
	ErrBadCredentials = errors.New("invalid username or password")
	ErrNotFound       = errors.New("not found")
	ErrForbidden      = errors.New("forbidden")
	ErrInvalid        = errors.New("invalid argument")
)

type User struct {
	UID      int64  `json:"uid"`
	Username string `json:"username"`
	Nickname string `json:"nickname"`
	Salt     string `json:"salt,omitempty"`
	Hash     string `json:"hash,omitempty"`
	Created  int64  `json:"created"`
}

// Public 返回去掉密码字段的副本。
func (u *User) Public() User {
	c := *u
	c.Salt, c.Hash = "", ""
	return c
}

type Group struct {
	ID      int64   `json:"id"`
	Name    string  `json:"name"`
	Owner   int64   `json:"owner"`
	Members []int64 `json:"members"`
	Created int64   `json:"created"`
}

func (g *Group) HasMember(uid int64) bool {
	for _, m := range g.Members {
		if m == uid {
			return true
		}
	}
	return false
}

func (g *Group) clone() *Group {
	c := *g
	c.Members = append([]int64(nil), g.Members...)
	return &c
}

type Message struct {
	MsgID       int64  `json:"msg_id"`
	ConvID      string `json:"conv_id"`
	Seq         int64  `json:"seq"`
	From        int64  `json:"from"`
	Type        int    `json:"type"`
	Content     string `json:"content"`
	ClientMsgID string `json:"cmid,omitempty"`
	Time        int64  `json:"time"` // 毫秒
}

type InboxItem struct {
	UserSeq int64
	Msg     *Message
}

type NewMessage struct {
	ConvID      string
	From        int64
	Recipients  []int64 // 收件箱写入对象（单聊=双方，群聊=全体成员，含发送者）
	Type        int
	Content     string
	ClientMsgID string
}

type AppendResult struct {
	Msg      Message
	UserSeqs map[int64]int64 // 每个收件人的 user_seq
	Dup      bool            // 幂等命中：此前已处理过相同的 client_msg_id
}

type ConvSummary struct {
	ConvID  string
	Last    *Message
	ReadSeq int64
	Unread  int64
}

// ---- 会话 ID 约定：单聊 s:<小uid>:<大uid>；群聊 g:<gid>

func SingleConvID(a, b int64) string {
	if a > b {
		a, b = b, a
	}
	return fmt.Sprintf("s:%d:%d", a, b)
}

func GroupConvID(gid int64) string { return "g:" + strconv.FormatInt(gid, 10) }

// ParseConv 解析会话 ID。单聊返回双方 uid；群聊返回 gid。
func ParseConv(conv string) (isGroup bool, a, b int64, ok bool) {
	parts := strings.Split(conv, ":")
	switch {
	case len(parts) == 3 && parts[0] == "s":
		x, e1 := strconv.ParseInt(parts[1], 10, 64)
		y, e2 := strconv.ParseInt(parts[2], 10, 64)
		return false, x, y, e1 == nil && e2 == nil
	case len(parts) == 2 && parts[0] == "g":
		x, e := strconv.ParseInt(parts[1], 10, 64)
		return true, x, 0, e == nil
	}
	return false, 0, 0, false
}
