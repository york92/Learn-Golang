// imcli：命令行客户端，用于调试与冒烟测试。
//
//	imcli register -u alice -p 123456
//	imcli chat     -u alice -p 123456            交互式聊天
//	imcli send     -u alice -p 123456 -to bob -text hi    发一条后退出
//	imcli pull     -u bob   -p 123456            拉取全部收件箱后退出（验证离线消息）
package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"time"

	"example.com/im/internal/client"
	"example.com/im/internal/protocol"
)

type common struct {
	httpURL, tcp, user, pass string
}

func (c *common) bind(fs *flag.FlagSet) {
	fs.StringVar(&c.httpURL, "http", "http://127.0.0.1:8080", "HTTP API 地址")
	fs.StringVar(&c.tcp, "tcp", "127.0.0.1:9000", "TCP 网关地址")
	fs.StringVar(&c.user, "u", "", "用户名")
	fs.StringVar(&c.pass, "p", "", "密码")
}

func main() {
	if len(os.Args) < 2 {
		usage()
	}
	cmd, args := os.Args[1], os.Args[2:]
	fs := flag.NewFlagSet(cmd, flag.ExitOnError)
	var c common
	c.bind(fs)
	switch cmd {
	case "register":
		nick := fs.String("nick", "", "昵称")
		fs.Parse(args)
		var out struct {
			UID      int64  `json:"uid"`
			Username string `json:"username"`
		}
		must(c.call("POST", "/api/register", "", map[string]any{"username": c.user, "password": c.pass, "nickname": *nick}, &out))
		fmt.Printf("注册成功: username=%s uid=%d\n", out.Username, out.UID)
	case "send":
		to := fs.String("to", "", "对方用户名")
		group := fs.Int64("group", 0, "群 id（与 -to 二选一）")
		text := fs.String("text", "", "内容")
		fs.Parse(args)
		cl, cancel := c.connect(0, nil)
		defer cancel()
		req := protocol.SendReq{Content: *text, GroupID: *group}
		if *to != "" {
			req.ToUID = c.uidOf(c.login().Token, *to)
		}
		ctx, cc := context.WithTimeout(context.Background(), 10*time.Second)
		defer cc()
		ack, err := cl.Send(ctx, req)
		must(err)
		fmt.Printf("sent ✓ msg_id=%d conv=%s seq=%d\n", ack.MsgID, ack.ConvID, ack.Seq)
	case "pull":
		fs.Parse(args)
		cl, cancel := c.connect(0, func(p *protocol.Push) {
			fmt.Printf("[%s #%d] %s: %s\n", p.ConvID, p.Seq, p.FromName, p.Content)
		})
		defer cancel()
		time.Sleep(1500 * time.Millisecond) // 给 sync 留出时间
		fmt.Printf("同步完成，游标 user_seq=%d\n", cl.Cursor())
	case "chat":
		fs.Parse(args)
		c.chat()
	default:
		usage()
	}
}

func usage() {
	fmt.Fprintln(os.Stderr, "用法: imcli <register|chat|send|pull> [flags]   使用 -h 查看各命令参数")
	os.Exit(2)
}

func must(err error) {
	if err != nil {
		fmt.Fprintln(os.Stderr, "错误:", err)
		os.Exit(1)
	}
}

// ---------------------------------------------------------------- HTTP 辅助

