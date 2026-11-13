---
name: web-fingerprint
description: 采集站点 HTTP 指纹与系统识别（产品/CMS/OA/中间件/WAF）、页面信息与前端可见端点时调用，输出状态码、标题、技术栈、favicon、robots/sitemap 与页面内接口。
---

# Web 指纹、系统识别与端点采集

在**已授权**前提下，对任务范围内的站点做**只读** HTTP 探测：状态码、响应头、标题、技术栈、favicon、robots/sitemap，以及页面里暴露的端点。

## 自动指纹（优先利用，不要重复劳动）

Hawkeye 对**每个登记为 service 的 HTTP 站点**（insert_assets 的 `url`、任务初始地址、资产导入/API 导入）都会自动做一次只读探测，并把结果写回该资产：

- `status_code` / `content_length` / `page_title` / `favicon_mmh3`
- `technologies`：识别出的**产品/系统**与技术栈，如 `["泛微 e-cology 9.0", "OA", "Java", "Nginx"]`、`["WordPress 6.4.2", "CMS", "PHP"]`

因此：**站点落库后先用 `list_assets` 按 URL 回读 technologies**，不要再用 curl 把首页/响应头/标题重新抓一遍。自动探测识别不出（登录后页面、纯 JS 渲染、非标准产品）时，才按下面的步骤人工补采，并把新发现追加进 `technologies`。

自动识别覆盖的类别：OA/ERP/协同办公（泛微、致远、用友、金蝶、通达、蓝凌、万户、帆软等）、CMS/博客/论坛（WordPress、Drupal、DedeCMS、PHPCMS、Discuz、PbootCMS 等）、运维/中间件管理面（宝塔、Jenkins、GitLab、Grafana、Nacos、MinIO、phpMyAdmin 等）、WAF/CDN/VPN/安全设备（Cloudflare、阿里云/腾讯云 CDN、创宇盾、安全狗、F5、深信服、Fortinet 等）、Web 服务器/语言/框架/前端框架。

## 边界

- 只发 GET/HEAD 类只读请求；**不做**目录爆破、参数 fuzz、登录尝试、越权探测或漏洞验证。
- 串行或低并发（≤ 5），单请求超时 10 秒；对同一站点连续请求间隔 ≥ 200ms。
- 只跟随同范围重定向；跳到范围外域名时停止并记录，不继续抓取。
- 遇到 WAF/验证码/401 属正常采集结果，记录状态与页面标题即可，不尝试绕过。

## 输入

任务范围内的子域 / URL 清单。优先用已解析且存活的 HTTP 服务，逐个采集。

## 步骤

### 1. 批量存活与基础指纹

有 httpx 时：

```bash
httpx -l /tmp/hosts.txt -silent -status-code -title -tech-detect -web-server -follow-redirects -timeout 10
```

无 httpx 时用 curl 串行：

```bash
while read -r h; do
  out=$(curl -skL -m 10 -o /tmp/body.html -w '%{http_code}\t%{size_download}\t%{content_type}\t%{url_effective}' "https://$h/" 2>/dev/null)
  code=${out%%	*}
  if [ "$code" = "000" ]; then
    out=$(curl -skL -m 10 -o /tmp/body.html -w '%{http_code}\t%{size_download}\t%{content_type}\t%{url_effective}' "http://$h/" 2>/dev/null)
  fi
  printf '%s\t%s\n' "$h" "$out"
done < /tmp/hosts.txt
```

`-k` 仅为采集过期/自签证书站点信息，不代表忽略证书风险判断；证书主体信息另见 `ip-port-mapping`。

### 2. 响应头与技术栈

```bash
curl -skI -m 10 "https://host/" | sed -n '1,40p'
```

按实际头字段推断，不要凭页面风格猜：

| 证据 | 技术栈 |
|---|---|
| `Server: nginx` / `openresty` | Nginx / OpenResty |
| `X-Powered-By: PHP/8.x` | PHP |
| `Set-Cookie: JSESSIONID=` | Java 系 |
| `Set-Cookie: PHPSESSID=` | PHP |
| `X-AspNet-Version` / `ASP.NET_SessionId` | ASP.NET |
| `cf-ray` / `server: cloudflare` | Cloudflare（CDN，不代表源站） |
| 页面含 `__NEXT_DATA__` | Next.js |
| 页面含 `window.__NUXT__` | Nuxt |
| 页面含 `ng-version=` | Angular |
| `wp-content/` / `wp-json` | WordPress |

拿不准的写「疑似」，只登记有证据的项。

### 2.1 系统/产品识别补充（自动识别未覆盖时）

按「证据 → 产品」登记，**产品名用资产库里的规范写法**，不要写泛化描述：

