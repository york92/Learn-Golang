package service

import (
	"errors"
	"fmt"
	"strings"

	"example.com/im/internal/protocol"
	"example.com/im/internal/store"
)

// 这些方法供 HTTP API 调用（账号、好友、群、会话列表、历史）。

var ErrRegisterClosed = errors.New("registration is disabled")

type LoginResult struct {
	User    store.User `json:"user"`
	Token   string     `json:"token"`
	Expires int64      `json:"expires_at"` // unix 秒
}

func (s *Service) Register(username, nickname, password string) (*store.User, error) {
	if !s.Cfg.AllowRegister {
		return nil, ErrRegisterClosed
	}
	return s.St.CreateUser(strings.TrimSpace(username), strings.TrimSpace(nickname), password)
}

func (s *Service) Login(username, password string) (*LoginResult, error) {
	u, err := s.St.Verify(strings.TrimSpace(username), password)
	if err != nil {
		return nil, err
	}
	tok, exp, err := s.St.IssueToken(u.UID, s.Cfg.TokenTTL)
	if err != nil {
		return nil, err
	}
	return &LoginResult{User: *u, Token: tok, Expires: exp.Unix()}, nil
}

// ResolveUser 支持用 username 或 uid 定位用户。
func (s *Service) ResolveUser(username string, uid int64) (store.User, error) {
	if uid != 0 {
		if u, ok := s.St.GetUser(uid); ok {
			return u, nil
		}
	} else if u, ok := s.St.FindUser(strings.TrimSpace(username)); ok {
		return u, nil
	}
	return store.User{}, store.ErrNotFound
}

func (s *Service) AddFriend(me int64, username string, uid int64) (store.User, error) {
	u, err := s.ResolveUser(username, uid)
	if err != nil {
		return u, err
	}
	// 演示版：对方无需确认，直接互为好友。生产需要"申请 → 同意"流程。
	return u, s.St.AddFriend(me, u.UID)
}

func (s *Service) CreateGroup(me int64, name string, uids []int64, usernames []string) (*store.Group, error) {
	members := append([]int64(nil), uids...)
	for _, n := range usernames {
		u, ok := s.St.FindUser(strings.TrimSpace(n))
		if !ok {
			return nil, fmt.Errorf("%w: user %q", store.ErrNotFound, n)
		}
		members = append(members, u.UID)
	}
	return s.St.CreateGroup(me, strings.TrimSpace(name), members)
}

type GroupView struct {
	*store.Group
	MemberInfo []store.User `json:"member_info"`
}

func (s *Service) GroupView(g *store.Group) GroupView {
	v := GroupView{Group: g}
	for _, m := range g.Members {
		if u, ok := s.St.GetUser(m); ok {
			v.MemberInfo = append(v.MemberInfo, u)
		}
	}
	return v
}

// ConvView 是会话列表的一项（已补全展示所需的名称等信息）。
type ConvView struct {
	ConvID      string         `json:"conv_id"`
	Type        string         `json:"type"` // single | group
	Title       string         `json:"title"`
	PeerUID     int64          `json:"peer_uid,omitempty"`
	GroupID     int64          `json:"group_id,omitempty"`
	Last        *protocol.Push `json:"last,omitempty"`
	ReadSeq     int64          `json:"read_seq"`
	Unread      int64          `json:"unread"`
	PeerReadSeq int64          `json:"peer_read_seq,omitempty"` // 单聊：对方已读到的位置
}

func (s *Service) Conversations(me int64) (convs []ConvView, latestUserSeq int64) {
	sums, latest := s.St.Conversations(me)
	for _, cs := range sums {
		v := ConvView{ConvID: cs.ConvID, ReadSeq: cs.ReadSeq, Unread: cs.Unread}
		isGroup, a, b, ok := store.ParseConv(cs.ConvID)
		if !ok {
			continue
		}
		if isGroup {
			g, ok := s.St.GetGroup(a)
			if !ok {
				continue
			}
			v.Type, v.Title, v.GroupID = "group", g.Name, g.ID
		} else {
			peer := a
			if a == me {
				peer = b
			}
			v.Type, v.PeerUID = "single", peer
			if u, ok := s.St.GetUser(peer); ok {
				v.Title = u.Nickname
			}
			v.PeerReadSeq = s.St.ReadSeq(peer, cs.ConvID)
		}
		if cs.Last != nil {
			v.Last = toPush(cs.Last, 0, s.nickname(cs.Last.From))
		}
		convs = append(convs, v)
	}
	return convs, latest
}

// History 返回会话历史。仅会话成员可读。
func (s *Service) History(me int64, conv string, beforeSeq int64, limit int) ([]protocol.Push, error) {
	if !s.St.InConv(me, conv) {
		return nil, store.ErrForbidden
	}
	msgs := s.St.History(conv, beforeSeq, limit)
	names := map[int64]string{}
	out := make([]protocol.Push, 0, len(msgs))
	for i := range msgs {
		n, ok := names[msgs[i].From]
		if !ok {
			n = s.nickname(msgs[i].From)
			names[msgs[i].From] = n
		}
		out = append(out, *toPush(&msgs[i], 0, n))
	}
	return out, nil
}
