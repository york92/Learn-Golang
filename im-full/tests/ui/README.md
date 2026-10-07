# Web 客户端 UI 测试（可选）

需要 Node.js 18+。先启动一个**全新数据目录**的服务端，再运行测试：

```bash
./bin/imserver -http 127.0.0.1:18080 -tcp 127.0.0.1:19000 -data /tmp/im-ui-test &
cd tests/ui && npm install && npm test
```

或直接 `make test-ui`（自动起停服务端）。
