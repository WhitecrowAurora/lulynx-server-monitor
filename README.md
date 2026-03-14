# 探针（轻量多服务器监控）

特点：
- Linux 客户端（探针）采集 `/proc` 指标，HTTP Push 到中心端（无 WebSocket）。
- 中心端提供面板（卡片 + 每项小网格趋势线）和简单历史接口。
- 支持可选端口探活（每台服务器配置后在卡片底部显示绿/红点）。
- 支持可选加密上报（AES-GCM，基于 `ingest_token` 派生密钥；无需 HTTPS 也能避免明文泄露密码/指标）。
- 支持“静默拒绝”未授权上报：未带正确密码/密钥的请求会直接断开连接（减少被扫描指纹）。
- 支持可选自助注册（Enroll）：客户端用 `enroll_token`（接入密码/中心密码）换取中心端下发的“单机上报密码”（并自动写回配置），降低共享密码泄露风险。
- 支持可选“主动受控模式”：中心端可主动连客户端的控制端口（默认 38088）进行探测/下发；未连通时卡片会高斯模糊提示。

## 快速开始

## 一键部署（SSH 交互脚本，Linux）

把 `run.sh` 和对应二进制放到同一目录后：

- 部署中心端：
  - `chmod +x run.sh tanzhen-center`
  - `sudo ./run.sh server install`（或 `./run.sh center install`）
  - `sudo ./run.sh server configure`（或 `./run.sh center configure`）
  - `sudo ./run.sh server start`（或 `./run.sh center start`）
  - 打开：`http://CENTER_HOST:端口/` 和 `http://CENTER_HOST:端口/admin`

- 部署客户端（探针）：
  - `chmod +x run.sh tanzhen-probe`
  - `sudo ./run.sh client install`（或 `./run.sh probe install`）
  - `sudo ./run.sh client configure`（或 `./run.sh probe configure`）
  - `sudo ./run.sh client start`（或 `./run.sh probe start`）

> `client configure` 默认只需要填写“中心地址 + 中心密码”；密码留空会保持不变（避免在终端回显已有密码）。

脚本支持：`install / configure / start / stop / restart / status / show-config / uninstall`（优先 systemd，没 systemd 就用 nohup+pidfile）。

语言切换：
- 默认中文：直接运行
- 英文：`TZ_LANG=en ./run.sh`

菜单模式（更适合 SSH 操作）：
- 客户端菜单：`bash run.sh c`（或 `./run.sh client` / `./run.sh probe`）
- 服务端菜单：`bash run.sh s`（或 `./run.sh server` / `./run.sh center`）

可选美化 UI（dialog）：
- 安装 `dialog` 后，脚本会自动使用 TUI 菜单（更好看）。
- 强制使用/禁用：`TZ_UI=dialog ./run.sh` / `TZ_UI=text ./run.sh`

更多/修复：
- 菜单里的 `10. 更多/修复` 提供：自检（端口/连通性/密码）、安装依赖、防火墙放行/持久化提示、导入/导出备份、回滚二进制、校验配置、实时跟随日志等。

卸载可控：
- 默认卸载只删除二进制+配置；会询问是否删除数据/日志/备份。
- 强制清理（谨慎）：`TZ_UNINSTALL_PURGE=1 ./run.sh server uninstall`

### 1) 运行中心端

1. 复制配置：
   - `configs/center.example.json` → `center.json`
2. 修改 `center.json`：
   - `ingest_token`：上报密码（中心端用来校验/解密；客户端自助注册后会拿到“单机上报密码”）
   - `admin_token`：管理密码（管理 API / 控制面板登录）
   - `enroll_token`：接入密码（用于 `POST /api/enroll` 下发单机上报密码；可把它设为和 `admin_token` 一样以简化部署）
   - `enroll_max_fails` / `enroll_ban_hours`：同一 IP 连续输错 enroll_secret 会自动封禁（默认 5 次 / 8 小时）
   - `data_dir`：数据目录（会生成 `settings.json`、`servers.json`、以及历史数据）
   - `stealth_ingest_unauthorized`：未授权上报是否静默断开（默认建议 `true`）
3. 启动：
   - `go run ./cmd/center -config center.json`
4. 打开：
   - `http://CENTER_HOST:38088/`

### 2) 运行 Linux 客户端（Push）

1. 复制配置：
   - `configs/probe.example.json` → `probe.json`（兼容旧名：`agent.json`）
2. 修改 `probe.json`：
   - `central_url`：中心端地址（例如 `http://center.example.com:38088` 或 `center.example.com:38088`）
   - `enroll_token`：接入密码/中心密码（推荐：仅填它即可；客户端启动后会自动 enroll 并把中心端返回的“单机上报密码”写回 `probe.json`，同时移除 `enroll_token`）
   - （可选）`agent_id` / `name`：不填时会用 hostname 自动生成
   - （可选）`ingest_token`：传统方式（和中心端一致），不建议长期共享
   - `encrypt_enabled`：是否启用加密上报（可选）
3. 编译（示例：Linux amd64）：
   - `GOOS=linux GOARCH=amd64 go build -o tanzhen-probe ./cmd/agent`
4. 在服务器运行：
   - `./tanzhen-probe -config probe.json`

> 客户端当前仅支持 Linux（依赖 `/proc`）。

## 配置：到期时间与端口探活

中心端会在 `data_dir/servers.json` 保存每台服务器的配置（可手动编辑，也可用管理 API 更新）。

### 到期时间

在对应服务器对象里设置：
- `expires_text`: `"长期"`（默认）
- 或 `expires_date`: `"YYYY-MM-DD"`（UTC）

面板会显示：
- `到期日期: YYYY/MM/DD`
- `距离续费: N天`（过期则显示 `已过期: N天`）

### 端口探活（可选）

在 `servers.json` 为某台服务器设置：
- `port_probe_enabled: true`
- `port_probe_host: "127.0.0.1"`（一般探本机端口）
- `ports: [22, 80, 443]`

客户端下次上报会携带端口状态，面板底部显示绿/红点。

## 管理 API（需要 `X-Admin-Token` 管理密码）

更新全局设置（采集间隔/保留天数/面板轮询）：
- `POST /api/admin/settings`

更新某台服务器配置（包含 `expires_date`、端口列表、采集间隔等）：
- `POST /api/admin/server`

查看/解除 enroll 黑名单：
- `GET /api/admin/bans`
- `POST /api/admin/bans`（body：`{"ip":"x.x.x.x"}`）

查看/解除 Admin Token 尝试黑名单：
- `GET /api/admin/admin_bans`
- `POST /api/admin/admin_bans`（body：`{"ip":"x.x.x.x"}`）

## 分组/标签（可选）

在控制面板（`/admin`）里开启 `启用分组/标签` 后：
- 每台服务器可填写 `tags`（逗号分隔）
- 监控主页会按“第一个标签”分组，并支持折叠（折叠状态会被浏览器记住）

示例（curl）：
```bash
curl -X POST "http://CENTER_HOST:38088/api/admin/server" \
  -H "X-Admin-Token: change-me-too" \
  -H "Content-Type: application/json" \
  -d '{"id":"la-01","name":"LA-01","expires_date":"2026-12-31","port_probe_enabled":true,"port_probe_host":"127.0.0.1","ports":[22,80,443],"tcp_conn_enabled":true}'
```
