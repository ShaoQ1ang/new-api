# Trace Log

本地请求链路日志 SDK，默认关闭。开启后使用传入的 `X-Trace-ID` 将 New API 入参、转换后的上游请求、上游响应和公开响应追加到 JSONL 文件。

```bash
export TRACE_LOG_ENABLED=true
export TRACE_LOG_DIR=/tmp/aigc-traces
export TRACE_LOG_BUFFER_SIZE=256
```

New API 写入 `${TRACE_LOG_DIR}/newapi.jsonl`。关闭时不会创建目录、文件或后台任务，也不会执行延迟 Marshal 或添加 Trace Header。队列满时丢弃调试事件，不阻塞 Relay。

日志包含完整请求、响应和鉴权 Header，仅限本地测试。
