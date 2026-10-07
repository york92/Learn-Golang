// Package api 是 HTTP 接口层：账号、好友、群、会话、历史、同步，
// 以及 Web 客户端静态页面和 WebSocket 入口(/ws)。
package api

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net"
	"net/http"
	"strconv"
	"strings"

	"example.com/im/internal/gateway"
	"example.com/im/internal/protocol"
	"example.com/im/internal/service"
	"example.com/im/internal/store"
	"example.com/im/web"
)

type API struct {
	svc   *service.Service
	gw    *gateway.Server
	log   *slog.Logger
	limit *ipLimiter // 登录 / 注册限流
}

type envelope struct {
	Code int    `json:"code"`
	Msg  string `json:"msg,omitempty"`
	Data any    `json:"data,omitempty"`
}

// New 构建 HTTP 处理器。authPerMin 是每个来源 IP 每分钟允许的「注册 + 登录失败」次数（<=0 取默认 60）。
func New(svc *service.Service, gw *gateway.Server, wsAnyOrigin bool, authPerMin int, log *slog.Logger) http.Handler {
	if authPerMin <= 0 {
		authPerMin = 60
	}
	a := &API{svc: svc, gw: gw, log: log, limit: newIPLimiter(authPerMin, max(20, authPerMin/3))}
	mux := http.NewServeMux()

	mux.Handle("GET /{$}", http.FileServerFS(web.FS)) // 内嵌的 Web 客户端
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte("ok")) })
	mux.HandleFunc("GET /ws", gw.HandleWS(wsAnyOrigin))
	mux.HandleFunc("GET /api/stats", a.stats)

	mux.HandleFunc("POST /api/register", a.register)
	mux.HandleFunc("POST /api/login", a.login)
	mux.HandleFunc("POST /api/logout", a.auth(a.logout))
	mux.HandleFunc("GET /api/me", a.auth(a.me))

	mux.HandleFunc("GET /api/users/search", a.auth(a.searchUser))
	mux.HandleFunc("GET /api/users", a.auth(a.batchUsers))
	mux.HandleFunc("GET /api/friends", a.auth(a.listFriends))
	mux.HandleFunc("POST /api/friends", a.auth(a.addFriend))

	mux.HandleFunc("GET /api/groups", a.auth(a.listGroups))
	mux.HandleFunc("POST /api/groups", a.auth(a.createGroup))
	mux.HandleFunc("GET /api/groups/{id}", a.auth(a.getGroup))
	mux.HandleFunc("POST /api/groups/{id}/members", a.auth(a.addGroupMember))
	mux.HandleFunc("POST /api/groups/{id}/leave", a.auth(a.leaveGroup))

	mux.HandleFunc("GET /api/conversations", a.auth(a.conversations))
	mux.HandleFunc("GET /api/messages", a.auth(a.history))
	mux.HandleFunc("POST /api/messages", a.auth(a.sendMessage)) // HTTP 发消息（脚本/机器人/curl 测试）
	mux.HandleFunc("GET /api/sync", a.auth(a.sync))
	mux.HandleFunc("POST /api/read", a.auth(a.markRead))
	return a.logging(mux)
}

// ---------------------------------------------------------------- 中间件 / 工具

type userHandler func(w http.ResponseWriter, r *http.Request, uid int64)

func bearer(r *http.Request) string {
	h := r.Header.Get("Authorization")
	if len(h) > 7 && strings.EqualFold(h[:7], "bearer ") {
		return strings.TrimSpace(h[7:])
	}
	return ""
}

func (a *API) auth(h userHandler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		uid, ok := a.svc.St.ResolveToken(bearer(r))
		if !ok {
			fail(w, http.StatusUnauthorized, protocol.CodeAuthFailed, "missing or invalid token")
			return
		}
		h(w, r, uid)
	}
}

func (a *API) logging(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		a.log.Debug("http", "method", r.Method, "path", r.URL.Path, "remote", r.RemoteAddr)
		next.ServeHTTP(w, r)
	})
}

func clientIP(r *http.Request) string {
	ip, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return ip
}

// 注册：每次尝试都计数（防刷号）。登录：只有「失败」才计数（防暴力破解），成功登录不受限，
// 这样同一台机器/同一个反向代理后面的大量正常用户不会互相影响。
func (a *API) tooMany(w http.ResponseWriter) {
	fail(w, http.StatusTooManyRequests, protocol.CodeRateLimited, "too many attempts, slow down")
}

func ok(w http.ResponseWriter, data any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_ = json.NewEncoder(w).Encode(envelope{Code: 0, Data: data})
}

func fail(w http.ResponseWriter, status, code int, msg string) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(envelope{Code: code, Msg: msg})
}

