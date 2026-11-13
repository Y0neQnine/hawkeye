<div align="center">

# Hawkeye

基于 ARTEX 二次开发的资产信息收集 Web 系统：移除了攻击、漏洞扫描/验证/利用等能力，只保留攻击面信息收集与持续监控（Go 后端 + Next.js 前端）。

</div>

---

## 定位

本仓库是 **ARTEX 的二次开发版本**，删除了攻击与漏洞相关能力（漏洞扫描、验证、利用、PoC 等），只做信息收集这一件事，形态是一个 Web 系统。

Hawkeye 聚焦两件事：**攻击面资产收集**与**持续监控展示**。

- 通过 Agent 任务或外部数据源采集公司/项目名下的域名、子域、IP、端口、站点、服务、端点，以及移动 App、公众号/服务号、小程序；
- 按批次生成资产快照，自动计算与上一批次相比的**新增 / 消失 / 变更**，提供趋势与明细展示；
- 不提供漏洞扫描、漏洞验证、利用复测或 PoC 能力。

## 功能

- **任务采集**：为每个公司/项目下发采集目标，Agent 使用命令行工具与内置采集工具推进任务，过程可在会话中回放。
- **资产库**：统一管理公司、资产范围与各类资产，支持 DSL 检索与资产图。
- **监控中心**：快照批次列表、变更明细（new / gone / changed）、按公司/类型/关键字筛选、近 14 天趋势、手动立即快照。
- **自动监控（可开关）**：按「企业 + 根域名」配置监控目标与周期，开启总开关后自动发起增量采集，任务结束立即快照对比；可随时关闭总开关或单独停用某个目标。
- **资产同步**：从 [ScopeSentry](https://github.com/Autumn-27/ScopeSentry) 按项目或任务维度导入资产，导入后可自动触发监控快照。
- **对话**：与 Agent 多轮对话，直接查询或补采目标资产。
- **流量与工具执行**：查看录制流量与工具调用记录，便于核对采集结果。
- **系统管理**：LLM 配置、Agent 提示词、MCP、Skill、工具、拦截规则与审批、日志、系统配置。

## 监控机制

- 每次快照把当前全部资产的指纹写入 `monitor_snapshots`，并与上一成功批次比对，差异写入 `monitor_changes`；
- 首次快照作为基线，不产生变更记录；
- 定时快照默认每 **360 分钟**一次，可用环境变量 `HAWKEYE_MONITOR_INTERVAL_MIN` 调整；设为 `0` 或负数关闭定时快照，页面仍可手动触发；
- 多实例并发快照由数据库 advisory lock 串行化；
- 快照批次保留 **90 天**；
- 资产同步完成后会自动补一次快照（`source=sync`）；
- **自动监控**：在监控中心添加「企业 + 根域名」目标并设置周期（6 小时 ~ 30 天），打开总开关后，调度器到期发起一轮增量采集——先用 `list_assets` 读已有资产作基线，再通过证书日志、被动 DNS、测绘 API、子域校验发现新增子域/主机，站点登记时自动识别指纹；只登记新发现或属性缺失的资产。任务结束后立即快照并对比，新增资产按根域名归属自动挂到该企业。总开关默认**关闭**，关闭后不再自动采集，只保留手动快照与同步快照；每个目标也可单独停用；
- 变更记录保存前后稳定属性，并给出字段级差异（标题、技术栈、状态码、端口、解析记录等），不再只是一句“整行指纹变了”。

## 安装

> 依赖 **PostgreSQL**。采集任务需要配置 LLM（`ANTHROPIC_API_KEY` 或 `OPENAI_API_KEY`，也可在 UI 中配置）。

### 方式一：Docker Compose

```bash
git clone https://github.com/Y0neQnine/hawkeye.git hawkeye
cd hawkeye
cp .env.example .env          # 填 POSTGRES_PASSWORD，可选 LLM Key
docker compose up -d
# → http://localhost:8000
```

镜像来自 GitHub Container Registry：`ghcr.io/y0neqnine/hawkeye`（`latest` 或版本 tag，可用 `.env` 的 `ARTEX_TAG` 指定）。

首次进入 `/setup` 设置登录密码，用户名固定为“Hawkeye”。

### 方式二：一键安装脚本

```bash
./install.sh
```

脚本会检测/安装 Docker，并让你选择「全部 Docker」或「本地编译运行」；本地模式会生成 `config.json` 并编译内嵌前端的单二进制。

### 方式三：预编译二进制

到 Releases 下载对应平台的 zip，解压后得到可执行文件、`start.sh`（Windows 为 `start.bat`）、`skills/` 与 `config.example.json`：

```bash
cp config.example.json config.json   # 填好 database 连接
./start.sh                           # → http://localhost:8000
```

> 请通过 `start.sh` / `start.bat` 启动。它是守护脚本，程序退出后会按退出码决定是否重新拉起，崩溃可自动恢复。

### 方式四：从源码编译单二进制

```bash
# 1) 前端静态导出
cd web && npm ci && NEXT_EXPORT=1 npx next build && cd ..
# 2) 拷进内嵌目录
cp -r web/out server/webui/dist
# 3) 编译（-tags embedui 才内嵌前端）
CGO_ENABLED=0 go build -tags embedui -o hawkeye ./cmd/artex
./start.sh
```

### 方式五：构建跨平台发布包

```bash
./build.sh --release
# 产物在 dist/ 下
```

默认目标为 Linux amd64/arm64、macOS amd64/arm64 与 Windows amd64。可用 `ARTEX_TARGETS`（构建变量名沿用旧命名）自定义目标。

### 拉取镜像报 `unexpected EOF`

`docker compose pull` 报 `unexpected EOF` / `connection reset`，通常是 **Docker 守护进程没有走代理**（浏览器能访问 GitHub 不代表 Docker 能），或镜像层较大时网络中断。镜像包含 Playwright/Chromium、nmap 等工具，体积较大，重跑会复用已下载层。

先确认 Docker 能否直连 GHCR：

```bash
docker pull ghcr.io/y0neqnine/hawkeye:v0.1.1
```

- **Docker Desktop**：Settings → Resources → Proxies → Manual，HTTP 与 HTTPS 都填代理地址（如 `http://127.0.0.1:7890`），Apply & Restart。
- **Linux / Kali（Docker 在虚拟机，代理在宿主机）**：先取宿主机地址，再给 Docker 配置代理：

```bash
HOST_IP=$(ip route | awk '/default/ {print $3}')
sudo mkdir -p /etc/systemd/system/docker.service.d
sudo tee /etc/systemd/system/docker.service.d/http-proxy.conf >/dev/null <<EOF
[Service]
Environment="HTTP_PROXY=http://$HOST_IP:7890"
Environment="HTTPS_PROXY=http://$HOST_IP:7890"
Environment="NO_PROXY=localhost,127.0.0.1,::1"
EOF
sudo systemctl daemon-reload
sudo systemctl restart docker
```

宿主机代理需允许 LAN，并放行对应端口。配好后重新执行 `./install.sh` 或 `./update.sh`，脚本会自动重试 3 次。

仍拉取失败时，可改用**方式三：预编译二进制**（[Releases](https://github.com/Y0neQnine/hawkeye/releases/tag/v0.1.1) 下载 `hawkeye-0.1.1-<平台>.zip`），配合已有 PostgreSQL 运行，不依赖 GHCR。

## 配置

**数据库**（`config.json`，或用环境变量 `HAWKEYE_PG_DSN` 覆盖）：

```json
{
  "database": {
    "host": "127.0.0.1", "port": 5432,
    "user": "hawkeye", "password": "yourpass",
    "dbname": "hawkeye", "sslmode": "disable"
  }
}
```

**LLM**：`export ANTHROPIC_API_KEY=sk-...`（或 `OPENAI_API_KEY`），也可在 UI 的「LLM」页填写。
可选：`ARTEX_LLM_PROVIDER` / `ARTEX_LLM_MODEL` / `ARTEX_LLM_BASE_URL` / `ARTEX_LLM_PROXY`。

**并发**：每个任务的 work agent 数在「系统配置」中设置（默认 3）。

**常用参数**：`./start.sh -addr :8000 -proxy :8788`（`-addr` 页面与 API，`-proxy` 流量录制代理）。

**SEO**：构建前端时设置 `NEXT_PUBLIC_SITE_URL=https://your-domain`，用于 canonical、Open Graph、`robots.txt` 与 `sitemap.xml` 的绝对地址；未设置时回退 `http://localhost:8000`。

**测绘 API**：FOFA（邮箱 + Key）、Quake、Hunter、Shodan 的凭据在「系统配置 → 测绘 API」中填写，保存在数据库，仅回显是否已配置。内置 `recon_search` 工具由服务端代发查询，密钥不会进入 Agent 提示词或命令；配套 `fofa-mapping` / `quake-mapping` / `hunter-mapping` / `shodan-mapping` 四个 Skill，内置查询语法与资产写回流程。

**内置收集 Skill**：除测绘引擎外，还提供 `subdomain-recon`（证书日志与子域校验）、`web-fingerprint`（HTTP 指纹、系统/产品识别与页面端点）、`ip-port-mapping`（ASN/端口/TLS 证书）、`company-attribution`（ICP 备案与企业归属）、`app-wechat-recon`（移动 App、公众号/服务号、小程序的公开信息与归属）五个基础收集 Skill，均按授权范围与低速率执行，结果通过 `insert_assets` / `record_fact` 写回。

**站点自动指纹**：站点以 `service` 类型登记（Agent `insert_assets`、任务初始地址、资产导入/API）后，服务端会自动做一次只读 GET，回填状态码、响应长度、页面标题、favicon MMH3 与 `technologies`。`technologies` 不只记语言/框架，还会识别具体系统与产品（如 `泛微 e-cology 9.0`、`致远 A8`、`WordPress 6.4.2`、`用友 NC`、`宝塔面板`、`Cloudflare`），并附 `OA` / `CMS` / `WAF` / `ERP` 等类别标签；同一 URL 5 分钟内只探一次，跨主机重定向不跟随。

> 环境变量前缀 `ARTEX_*` 与 `artex_token` 仍沿用旧命名，属于兼容契约；可执行文件、Docker 服务/容器与脚本输出已统一为 `hawkeye`。数据库连接串变量为 `HAWKEYE_PG_DSN`，旧名 `ARTEX_PG_DSN` 不再读取。

## 技术架构

Go 单体后端（`go:embed` 内嵌 Next.js 静态前端）+ PostgreSQL，Agent 能力由 [`norma`](https://github.com/Autumn-27/norma) SDK 提供。

- **资产图（全局共享）**：跨任务同一份资产真值库，节点为 `root_domain / subdomain / ip / service / app / endpoint`，归属公司。父子关系与去重 key 由程序计算，Agent 只提交原始信息。
- **探索图（每任务独立）**：记录一次采集任务的推进过程，节点为 `goal / intent / fact / hint / digest`，靠 `spawns / derived_from / yields` 等边组成血缘链。
- **锚点**：`exploration_anchors` 把意图/事实锚定到具体资产，支持从采集方向看覆盖资产，也从资产反查采集过程。
- **引擎**：每任务一个 planner 循环 + N 个 worker；planner 派发意图，worker 领取意图、执行工具并把新资产/事实写回图。
- **监控快照**：`monitor_snapshots` + `monitor_changes`，定时或手动滚动计算资产差异，供监控中心展示。

## 更新与升级

**一键更新**：进入「系统配置 → 版本信息」，卡片会自动查询 GitHub 最新 Release；发现新版本时显示更新说明和「立即更新」按钮。点击后后端下载当前平台的 Release 包，校验 `SHA256SUMS`，暂存新程序并以退出码 `75` 交给 `start.sh` / `start.bat` 换装重启，页面会自动轮询并在新版本就绪后刷新。进行中的任务会中断。

- Docker 部署更新的是**容器内程序**；重建容器（`docker compose up`）会回到镜像版本，镜像级升级仍用 `./update.sh`。
- 容器无法直连 GitHub 时，在 `.env` 设置 `HAWKEYE_UPDATE_PROXY=http://<代理地址>`（也可用 `HTTPS_PROXY`）。
- GitHub API 触发限流时，可设置 `HAWKEYE_UPDATE_GITHUB_TOKEN`（只需 public repo 读取权限）。
- 更新依赖守护脚本：请通过 `start.sh`（Windows 为 `start.bat`）启动；直接运行二进制时，下载完成后需要手动重启。

仓库自带 `update.sh`，与 ARTEX 一致：先 `git pull` 同步代码，再按部署方式二选一升级——

```bash
git pull --ff-only
./update.sh          # 1) Docker：拉取 ghcr.io/y0neqnine/hawkeye 新镜像并重建
                     # 2) 本地：重建前端与内嵌单二进制
```

不便拉代码时也可手动升级：Docker 部署执行 `docker compose pull hawkeye && docker compose up -d --remove-orphans hawkeye`；二进制部署则停掉服务，用新的可执行文件替换旧的，再通过 `start.sh` / `start.bat` 重新启动。Postgres 数据卷、`./data`（jwt.key 等）与 `./skills` 都会保留。后端每次启动会幂等重跑 `schema.sql`，无需手动迁移；升级前仍建议备份数据库与 `./data`。

> 从 v0.1.0 升级：Docker 服务名由 `artex` 改为 `hawkeye`。上面的 `--remove-orphans` 会自动清理旧的 `artex` 容器（数据卷与 `./data` 保留），无需手动删除。

## 开发

```bash
./dev.sh    # 后端(:8000) + 流量代理(:8788) + 前端 next dev(:5173) → http://localhost:5173
```

- 后端：`go run ./cmd/artex`（不带 `-tags embedui` 则不内嵌前端）
- 前端：`cd web && npm run dev`（`/api` 反代到后端，带热更新）
- 测试：`go test ./...`
- Mock 预览（无后端）：`cd web && NEXT_PUBLIC_MOCK=1 npm run dev`

## 许可与合规

本项目采用 **GNU Affero General Public License v3.0（AGPL-3.0）** 授权，完整条款见根目录的 [LICENSE](LICENSE)。衍生作品须同样以 AGPL-3.0 开源；若修改后通过网络向用户提供服务，也须公开对应的完整源码。

本软件仅用于自有或已获明确书面授权资产的采集与监控。请勿对未授权的第三方系统发起采集、扫描或探测，并自行遵守所在国家或地区的法律法规。详细条款见登录页《使用须知》。
