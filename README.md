# Lulynx Server Probe

一个偏轻量、偏折腾友好的多服务器监控项目。

它的思路很直接：Linux 节点负责采集指标，然后通过 HTTP 主动推送到中心端；中心端负责展示面板、保存历史、管理节点和下发一些控制配置。没有很重的依赖，比较适合自己部署在 VPS 上长期挂着，也适合拿来做一个顺手的自用探针。

## 它能做什么

- Linux 节点采集 `/proc` 指标，并通过 HTTP Push 上报到中心端
- 中心端提供监控主页和 `/admin` 管理面板
- 支持 CPU、内存、磁盘、负载、流量、TCP 连接数等常见指标
- 支持可选端口探测，卡片底部会直接显示绿点/红点
- 支持“静默拒绝”未授权上报，减少被随手扫出来的特征
- 支持可选自动注册（Enroll），也支持直接给单机签发专用密码
- 支持主动/被动两种控制模式，适合不同部署习惯

## 仓库里主要有什么

- `cmd/center/web`
  真正内嵌到 `tanzhen-center` 里的监控主页和管理面板，项目跑起来后用户看到的是这一套。
- `site/`
  一个独立的 React/Vite 站点工程，偏官网/展示页，不影响探针本体运行。
- `run.sh`
  一键安装管理脚本。正常部署时，大多数人直接用它就够了。

## 最快的用法

如果你只是想先把它跑起来，建议直接用脚本。

把 `run.sh` 和对应二进制放到同一个目录里，然后执行：

### 中心端

```bash
chmod +x run.sh tanzhen-center
sudo ./run.sh server install
sudo ./run.sh server configure
sudo ./run.sh server start
```

也可以用别名：

```bash
sudo ./run.sh center install
sudo ./run.sh center configure
sudo ./run.sh center start
```

启动后可以打开：

- `http://CENTER_HOST:端口/`
- `http://CENTER_HOST:端口/admin`

### 客户端

```bash
chmod +x run.sh tanzhen-probe
sudo ./run.sh client install
sudo ./run.sh client configure
sudo ./run.sh client start
```

也可以这样写：

```bash
sudo ./run.sh probe install
sudo ./run.sh probe configure
sudo ./run.sh probe start
```

`client configure` 默认只会让你填最核心的几项，比如中心地址和上报密码；不想改已有密码的话，留空就行。

## 脚本支持什么

脚本常用动作有这些：

- `install`
- `configure`
- `start`
- `stop`
- `restart`
- `status`
- `show-config`
- `uninstall`

优先走 `systemd`；如果系统里没有 `systemd`，就退回到 `nohup + pidfile`。

### 语言切换

- 默认中文：直接运行
- 英文：`TZ_LANG=en ./run.sh`

### 菜单模式

如果你更喜欢 SSH 里那种数字菜单，也可以这样进：

- 客户端菜单：`bash run.sh c`
- 服务端菜单：`bash run.sh s`

或者：

- `./run.sh client`
- `./run.sh probe`
- `./run.sh server`
- `./run.sh center`

### `dialog` 界面

装了 `dialog` 之后，脚本会自动切成更好看的 TUI 菜单。

- 强制用 `dialog`：`TZ_UI=dialog ./run.sh`
- 强制用纯文本：`TZ_UI=text ./run.sh`

### 更多/修复

菜单里的 `10. 更多/修复` 目前可以做这些事：

- 自检端口、连通性和密码
- 安装依赖
- 防火墙放行和持久化提示
- 导入/导出备份
- 回滚二进制
- 校验配置
- 实时跟日志

### 卸载策略

- 默认卸载只删二进制和配置，同时会问你要不要顺手删数据、日志、备份
- 如果你就是想一把清掉，可以用：

```bash
TZ_UNINSTALL_PURGE=1 ./run.sh server uninstall
```

## 如果你不想用脚本

也可以手动跑。

### 中心端

1. 把 `configs/center.example.json` 复制成 `center.json`
2. 按需要改配置
3. 运行：

```bash
go run ./cmd/center -config center.json
```

最重要的几个字段：

- `ingest_token`
  节点上报密码。中心端会用它校验请求。
- `admin_user` / `admin_password`
  管理面板登录账号和密码，对应 `/admin`
- `enroll_token`
  可选。用于自动注册节点，留空就是关闭
- `data_dir`
  数据目录，里面会放 `settings.json`、`servers.json` 和历史数据
- `stealth_ingest_unauthorized`
  是否对未授权上报走静默拒绝，建议开着

### 客户端

1. 把 `configs/probe.example.json` 复制成 `probe.json`
2. 按需要改配置
3. 运行：

```bash
./tanzhen-probe -config probe.json
```

常用字段：

- `central_url`
  中心端地址，比如 `http://center.example.com:38088`
- `ingest_token`
  上报密码。最简单的用法就是直接填这个
- `enroll_token`
  可选。只有你打算走自动注册时才需要
- `agent_id` / `name`
  不填也行，默认会用 hostname 自动生成

客户端目前只支持 Linux，因为采集逻辑依赖 `/proc`。

## 一些常见配置

### 到期时间

中心端会把每台机器的配置存到 `data_dir/servers.json`。

如果你想在卡片上显示到期信息，可以给节点写：

- `expires_text: "长期"`
- `expires_date: "YYYY-MM-DD"`

面板会自动显示距离续费还有几天；如果已经过期，也会直接标出来。

### 端口探测

同样是在 `servers.json` 里给对应节点加上：

- `port_probe_enabled: true`
- `port_probe_host: "127.0.0.1"`
- `ports: [22, 80, 443]`

客户端下次上报时就会顺手把端口状态带回来。

### 分组/标签

在 `/admin` 里打开“启用分组/标签”之后：

- 每台服务器可以填 `tags`
- 主页会按第一个标签分组
- 折叠状态会记在浏览器本地

## 管理 API

平时直接用 Web 管理面板就行：

- `/admin`

登录用的是 `admin_user` / `admin_password`。

如果你想自己写脚本，也可以继续用 Header：

- `X-Admin-Token: <admin_password>`

目前比较常用的接口有：

- `POST /api/admin/settings`
  更新全局设置，比如采集间隔、历史保留天数、面板轮询间隔
- `POST /api/admin/server`
  更新某台服务器的配置，比如到期时间、端口、采集间隔等
- `POST /api/admin/issue_agent_token`
  给某个节点预注册并签发单机上报密码
- `GET /api/admin/bans`
- `POST /api/admin/bans`
  查看或解除 enroll 黑名单
- `GET /api/admin/admin_bans`
- `POST /api/admin/admin_bans`
  查看或解除管理密码尝试黑名单

给节点签发单机密码的例子：

```bash
curl -X POST "http://CENTER_HOST:38088/api/admin/issue_agent_token" \
  -H "X-Admin-Token: change-me-too" \
  -H "Content-Type: application/json" \
  -d '{"agent_id":"la-01","name":"LA-01"}'
```

返回结果里会有 `ingest_token`，客户端把它填进 `probe.json` 就能用，不一定非要和别的机器共用全局密码。

更新某台服务器配置的例子：

```bash
curl -X POST "http://CENTER_HOST:38088/api/admin/server" \
  -H "X-Admin-Token: change-me-too" \
  -H "Content-Type: application/json" \
  -d '{"id":"la-01","name":"LA-01","expires_date":"2026-12-31","port_probe_enabled":true,"port_probe_host":"127.0.0.1","ports":[22,80,443],"tcp_conn_enabled":true}'
```
