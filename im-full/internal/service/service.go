// Package service 是业务逻辑层：账号、好友、群、消息收发、同步、已读。
// 它实现 gateway.Backend，所以网关只管连接，所有"业务规则"都在这里。
package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"example.com/im/internal/gateway"
	"example.com/im/internal/protocol"
	"example.com/im/internal/store"
)

type Config struct {
	AllowRegister bool          // 是否开放注册
	RequireFriend bool          // 单聊是否要求双方是好友
	TokenTTL      time.Duration // 登录令牌有效期
	MaxContent    int           // 单条消息内容上限（字节）
}

// Pusher 由网关实现：把消息送到用户的在线连接上。
type Pusher interface {
	PushToUser(uid int64, exceptDevice string, body []byte) int
	NotifyUser(uid int64, exceptDevice string, cmd uint16, body []byte) int
}

type Service struct {
	St     *store.Store
	Cfg    Config
	pusher Pusher
}

func New(st *store.Store, cfg Config) *Service {
	if cfg.TokenTTL <= 0 {
		cfg.TokenTTL = 30 * 24 * time.Hour
	}
	if cfg.MaxContent <= 0 {
		cfg.MaxContent = 4096
	}
	return &Service{St: st, Cfg: cfg}
}

func (s *Service) SetPusher(p Pusher) { s.pusher = p }

var _ gateway.Backend = (*Service)(nil)

// ---------------------------------------------------------------- gateway.Backend

func (s *Service) Authenticate(_ context.Context, token string) (int64, error) {
	if uid, ok := s.St.ResolveToken(token); ok {
		return uid, nil
	}
	return 0, errors.New("invalid or expired token")
}

func (s *Service) HandleSend(_ context.Context, sess gateway.Session, req *protocol.SendReq) (*protocol.SendAck, error) {
	ack := &protocol.SendAck{ClientMsgID: req.ClientMsgID}
	fail := func(code int, msg string) (*protocol.SendAck, error) {
		ack.Code, ack.Msg = code, msg
		return ack, nil
	}
	if strings.TrimSpace(req.Content) == "" {
		return fail(protocol.CodeBadRequest, "empty content")
	}
	if len(req.Content) > s.Cfg.MaxContent || !utf8.ValidString(req.Content) {
		return fail(protocol.CodeTooLong, fmt.Sprintf("content invalid or longer than %d bytes", s.Cfg.MaxContent))
	}
	if req.MsgType == 0 {
		req.MsgType = 1
	}

	var conv string
	var rcpt []int64
	switch {
	case req.GroupID != 0:
		g, ok := s.St.GetGroup(req.GroupID)
		if !ok {
			return fail(protocol.CodeNotFound, "group not found")
		}
		if !g.HasMember(sess.UID) {
			return fail(protocol.CodeNotInGroup, "you are not a member of this group")
		}
		conv, rcpt = store.GroupConvID(g.ID), g.Members
	case req.ToUID != 0:
		if req.ToUID == sess.UID {
			return fail(protocol.CodeBadRequest, "cannot send to yourself")
		}
		if _, ok := s.St.GetUser(req.ToUID); !ok {
			return fail(protocol.CodeNotFound, "user not found")
		}
		if s.Cfg.RequireFriend && !s.St.AreFriends(sess.UID, req.ToUID) {
			return fail(protocol.CodeNotFriend, "not friends")
		}
		conv, rcpt = store.SingleConvID(sess.UID, req.ToUID), []int64{sess.UID, req.ToUID}
	default:
		return fail(protocol.CodeBadRequest, "to_uid or group_id required")
	}

	// ① 先落库（幂等、分配 seq、写收件箱）。落库成功才算"发送成功"。
	res, err := s.St.AppendMessage(store.NewMessage{
		ConvID: conv, From: sess.UID, Recipients: rcpt,
		Type: req.MsgType, Content: req.Content, ClientMsgID: req.ClientMsgID,
	})
	if err != nil {
		return nil, err
	}
	ack.MsgID, ack.Seq, ack.ConvID, ack.ServerTime = res.Msg.MsgID, res.Msg.Seq, conv, res.Msg.Time

	// ② 再推送。幂等命中（重发）时不再重复推送。
	//    推送失败/收件人不在线都没关系：消息已在收件箱，对方上线后 sync 补齐。
	if !res.Dup && s.pusher != nil {
		s.fanout(&res, sess.UID, req.ClientMsgID)
	}
	return ack, nil
}

// fanout 给每个收件人推送各自 user_seq 的版本；发送者自己（所有设备）也会收到回显，
// 这样多端同步"我发的消息"与"收别人的消息"走的是同一条路径。
func (s *Service) fanout(res *store.AppendResult, sender int64, cmid string) {
	name := s.nickname(sender)
	for uid, us := range res.UserSeqs {
		p := toPush(&res.Msg, us, name)
		if uid == sender {
			p.ClientMsgID = cmid
		}
		s.pusher.PushToUser(uid, "", protocol.Marshal(p))
	}
}

func (s *Service) HandleSync(_ context.Context, sess gateway.Session, req *protocol.SyncReq) (*protocol.SyncResp, error) {
	items, latest, more := s.St.Sync(sess.UID, req.Since, req.Limit)
	names := map[int64]string{}
	out := make([]protocol.Push, 0, len(items))
	for _, it := range items {
		n, ok := names[it.Msg.From]
		if !ok {
			n = s.nickname(it.Msg.From)
			names[it.Msg.From] = n
		}
		out = append(out, *toPush(it.Msg, it.UserSeq, n))
	}
	return &protocol.SyncResp{Code: protocol.CodeOK, Msgs: out, HasMore: more, Latest: latest}, nil
}

func (s *Service) HandleRead(_ context.Context, sess gateway.Session, req *protocol.ReadReq) (*protocol.ReadAck, error) {
	seq, changed, err := s.St.MarkRead(sess.UID, req.ConvID, req.Seq)
	if errors.Is(err, store.ErrForbidden) {
		return &protocol.ReadAck{Code: protocol.CodeNotInGroup, ConvID: req.ConvID}, nil
	}
	if err != nil {
		return nil, err
	}
	if changed && s.pusher != nil {
		body := protocol.Marshal(&protocol.ReadNotify{ConvID: req.ConvID, UID: sess.UID, Seq: seq})
		// 本人的其他设备：同步红点；单聊对方：展示"已读"。
		s.pusher.NotifyUser(sess.UID, sess.DeviceID, protocol.CmdReadNotify, body)
		if isGroup, a, b, ok := store.ParseConv(req.ConvID); ok && !isGroup {
			peer := a
			if a == sess.UID {
				peer = b
			}
			s.pusher.NotifyUser(peer, "", protocol.CmdReadNotify, body)
		}
	}
	return &protocol.ReadAck{Code: protocol.CodeOK, ConvID: req.ConvID, Seq: seq}, nil
}

func (s *Service) OnPushAck(context.Context, gateway.Session, int64) {}

// ---------------------------------------------------------------- helpers

func (s *Service) nickname(uid int64) string {
	if u, ok := s.St.GetUser(uid); ok {
		return u.Nickname
	}
	return ""
}

func toPush(m *store.Message, userSeq int64, fromName string) *protocol.Push {
	return &protocol.Push{
		MsgID: m.MsgID, UserSeq: userSeq, Seq: m.Seq, ConvID: m.ConvID, FromUID: m.From,
		FromName: fromName, MsgType: m.Type, Content: m.Content, SendTime: m.Time,
	}
}
