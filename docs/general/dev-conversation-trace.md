# 开发态会话 Trace 日志

> Type: `general`
> Updated: `2026-10-07`
> Summary: 公开命令和产品名称统一为 Codex Feishu Link，既有安装路径与环境变量继续保持兼容。

## 1. 适用范围

这套 trace 只用于开发排障和离线复盘，不对公开发布产物开放。

- 默认构建（包括 release/beta 公共产物）不会启用真实 trace writer
- 只有本地显式使用 `devtrace` build tag 构建时，才会写入 trace 文件

## 2. 如何启用

示例（本地开发构建）：

```bash
go build -tags devtrace ./cmd/codex-feishu-link
```

未带 `devtrace` tag 时，trace 逻辑会走空实现，不会落盘。

## 3. 日志路径

文件名固定为：

- `codex-feishu-relay-conversation-trace.ndjson`

位置在运行时 logs 目录下：

- Linux 默认：`~/.local/share/codex-feishu-relay/logs/`
- 若自定义了 XDG 路径，则跟随运行时 `Paths.LogsDir`

## 4. 记录格式

按行 NDJSON（每行一个 JSON 对象），append-only。

核心字段：

- `ts`: UTC 时间戳
- `event`: 事件类型
- `actor`: `user` / `assistant`
- `surfaceSessionId` / `chatId` / `messageId`（可得时）
- `instanceId` / `threadId` / `turnId`（可得时）
- `text`（消息正文或错误信息）
- `status`（turn 生命周期状态）
- `final`（assistant 文本是否 final）

## 5. 事件类型

当前固定为：

- `user_message`
- `steer_message`
- `assistant_text`
- `turn_started`
- `turn_completed`

说明：

- `steer_message` 单独成类，不与普通 `user_message` 混合
- `assistant_text` 以“实际投递成功的可见文本”为准
- `turn_*` 只记录非 internal-helper 的生命周期事件

## 6. 复盘建议

常见读法：

1. 先按 `threadId + turnId` 聚合
2. 再按 `ts` 排序
3. 重点对照：
   - `user_message` -> `turn_started` 的间隔
   - `steer_message` 插入时点（中途纠偏）
   - `assistant_text` 的非 final / final 演进
   - `turn_completed.status` 与 `text`（失败信息）

这样可以快速还原“用户看到了什么、assistant 在该 turn 内实际输出了什么、以及什么时候被 steer”。
