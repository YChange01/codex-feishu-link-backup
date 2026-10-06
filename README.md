# Codex Feishu Link — Remote Control for AI Coding Agents

**Codex Feishu Link** 把本机的 Codex、Claude Code 和 OpenCode 工作现场连接到飞书。它不只推送通知：你可以在手机上查看实时进度、接续桌面已有会话、切换工作区和模型设置，并在任务运行时继续发送指令。

> A full-featured Feishu (Lark) remote for OpenAI Codex, Claude Code, and OpenCode: continue desktop sessions, manage workspaces and permissions, steer running tasks, and monitor progress from your phone.

主要能力：

- **多后端接入**：连接本机 Codex、Claude Code 或 OpenCode；按需集成 VS Code，让飞书跟随电脑上的工作现场。
- **会话与工作区管理**：继续已有会话、创建会话、切换工作区，并查看历史记录和当前状态。
- **运行中远程协作**：查看实时进展、追加或排队指令、将后续消息并入当前任务，或停止执行。
- **模型与权限控制**：查看或切换模型、推理强度、权限模式和后端配置；支持独立配置 Profile。
- **飞书原生交互**：发送图片作为任务输入、收取文件、通过命令和菜单操作，并在消息线程中查看结果。
- **安装与维护**：提供 macOS、Windows、Linux 安装流程、Web 配置页、状态与配额查询及手动升级入口。

详细使用说明见 [用户使用说明书](./docs/general/user-guide.md)。

本仓库地址与主命令统一为 `codex-feishu-link`，产品名称为 **Codex Feishu Link**。源码从上游 v2.1.2 快照继续开发，包含桌面共享会话等本地扩展；来源见 [NOTICE](./NOTICE)。

升级兼容：旧命令 `codex-feishu-relay` 仍可使用；既有 `codex-feishu-relay` 配置、数据与日志目录、服务标识及 `CODEX_FEISHU_RELAY_*` 环境变量保持兼容，升级沿用原有凭据和安装状态。

## 安装

当前支持 Linux、macOS（Intel / Apple Silicon）和 Windows（amd64）。新仓库尚未发布二进制 Release 时，请使用源码安装；下面的安装器与在线脚本依赖本仓库已发布的 Release。

### 方式一：从源码安装

准备 Go（版本要求见 `go.mod`）、Node.js 22.12+、npm，以及已配置好的 Codex。

macOS / Linux：

```bash
git clone https://github.com/YChange01/codex-feishu-link.git
cd codex-feishu-link
cd web
npm ci
npm run build
cd ..
go build -o ./bin/codex-feishu-link ./cmd/codex-feishu-link
./bin/codex-feishu-link install -bootstrap-only -start-daemon
```

Windows PowerShell 可执行相同的克隆与前端构建步骤，最后两条改为：

```powershell
go build -o .\bin\codex-feishu-link.exe ./cmd/codex-feishu-link
.\bin\codex-feishu-link.exe install -bootstrap-only -start-daemon
```

打开命令输出中的 `/setup` 地址完成飞书接入。桌面共享后台适配是独立的可选能力，另见 [codex-desktop-link](./cmd/codex-desktop-link/README.md)。

### 方式二：原生安装器（发布 Release 后可用）

#### Windows