func (c *common) call(method, path, token string, in, out any) error {
	var body io.Reader
	if in != nil {
		b, _ := json.Marshal(in)
		body = bytes.NewReader(b)
	}
	req, _ := http.NewRequest(method, c.httpURL+path, body)
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	var env struct {
		Code int             `json:"code"`
		Msg  string          `json:"msg"`
		Data json.RawMessage `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&env); err != nil {
		return fmt.Errorf("bad response (HTTP %d)", resp.StatusCode)
	}
	if env.Code != 0 {
		return fmt.Errorf("%s (code %d)", env.Msg, env.Code)
	}
	if out != nil {
		return json.Unmarshal(env.Data, out)
	}
	return nil
}

type loginInfo struct {
	Token string `json:"token"`
	User  struct {
		UID int64 `json:"uid"`
	} `json:"user"`
}

func (c *common) login() loginInfo {
	var li loginInfo
	must(c.call("POST", "/api/login", "", map[string]any{"username": c.user, "password": c.pass}, &li))
	return li
}

func (c *common) uidOf(token, username string) int64 {
	var u struct {
		UID int64 `json:"uid"`
	}
	must(c.call("GET", "/api/users/search?username="+url.QueryEscape(username), token, nil, &u))
	return u.UID
}

func (c *common) connect(cursor int64, onPush func(*protocol.Push)) (*client.Client, context.CancelFunc) {
	li := c.login()
	cl := client.New(client.Options{Addr: c.tcp, Token: li.Token, DeviceID: "cli-" + strconv.Itoa(os.Getpid()),
		Cursor: cursor, OnPush: onPush})
	ctx, cancel := context.WithCancel(context.Background())
	go cl.Run(ctx)
	if !cl.WaitConnected(5 * time.Second) {
		cancel()
		must(fmt.Errorf("无法连接 TCP 网关 %s", c.tcp))
	}
	return cl, cancel
}

// ---------------------------------------------------------------- 交互聊天

func (c *common) chat() {
	li := c.login()
	fmt.Printf("已登录 uid=%d。输入 /help 查看命令\n", li.User.UID)
	cl := client.New(client.Options{
		Addr: c.tcp, Token: li.Token, DeviceID: "cli-" + strconv.Itoa(os.Getpid()),
		OnPush: func(p *protocol.Push) {
			who := p.FromName
			if p.FromUID == li.User.UID {
				who = "我"
			}
			fmt.Printf("\r[%s #%d] %s: %s\n> ", p.ConvID, p.Seq, who, p.Content)
		},
		OnReadNotify: func(n *protocol.ReadNotify) {
			fmt.Printf("\r[已读] %s uid=%d 读到 #%d\n> ", n.ConvID, n.UID, n.Seq)
		},
		OnState: func(ok bool) { fmt.Printf("\r[连接状态: %v]\n> ", ok) },
	})
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	go cl.Run(ctx)

	sc := bufio.NewScanner(os.Stdin)
	fmt.Print("> ")
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		switch {
		case line == "":
		case line == "/quit":
			return
		case line == "/help":
			fmt.Println("/to <用户名> <内容>   发单聊\n/g <群id> <内容>      发群聊\n/friends              好友列表\n/groups               群列表\n/addfriend <用户名>   加好友\n/newgroup <名称> <用户名,用户名>  建群\n/quit                 退出")
		case line == "/friends":
			var fr []struct {
				UID                int64
				Username, Nickname string
			}
			must(c.call("GET", "/api/friends", li.Token, nil, &fr))
			for _, f := range fr {
				fmt.Printf("  %s (uid=%d)\n", f.Username, f.UID)
			}
		case line == "/groups":
			var gs []struct {
				ID      int64  `json:"id"`
				Name    string `json:"name"`
				Members []int64
			}
			must(c.call("GET", "/api/groups", li.Token, nil, &gs))
			for _, g := range gs {
				fmt.Printf("  群 %d: %s (%d 人)\n", g.ID, g.Name, len(g.Members))
			}
		case strings.HasPrefix(line, "/addfriend "):
			err := c.call("POST", "/api/friends", li.Token, map[string]any{"username": strings.TrimSpace(line[11:])}, nil)
			report(err, "已添加好友")
		case strings.HasPrefix(line, "/newgroup "):
			parts := strings.Fields(line[10:])
			if len(parts) != 2 {
				fmt.Println("用法: /newgroup <名称> <用户名,用户名>")
				break
			}
			err := c.call("POST", "/api/groups", li.Token, map[string]any{"name": parts[0], "member_usernames": strings.Split(parts[1], ",")}, nil)
			report(err, "群已创建，用 /groups 查看 id")
		case strings.HasPrefix(line, "/to "), strings.HasPrefix(line, "/g "):
			parts := strings.SplitN(line, " ", 3)
			if len(parts) < 3 {
				fmt.Println("用法: /to <用户名> <内容> 或 /g <群id> <内容>")
				break
			}
			var req protocol.SendReq
			req.Content = parts[2]
			if parts[0] == "/g" {
				req.GroupID, _ = strconv.ParseInt(parts[1], 10, 64)
			} else {
				var u struct {
					UID int64 `json:"uid"`
				}
				if err := c.call("GET", "/api/users/search?username="+url.QueryEscape(parts[1]), li.Token, nil, &u); err != nil {
					fmt.Println("错误:", err)
					break
				}
				req.ToUID = u.UID
			}
			sctx, cancel := context.WithTimeout(ctx, 10*time.Second)
			ack, err := cl.Send(sctx, req)
			cancel()
			if err != nil {
				fmt.Println("发送失败:", err)
			} else {
				fmt.Printf("  ✓ 已发送 seq=%d\n", ack.Seq)
			}
		default:
			fmt.Println("未知命令，输入 /help")
		}
		fmt.Print("> ")
	}
}

func report(err error, okMsg string) {
	if err != nil {
		fmt.Println("错误:", err)
	} else {
		fmt.Println(okMsg)
	}
}