| 证据 | 登记为 |
|---|---|
| `Server: Apache-Coyote/1.1`、页面含 Apache Tomcat | `Apache Tomcat` |
| Cookie `ecology_JSessionid`、页面含 weaver / 泛微 | `泛微 e-cology` |
| 页面含 seeyon / 致远 | `致远 A8` |
| 页面含 nccloud / yonyou / 用友 | `用友 NC` |
| 页面含 k3cloud / kingdee / 金蝶 | `金蝶 EAS/K3 Cloud` |
| 页面含 /general/login/index.php + 通达 | `通达 OA` |
| 页面含 landray / /ekp/ | `蓝凌 EKP` |
| 页面含 /WebReport/ / finereport | `帆软 FineReport` |
| 页面含 ruoyi / 若依 | `若依 RuoYi` |
| Cookie `rememberMe`、Whitelabel Error Page | `Apache Shiro` / `Spring Boot` |
| Cookie `laravel_session` / `csrftoken` | `Laravel` / `Django` |
| 页面含 `__NEXT_DATA__` / `__NUXT__` / `ng-version=` | `Next.js` / `Nuxt` / `Angular` |
| 响应头 `cf-ray`、`ali-swift-global-savetime`、`X-Nws-Log-Uuid` | `Cloudflare` / `阿里云 CDN` / `腾讯云 CDN` |
| Cookie `yunsuo_session_verify`、`safedog-flow-item`、`wzws_sessionid` | `创宇盾` / `安全狗` / `360 网站卫士` |
| 响应头 `X-Jenkins`、Cookie `_gitlab_session`、页面含 Nacos/Grafana | `Jenkins` / `GitLab` / `Nacos` / `Grafana` |
| `<meta name="generator" content="...">` | 按 generator 值登记（常直接给出 CMS 与版本） |

- 产品识别与「语言/框架/中间件/前端」分开登记：一个站点可以同时是 `["致远 A8","OA","Java","Nginx"]`。
- 版本能从 `Server`、`X-Powered-By`、`generator` 或产品版本号直接读到时带上（如 `Nginx 1.24.0`、`WordPress 6.4.2`）；读不到就只写产品名。
- WAF/CDN 只说明边缘设施，**不代表源站**；源站系统另按页面/头/Cookie 判断。

### 3. 标题与页面元信息

```bash
grep -oE '<title[^>]*>[^<]{0,200}' /tmp/body.html | head -1
grep -oE '<meta[^>]+name="(description|keywords|generator)"[^>]*>' /tmp/body.html | head -5
```

`generator` 常直接给出 CMS 版本，优先登记。

### 4. robots / sitemap / favicon

```bash
curl -sk -m 10 "https://host/robots.txt" -o /tmp/robots.txt -w '%{http_code}\n'
curl -sk -m 10 "https://host/sitemap.xml" -o /tmp/sitemap.xml -w '%{http_code}\n'
curl -sk -m 10 "https://host/favicon.ico" -o /tmp/favicon.ico -w '%{http_code} %{size_download}\n'
```

- robots/sitemap 中出现的路径是**站点自述**，只登记为端点候选，不主动请求敏感路径。
- favicon 的 MMH3 仅在环境已装 `mmh3` 时计算：

```bash
python3 -c 'import base64,mmh3;print(mmh3.hash(base64.encodebytes(open("/tmp/favicon.ico","rb").read())))'
```

失败（未安装）就跳过，不要安装依赖。

### 5. 页面内端点提取（静态）

只从已抓取的 HTML/JS 文本中提取，不构造请求：

```bash
grep -oE '(href|src|action)="[^"]{1,300}"' /tmp/body.html | head -100
grep -oE '"/[a-zA-Z0-9_./-]*(api|rest|graphql)[a-zA-Z0-9_./-]*"' /tmp/body.html | sort -u | head -50
curl -sk -m 10 "https://host/static/app.js" -o /tmp/app.js && \
  grep -oE '"/[a-zA-Z0-9_./-]*(api|rest|graphql)[a-zA-Z0-9_./-]*"' /tmp/app.js | sort -u | head -50
```

- 只取同源路径；第三方统计、CDN、字体域名不入端点。
- 动态拼接或加密的端点无法静态还原，标注「待触发」，不臆造。
- 需要更完整的登录后接口测绘时，交由 `api-recon` Skill，不在本流程做登录与攻击。

## 写回

```json
{"assets":[
  {"type":"service","url":"https://www.example.com/","status_code":200,"content_length":15230,
   "page_title":"Example 官网","technologies":["Nginx","Next.js"],"favicon_mmh3":"123456789"},
  {"type":"endpoint","url":"https://www.example.com/api/news","method":"GET",
   "params":[{"location":"query","name":"page","type":"integer"}]}
]}
```

- `url` 必须带协议与端口；跳转后的最终 URL 用 `url_effective`，原始入口写事实。
- 只填 `url` 也可以：`status_code`/`content_length`/`page_title`/`technologies`/`favicon_mmh3` 会被自动探测补全。人工写回时只补自动识别**没有覆盖**的产品（`technologies` 是追加合并，不会覆盖已有值）。
- `params` 只登记页面/JS 中**实际出现**的字段，位置不确定时写 `query` 并在事实里说明推断依据。
- 同一 URL 重复采集走 `insert_assets` 自动合并，不要手工覆盖已有技术栈。

## 错误处理

| 现象 | 处理 |
|---|---|
| TLS 握手失败 | 换 http:// 再试一次；仍失败则记录不可达，不反复重试 |
| 403 / 429 | 记录状态与 Server 头，停止该站点后续请求 |
| 页面为空或全 JS 渲染 | 记录标题与技术栈，端点标注「待 api-recon」 |
| 大响应（>5MB） | 只保留头部与标题，不整包入库 |
