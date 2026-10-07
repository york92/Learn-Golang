const { JSDOM } = require("jsdom");
const fs = require("fs");
const BASE = process.env.IM_BASE || "http://127.0.0.1:18080";
const html = fs.readFileSync(require("path").join(__dirname, "../../web/index.html"), "utf8");
const sleep = ms => new Promise(r => setTimeout(r, ms));
let failed = 0;
const check = (name, cond, extra="") => { console.log((cond ? "PASS " : "FAIL ") + name + (cond ? "" : "  -> " + extra)); if (!cond) failed++; };

async function tab(name) {
  // 每个 JSDOM 实例 = 一个浏览器标签页（独立的 sessionStorage）
  const dom = new JSDOM(html, { url: BASE + "/", runScripts: "dangerously", pretendToBeVisual: true,
    beforeParse(w) {
      w.fetch = (u, o) => fetch(new URL(u, BASE), o);
      w.TextEncoder = TextEncoder; w.TextDecoder = TextDecoder;
      Object.defineProperty(w, "crypto", { value: require("crypto").webcrypto });
      w.prompt = () => null; w.confirm = () => true;
    } });
  const w = dom.window, d = w.document;
  w.__name = name;
  return { w, d, $: id => d.getElementById(id) };
}
async function register(t, u) {
  t.$("u").value = u; t.$("p").value = "secret1"; t.$("n").value = u;
  t.$("register").click();
  for (let i = 0; i < 50 && t.$("app").style.display !== "flex"; i++) await sleep(100);
}
const dotOn = t => t.$("dot").classList.contains("on");

(async () => {
  const A = await tab("alice"), B = await tab("bob");
  await register(A, "ui_alice"); await register(B, "ui_bob");
  check("alice 登录并进入主界面", A.$("app").style.display === "flex", A.$("err").textContent);
  check("bob 登录并进入主界面", B.$("app").style.display === "flex", B.$("err").textContent);
  for (let i = 0; i < 30 && !(dotOn(A) && dotOn(B)); i++) await sleep(100);
  check("两个标签页的 WebSocket 都已连接并认证", dotOn(A) && dotOn(B));

  // alice 发起与 bob 的聊天（newChat 里用 prompt 取用户名）
  A.w.prompt = () => "ui_bob";
  A.$("tools").querySelector("button").click();           // "+ 新聊天"
  for (let i = 0; i < 30 && A.$("text").disabled; i++) await sleep(100);
  check("alice 打开了与 bob 的会话", A.$("title").textContent === "ui_bob", A.$("title").textContent);

  A.$("text").value = "你好 bob <b>xss</b>"; A.$("send").click();
  await sleep(800);
  const bubA = [...A.$("msgs").querySelectorAll(".bub")].map(e => e.textContent);
  check("alice 看到自己发的消息（回显去重后只有一条）", bubA.length === 1 && bubA[0] === "你好 bob <b>xss</b>", JSON.stringify(bubA));
  check("消息内容按纯文本渲染（无 XSS）", A.$("msgs").querySelector(".bub b") === null);

  // bob 实时收到：会话列表出现新会话 + 未读红点
  await sleep(500);
  const itemB = B.$("list").querySelector(".item");
  check("bob 会话列表实时出现 alice 的会话", !!itemB && itemB.textContent.includes("ui_alice"), B.$("list").textContent);
  check("bob 看到未读红点 1", !!itemB && itemB.querySelector(".badge") && itemB.querySelector(".badge").textContent === "1");

  // bob 打开会话 → 已读 → alice 看到“已读”
  itemB.click(); await sleep(900);
  const bubB = [...B.$("msgs").querySelectorAll(".bub")].map(e => e.textContent);
  check("bob 打开会话看到消息", bubB.length === 1 && bubB[0].startsWith("你好 bob"), JSON.stringify(bubB));
  check("bob 红点清零", !B.$("list").querySelector(".badge"));
  check("alice 端显示“已读”回执", A.$("msgs").textContent.includes("已读"), A.$("msgs").textContent);

  // bob 回复
  B.$("text").value = "收到"; B.$("send").click(); await sleep(800);
  check("alice 实时收到 bob 的回复", A.$("msgs").textContent.includes("收到"));

  // 断线重连 + 离线补消息：关掉 bob 的 WebSocket，alice 再发两条，bob 重连后应自动 sync 补齐
  B.w.eval("window.__c=connect; connect=()=>setTimeout(window.__c,2500); S.ws.close()"); // 强制离线 2.5 秒
  await sleep(200);
  check("bob 断线后状态灯变红", !dotOn(B));
  A.$("text").value = "离线1"; A.$("send").click(); await sleep(200);
  A.$("text").value = "离线2"; A.$("send").click();
  await sleep(500);
  check("离线期间 bob 确实没收到（只能靠 sync 补）", !B.$("msgs").textContent.includes("离线1"));
  for (let i = 0; i < 60 && !dotOn(B); i++) await sleep(100);
  check("bob 自动重连成功", dotOn(B));
  await sleep(1000);
  const txtB = B.$("msgs").textContent;
  check("bob 重连后自动补齐离线消息（按顺序）", txtB.includes("离线1") && txtB.includes("离线2") && txtB.indexOf("离线1") < txtB.indexOf("离线2"), txtB);

  // 群聊：通过 HTTP 建群（UI 的 prompt 流程），alice 发群消息，bob 收到
  A.w.prompt = (q) => q.includes("群名称") ? "测试群" : "ui_bob";
  A.$("tabs").querySelector('[data-t="groups"]').click();
  A.$("tools").querySelector("button").click();
  for (let i = 0; i < 30 && A.$("title").textContent !== "测试群"; i++) await sleep(100);
  check("alice 创建群并进入群会话", A.$("title").textContent === "测试群", A.$("title").textContent);
  A.$("text").value = "群里好"; A.$("send").click(); await sleep(1000);
  B.$("tabs").querySelector('[data-t="convs"]').click();
  const gItem = [...B.$("list").querySelectorAll(".item")].find(e => e.textContent.includes("测试群"));
  check("bob 会话列表里出现该群并有未读", !!gItem && !!gItem.querySelector(".badge"), B.$("list").textContent);
  gItem && gItem.click(); await sleep(800);
  check("bob 在群里看到消息和发送者名字", B.$("msgs").textContent.includes("群里好") && B.$("msgs").textContent.includes("ui_alice"));

  console.log(failed ? `\n${failed} 项失败` : "\n全部通过");
  process.exit(failed ? 1 : 0);
})().catch(e => { console.error("ERR", e); process.exit(2); });