// failErr 把业务错误映射成 HTTP 状态码 + 业务码。
func failErr(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, store.ErrBadCredentials):
		fail(w, http.StatusUnauthorized, protocol.CodeAuthFailed, err.Error())
	case errors.Is(err, store.ErrUserExists):
		fail(w, http.StatusConflict, 3001, err.Error())
	case errors.Is(err, store.ErrNotFound):
		fail(w, http.StatusNotFound, protocol.CodeNotFound, err.Error())
	case errors.Is(err, store.ErrForbidden), errors.Is(err, service.ErrRegisterClosed):
		fail(w, http.StatusForbidden, protocol.CodeNotInGroup, err.Error())
	case errors.Is(err, store.ErrInvalid):
		fail(w, http.StatusBadRequest, protocol.CodeBadRequest, err.Error())
	default:
		fail(w, http.StatusInternalServerError, protocol.CodeInternal, "internal error")
	}
}

func decode(w http.ResponseWriter, r *http.Request, v any) bool {
	r.Body = http.MaxBytesReader(w, r.Body, 64<<10)
	if err := json.NewDecoder(r.Body).Decode(v); err != nil {
		fail(w, http.StatusBadRequest, protocol.CodeBadRequest, "invalid JSON body")
		return false
	}
	return true
}

func qInt(r *http.Request, key string, def int64) int64 {
	if v, err := strconv.ParseInt(r.URL.Query().Get(key), 10, 64); err == nil {
		return v
	}
	return def
}

func pathID(w http.ResponseWriter, r *http.Request) (int64, bool) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		fail(w, http.StatusBadRequest, protocol.CodeBadRequest, "bad id")
		return 0, false
	}
	return id, true
}

// ---------------------------------------------------------------- 账号

func (a *API) register(w http.ResponseWriter, r *http.Request) {
	if !a.limit.Allow(clientIP(r)) {
		a.tooMany(w)
		return
	}
	var in struct{ Username, Nickname, Password string }
	if !decode(w, r, &in) {
		return
	}
	u, err := a.svc.Register(in.Username, in.Nickname, in.Password)
	if err != nil {
		failErr(w, err)
		return
	}
	ok(w, u)
}

func (a *API) login(w http.ResponseWriter, r *http.Request) {
	ip := clientIP(r)
	if !a.limit.Peek(ip) {
		a.tooMany(w)
		return
	}
	var in struct{ Username, Password string }
	if !decode(w, r, &in) {
		return
	}
	res, err := a.svc.Login(in.Username, in.Password)
	if err != nil {
		if errors.Is(err, store.ErrBadCredentials) {
			a.limit.Allow(ip) // 仅失败计数
		}
		failErr(w, err)
		return
	}
	ok(w, res)
}

func (a *API) logout(w http.ResponseWriter, r *http.Request, _ int64) {
	_ = a.svc.St.RevokeToken(bearer(r))
	ok(w, nil)
}

func (a *API) me(w http.ResponseWriter, _ *http.Request, uid int64) {
	u, _ := a.svc.St.GetUser(uid)
	ok(w, map[string]any{"user": u, "latest_user_seq": a.svc.St.LatestUserSeq(uid)})
}

func (a *API) searchUser(w http.ResponseWriter, r *http.Request, _ int64) {
	u, err := a.svc.ResolveUser(r.URL.Query().Get("username"), 0)
	if err != nil {
		failErr(w, err)
		return
	}
	ok(w, u)
}

func (a *API) batchUsers(w http.ResponseWriter, r *http.Request, _ int64) {
	var out []store.User
	for i, s := range strings.Split(r.URL.Query().Get("ids"), ",") {
		if i >= 100 {
			break
		}
		if id, err := strconv.ParseInt(strings.TrimSpace(s), 10, 64); err == nil {
			if u, ok := a.svc.St.GetUser(id); ok {
				out = append(out, u)
			}
		}
	}
	ok(w, out)
}

// ---------------------------------------------------------------- 好友

func (a *API) listFriends(w http.ResponseWriter, _ *http.Request, uid int64) {
	ok(w, a.svc.St.Friends(uid))
}

func (a *API) addFriend(w http.ResponseWriter, r *http.Request, uid int64) {
	var in struct {
		Username string `json:"username"`
		UID      int64  `json:"uid"`
	}
	if !decode(w, r, &in) {
		return
	}
	u, err := a.svc.AddFriend(uid, in.Username, in.UID)
	if err != nil {
		failErr(w, err)
		return
	}
	ok(w, u)
}

// ---------------------------------------------------------------- 群

func (a *API) listGroups(w http.ResponseWriter, _ *http.Request, uid int64) {
	gs := a.svc.St.GroupsOf(uid)
	out := make([]service.GroupView, 0, len(gs))
	for _, g := range gs {
		out = append(out, a.svc.GroupView(g))
	}
	ok(w, out)
}