1. 从 [GitHub Releases](https://github.com/YChange01/codex-feishu-link/releases) 下载：

   ```text
   codex-feishu-link_<version>_windows_amd64_installer.exe
   ```

2. 双击运行安装器。
3. 首次安装完成后，安装器结果页会出现 **Continue WebSetup**，点击后进入 WebSetup 完成飞书接入。
4. 已经安装过时再次运行安装器，会按 repair / 升级处理，不会覆盖你已有的配置。

#### macOS

1. 从 [GitHub Releases](https://github.com/YChange01/codex-feishu-link/releases) 下载：

   ```text
   codex-feishu-link_<version>_darwin_universal_installer.dmg
   ```

2. 打开 DMG，运行其中的 **Install Codex Feishu Link.app**。
3. 首次安装可以选择安装目录；完成后在结果页打开 WebSetup 完成飞书接入。
4. 已安装过时再次运行，会按 repair / 升级处理。

### 方式三：在线脚本安装（发布 Release 后可用）

macOS / Linux：

```bash
curl -fsSL https://raw.githubusercontent.com/YChange01/codex-feishu-link/main/install-release.sh | bash
```

Windows PowerShell：

```powershell
irm https://raw.githubusercontent.com/YChange01/codex-feishu-link/main/install-release.ps1 | iex
```

安装最新 `beta` track：

```bash
curl -fsSL https://raw.githubusercontent.com/YChange01/codex-feishu-link/main/install-release.sh | bash -s -- --track beta
```

```powershell
& ([scriptblock]::Create((irm https://raw.githubusercontent.com/YChange01/codex-feishu-link/main/install-release.ps1))) -Track beta
```

安装指定版本：

```bash
curl -fsSL https://raw.githubusercontent.com/YChange01/codex-feishu-link/main/install-release.sh | bash -s -- --version v1.0.0
```

```powershell
& ([scriptblock]::Create((irm https://raw.githubusercontent.com/YChange01/codex-feishu-link/main/install-release.ps1))) -Version v1.0.0
```

脚本会自动识别平台、下载对应 release 包、安装并启动本地 daemon，然后打开或打印 WebSetup 地址。

### 方式四：手动解压 release 包

从 GitHub Releases 下载对应平台的二进制归档，解压后运行：

macOS / Linux：

```bash
./codex-feishu-link install -bootstrap-only -start-daemon
```

Windows PowerShell：

```powershell
.\codex-feishu-link.exe install -bootstrap-only -start-daemon
```

然后打开输出里的 `/setup` 地址完成初始化。

## 首次配置

安装完成后，WebSetup 会带你完成：

1. **运行环境检查**：确认本机 `codex` 可用、路径正确。
2. **连接飞书机器人**：扫码新建应用，或接入已有应用；WebSetup 会做只读配置检查，缺失权限会给出可复制的导入 JSON。
3. **飞书自动配置与菜单确认**：按页面提示处理。
4. **本机集成**：按需处理自动启动和 VS Code 集成；这两项都是可选的，确认后即可继续。
5. **完成**：进入本地管理页。

飞书应用不需要在安装前手动准备 `App ID` / `App Secret`；安装后通过 WebSetup 完成即可。

## 升级

日常升级直接在飞书会话里发送：

```text
/upgrade latest
```

查看或切换升级渠道：

```text
/upgrade track
```

daemon 不会在后台自动弹升级提示；升级只通过这条手动入口触发。

## 模型后端与配置

项目支持 **Codex**、**Claude Code** 和 **OpenCode** 后端：

- `/mode codex`（默认）：使用机器上已有的 Codex CLI、登录状态和配置
- `/mode claude`：使用机器上已有的 Claude Code（`claude` CLI）登录状态和配置
- `/mode opencode`：使用机器上已有的 OpenCode 环境

默认不需要你做任何额外模型配置，开箱就用机器上已经配好的环境。

如果你想在不影响现有配置的前提下，并行使用多套模型参数，也可以在管理页（Admin UI）中添加：

- **Codex API Profile**：独立设置 base URL、API key、模型、推理强度等
- **Claude Profile**：独立设置认证方式、base URL、模型、推理强度等
- **OpenCode Profile**：独立设置 OpenCode 后端参数

这些 profile 保存在 codex-feishu-link 自己的 `config.json` 里，不会改写 `~/.codex` 或 `~/.claude` 的原有配置，可以随时切换、并行使用。

在飞书里切换：

```text
/codexprofile
/claudeprofile
```

## 开始使用

最常用的入口：

- `/list`：选择或添加工作区
- `/use`：继续最近会话
- `/useall`：查看全部可见会话
- `/new`：在当前工作区新建会话
- `/history`：查看当前会话历史
- `/status`：查看当前状态、队列和模型配置
- `/quota`：查看当前 Codex 账户限额窗口的剩余比例和重置时间
- `/model`：切换 Codex 模型与推理强度
- `/permission`：调整当前后端支持的执行权限模式
- `/stop`：中断当前任务并清空队列
- `/detach`：断开当前接管
- `/sendfile`：把当前工作区里的文件发回飞书
- `/cron`：配置当前 daemon 的定时任务
- `/mode claude`：切换到 Claude Code 后端
- `/mode opencode`：切换到 OpenCode 后端
- `/codexprofile`：查看或切换 Codex Profile
- `/claudeprofile`：查看或切换 Claude Profile
- `/opencodeprofile`：查看或切换 OpenCode Profile
- `/help`：查看当前可用命令
- `/menu`：打开命令菜单首页

工作方式：

- 先发图片不会立刻触发请求，图片会暂存，等你下一条文字一起发给 Codex。
- 直接回复当前正在执行的那条消息，可以作为对当前任务的跟进；排队消息也可以点 `ThumbsUp` 升级成跟进。
- `/steerall` 可以把队列里可并入本轮执行的输入一次性并入。
- 最终回复会直接回在触发它的原始消息下面，群聊里更容易看懂上下文。

## VS Code（可选）

VS Code 接入是可选增强，不是开始使用的前提。如果你需要飞书跟随 VS Code 当前焦点：

1. 先退出 VS Code。
2. 在 WebSetup / Admin UI 中执行 VS Code 集成（当前统一使用 `managed_shim`）。
3. 完成后重新打开 VS Code。

旧版本的 `editor_settings` 接管方式已不再作为产品路径；检测到旧状态时会自动迁移到 `managed_shim`。

## 排障

先确认本地链路没有被代理污染：

```bash
unset http_proxy https_proxy HTTP_PROXY HTTPS_PROXY ALL_PROXY all_proxy
```

然后按顺序检查：

1. `curl --noproxy '*' -sf http://127.0.0.1:9501/api/admin/bootstrap-state`
2. `curl --noproxy '*' -sf http://127.0.0.1:9501/v1/status`
3. `config.json` 中的 `relay.serverURL`、飞书凭证和监听地址
4. daemon 日志（默认 `~/.local/share/codex-feishu-relay/logs/codex-feishu-relay-relayd.log`）

## 相关文档

- [用户使用说明书](./docs/general/user-guide.md)
- [飞书应用配置说明](./deploy/feishu/README.md)
- [文档索引](./docs/README.md)
- [变更记录](./CHANGELOG.md)

面向开发者：

- [开发者指南](./DEVELOPER.md)
- [安装与部署设计](./docs/general/install-deploy-design.md)
- [架构说明](./docs/general/architecture.md)

## 社区与项目政策

- [贡献指南](./CONTRIBUTING.md)
- [安全漏洞报告](./SECURITY.md)
- [行为准则](./CODE_OF_CONDUCT.md)
- [支持与帮助](./SUPPORT.md)
- [MIT License](./LICENSE)
