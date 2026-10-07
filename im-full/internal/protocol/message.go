package protocol

import "encoding/json"

// 命令字。按功能分段，方便扩展与做限流/鉴权策略。
const (
	CmdAuthReq      uint16 = 1
	CmdAuthResp     uint16 = 2
	CmdHeartbeat    uint16 = 3
	CmdHeartbeatAck uint16 = 4
	CmdSendReq      uint16 = 10 // 客户端发消息
	CmdSendAck      uint16 = 11 // 服务端落库确认（ServerAck）
	CmdPush         uint16 = 20 // 服务端推消息
	CmdPushAck      uint16 = 21 // 客户端收到确认（ClientAck）
	CmdKick         uint16 = 30 // 服务端要求断开（被踢 / 发版摘流）
	CmdSyncReq      uint16 = 40 // 增量同步：按 user_seq 游标拉取收件箱
	CmdSyncResp     uint16 = 41
	CmdReadReq      uint16 = 50 // 上报已读
	CmdReadAck      uint16 = 51
	CmdReadNotify   uint16 = 52 // 服务端通知：某会话已读位置变化（对端/本人其他设备）
	CmdErr          uint16 = 99
)

// 业务码
const (
	CodeOK          = 0
	CodeAuthFailed  = 1001
	CodeBadRequest  = 1002
	CodeRateLimited = 1003
	CodeNotFound    = 2001 // 用户 / 群不存在
	CodeNotFriend   = 2002
	CodeNotInGroup  = 2003
	CodeTooLong     = 2004
	CodeInternal    = 1500
)

// ErrBody 是 CmdErr 的消息体。seq 与触发它的请求相同，客户端据此结束等待。
type ErrBody struct {
	Code int    `json:"code"`
	Msg  string `json:"msg,omitempty"`
}

type AuthReq struct {
	Token    string `json:"token"`
	DeviceID string `json:"device_id"`
	Platform string `json:"platform"`
}

type AuthResp struct {
	Code         int    `json:"code"`
	Msg          string `json:"msg,omitempty"`
	HeartbeatSec int    `json:"heartbeat_sec"` // 服务端下发心跳间隔，便于动态调整
	ServerTime   int64  `json:"server_time"`   // 用于客户端校时（仅展示，不用于排序）
}

type SendReq struct {
	ClientMsgID string `json:"client_msg_id"`      // 客户端生成，幂等去重键
	ToUID       int64  `json:"to_uid,omitempty"`   // 单聊：对方 uid
	GroupID     int64  `json:"group_id,omitempty"` // 群聊：群 id（与 to_uid 二选一）
	MsgType     int    `json:"msg_type"`           // 1=文本
	Content     string `json:"content"`
}

type SendAck struct {
	ClientMsgID string `json:"client_msg_id"`
	Code        int    `json:"code"`
	Msg         string `json:"msg,omitempty"`
	MsgID       int64  `json:"msg_id"`
	Seq         int64  `json:"seq"` // 会话内序号
	ConvID      string `json:"conv_id,omitempty"`
	ServerTime  int64  `json:"server_time"`
}

// Push 既是实时推送的消息体，也是 sync / 历史接口返回的消息结构。
type Push struct {
	MsgID       int64  `json:"msg_id"`
	UserSeq     int64  `json:"user_seq"` // 收件箱序号（每个用户一条递增时间线）；历史接口里为 0
	Seq         int64  `json:"seq"`      // 会话内序号（用于排序、补洞、未读数）
	ConvID      string `json:"conv_id"`
	FromUID     int64  `json:"from_uid"`
	FromName    string `json:"from_name,omitempty"`
	MsgType     int    `json:"msg_type"`
	Content     string `json:"content"`
	SendTime    int64  `json:"send_time"`
	ClientMsgID string `json:"client_msg_id,omitempty"` // 仅发送者自己收到的回显里有
}

type SyncReq struct {
	Since int64 `json:"since"` // 客户端已处理到的 user_seq
	Limit int   `json:"limit"`
}

type SyncResp struct {
	Code    int    `json:"code"`
	Msgs    []Push `json:"msgs"`
	HasMore bool   `json:"has_more"`
	Latest  int64  `json:"latest"` // 服务端该用户当前最新 user_seq
}

type ReadReq struct {
	ConvID string `json:"conv_id"`
	Seq    int64  `json:"seq"`
}

type ReadAck struct {
	Code   int    `json:"code"`
	ConvID string `json:"conv_id"`
	Seq    int64  `json:"seq"`
}

type ReadNotify struct {
	ConvID string `json:"conv_id"`
	UID    int64  `json:"uid"` // 谁读的
	Seq    int64  `json:"seq"`
}

type PushAck struct {
	MsgID int64 `json:"msg_id"`
}

type Kick struct {
	Reason       string `json:"reason"`
	RetryAfterMs int    `json:"retry_after_ms"` // 发版摘流时带随机值，打散重连
}

// Marshal / Unmarshal 封装 body 编解码。
// 示例用 JSON 便于调试；生产换成 protobuf 只需改这两个函数。
func Marshal(v any) []byte {
	b, _ := json.Marshal(v)
	return b
}

func Unmarshal(b []byte, v any) error { return json.Unmarshal(b, v) }