func (a *API) createGroup(w http.ResponseWriter, r *http.Request, uid int64) {
	var in struct {
		Name            string   `json:"name"`
		Members         []int64  `json:"members"`
		MemberUsernames []string `json:"member_usernames"`
	}
	if !decode(w, r, &in) {
		return
	}
	g, err := a.svc.CreateGroup(uid, in.Name, in.Members, in.MemberUsernames)
	if err != nil {
		failErr(w, err)
		return
	}
	ok(w, a.svc.GroupView(g))
}

func (a *API) getGroup(w http.ResponseWriter, r *http.Request, uid int64) {
	id, good := pathID(w, r)
	if !good {
		return
	}
	g, found := a.svc.St.GetGroup(id)
	if !found || !g.HasMember(uid) {
		failErr(w, store.ErrNotFound)
		return
	}
	ok(w, a.svc.GroupView(g))
}

func (a *API) addGroupMember(w http.ResponseWriter, r *http.Request, uid int64) {
	id, good := pathID(w, r)
	if !good {
		return
	}
	var in struct {
		Username string `json:"username"`
		UID      int64  `json:"uid"`
	}
	if !decode(w, r, &in) {
		return
	}
	u, err := a.svc.ResolveUser(in.Username, in.UID)
	if err == nil {
		err = a.svc.St.AddGroupMember(id, uid, u.UID)
	}
	if err != nil {
		failErr(w, err)
		return
	}
	g, _ := a.svc.St.GetGroup(id)
	ok(w, a.svc.GroupView(g))
}

func (a *API) leaveGroup(w http.ResponseWriter, r *http.Request, uid int64) {
	id, good := pathID(w, r)
	if !good {
		return
	}
	if err := a.svc.St.LeaveGroup(id, uid); err != nil {
		failErr(w, err)
		return
	}
	ok(w, nil)
}

// ---------------------------------------------------------------- 会话 / 消息

func (a *API) conversations(w http.ResponseWriter, _ *http.Request, uid int64) {
	convs, latest := a.svc.Conversations(uid)
	if convs == nil {
		convs = []service.ConvView{}
	}
	ok(w, map[string]any{"latest_user_seq": latest, "conversations": convs})
}

func (a *API) history(w http.ResponseWriter, r *http.Request, uid int64) {
	msgs, err := a.svc.History(uid, r.URL.Query().Get("conv"), qInt(r, "before_seq", 0), int(qInt(r, "limit", 50)))
	if err != nil {
		failErr(w, err)
		return
	}
	ok(w, map[string]any{"messages": msgs})
}

func (a *API) sendMessage(w http.ResponseWriter, r *http.Request, uid int64) {
	var in struct {
		ClientMsgID string `json:"client_msg_id"`
		ToUID       int64  `json:"to_uid"`
		ToUsername  string `json:"to_username"`
		GroupID     int64  `json:"group_id"`
		Content     string `json:"content"`
	}
	if !decode(w, r, &in) {
		return
	}
	if in.ToUsername != "" && in.ToUID == 0 {
		u, err := a.svc.ResolveUser(in.ToUsername, 0)
		if err != nil {
			failErr(w, err)
			return
		}
		in.ToUID = u.UID
	}
	ack, err := a.svc.HandleSend(r.Context(), gateway.Session{UID: uid, DeviceID: "http"}, &protocol.SendReq{
		ClientMsgID: in.ClientMsgID, ToUID: in.ToUID, GroupID: in.GroupID, MsgType: 1, Content: in.Content,
	})
	if err != nil {
		failErr(w, err)
		return
	}
	if ack.Code != 0 {
		fail(w, http.StatusBadRequest, ack.Code, ack.Msg)
		return
	}
	ok(w, ack)
}

func (a *API) sync(w http.ResponseWriter, r *http.Request, uid int64) {
	resp, _ := a.svc.HandleSync(r.Context(), gateway.Session{UID: uid, DeviceID: "http"},
		&protocol.SyncReq{Since: qInt(r, "since", 0), Limit: int(qInt(r, "limit", 100))})
	ok(w, resp)
}

func (a *API) markRead(w http.ResponseWriter, r *http.Request, uid int64) {
	var in protocol.ReadReq
	if !decode(w, r, &in) {
		return
	}
	ack, err := a.svc.HandleRead(context.Background(), gateway.Session{UID: uid, DeviceID: "http"}, &in)
	if err != nil {
		failErr(w, err)
		return
	}
	if ack.Code != 0 {
		fail(w, http.StatusForbidden, ack.Code, "not a member of this conversation")
		return
	}
	ok(w, ack)
}

func (a *API) stats(w http.ResponseWriter, _ *http.Request) {
	ok(w, map[string]any{"gateway": a.gw.Stats(), "store": a.svc.St.Stats()})
}
