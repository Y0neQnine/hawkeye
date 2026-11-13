---
name: ip-port-mapping
description: 测绘目标 IP、ASN 归属、开放端口（常见/敏感/全端口三档）与 TLS 证书时调用，被动数据优先，仅在授权主机上做低速率端口探测。
---

# IP / 端口 / ASN 测绘

在**已授权**前提下，把域名解析到 IP，补充 ASN 归属、反向解析、开放端口与服务、TLS 证书信息，形成 IP 维度的暴露面。

## 边界

- **只处理任务范围或公司范围中的 IP 与 CIDR**；范围外的 IP 不做主动探测。
- 被动来源优先（测绘引擎、证书、PTR）；主动端口探测仅对明确授权的单个 IP 或 `/24` 及更小网段，且必须低速率。
- 禁止互联网大段扫描、UDP 洪泛、漏洞 NSE 脚本、口令爆破、规避检测；`nmap` 仅用 `-sT`/`-sV` 基础探测。
- 主动端口探测分三档：**常见端口**（Top 100/1000）、**危险/敏感端口**（管理面/数据库/远程类固定清单）、**全端口**（1–65535，需单独授权）。
- 单次任务主动探测目标数 ≤ 256 个 IP；全端口只对单个 IP 执行。
- 主动探测按主机串行、速率 ≤ 50 pps；连续出现 filtered/timeout 立即停止该网段，避免触发防火墙/WAF 封禁。

## 步骤

### 1. 域名到 IP

```bash
while read -r h; do
  ips=$(dig +short A "$h"; dig +short AAAA "$h")
  [ -n "$ips" ] && printf '%s\t%s\n' "$h" "$(echo "$ips" | paste -sd,)"
done < /tmp/hosts.txt | tee /tmp/host-ip.tsv
```

CDN 场景：同一域名解析到多个共享 CDN IP，不能据此认定源站；记录为 CDN 边缘即可，但仍要把解析到该 IP 的目标域名写进 `bound_domains`——IP 依靠它继承域名所属企业，共享不会因此把其他域名算进来。识别为 CDN/WAF 的 IP 同时打标签（`technologies`），如 `["CDN","WAF","saaswaf"]`。

**CDN/WAF 判定依据**（满足其一即可打标，命中越多越可信）：

- CNAME 指向厂商加速/防护域：`*.saaswaf.com`、`*.dbappwaf.cn`、`*.cdn.dnsv1.com`、`*.alicdn.com`、`*.cloudflare.net`、`*.akamaiedge.net`、`*.wscdns.com`、`*.cloudfront.net` 等。
- TLS 证书签发者/主体为 CDN/WAF 厂商，或单证书 SAN 覆盖大量无关域名。
- 同一 IP 承载大量不相关域名的站点（共享边缘特征），而非目标独占主机。
- 测绘引擎显示服务组件为 CloudFront / Cloudflare / CloudWAF / 网宿等。

**标签格式**：角色 `CDN` / `WAF` + 厂商小写短名（`cloudflare`、`aliyun`、`tencent-cloud`、`saaswaf`、`wangsu`、`akamai`…），例如 `["CDN","WAF","saaswaf"]`。标签只描述网络角色，不参与企业归属，也不能代替 `bound_domains`。

**非目标 IP 的处理**：采集过程中与目标存在解析/引用关系的 IP（共享 CDN、云主机、第三方平台）照常登记为 `ip` 资产并打标签；`company_id` 先看 `bound_domains` 命中的域名范围，再看企业 IP/CIDR/ICP 范围，都没有时归入采集它的公司任务所属企业（该任务只关联一个企业时）。只有多企业任务或无企业上下文时才保持未归属。不要为了归属而扩大 `company_scope`；与任务无关的扫描噪声 IP 不登记。

### 2. ASN 与归属（Team Cymru DNS，无需 Key）

```bash
dig +short TXT "8.8.8.8.origin.asn.cymru.com"
# "15169 | 8.8.8.0/24 | US | arin | 1992-12-01"
dig +short TXT "AS15169.asn.cymru.com"
# "15169 | US | arin | 2000-03-30 | GOOGLE - Google LLC, US"
```

无 dig 时可用 whois：

```bash
whois -h whois.cymru.com " -v 8.8.8.8"
```

ASN 归属只说明**网络运营方**，不等于资产归属；云厂商 ASN（AWS/阿里云/腾讯云）下的 IP 需结合证书、站点内容判断。

### 3. 反向解析

```bash
dig +short -x 1.2.3.4
```

PTR 常暴露 `ecs-...`、`mail.`、`vpn.` 等用途线索，登记为事实。

### 4. 被动端口与服务

```
recon_search(provider="quake",  query='ip: "1.2.3.4"', size=100)
recon_search(provider="fofa",   query='ip="1.2.3.4"', size=100)
recon_search(provider="shodan", query='ip:1.2.3.4')
recon_search(provider="hunter", query='ip="1.2.3.4"', size=100)
```

返回的 `port` / `service` / `banner` / `title` 直接用于端口与服务登记；Shodan 的 banner 可提取协议与版本，但不要据此做漏洞判断。

### 5. 授权范围内的主动端口探测（可选）

先被动，再主动；主动探测按下面的**三档端口集**选一档，不要自行缩减成随手几条端口。

| 档位 | 端口集 | 适用场景 | 前置条件 |
|---|---|---|---|
| 常见端口（默认） | Top 100，必要时 Top 1000 | 常规暴露面确认 | 授权 IP |
| 危险/敏感端口 | 固定清单（见下） | 排查管理面、数据库、远程类服务 | 授权 IP |
| 全端口 | 1–65535 | 需要完整端口暴露面 | 单个 IP + 明确书面授权 |

**5.1 常见端口**

```bash
nmap -sT -Pn -T3 --max-retries 2 --top-ports 100 -oG /tmp/nmap-top100.gnmap 1.2.3.4
# 结果不够再扩到 Top 1000
nmap -sT -Pn -T3 --max-retries 2 --top-ports 1000 -oG /tmp/nmap-top1000.gnmap 1.2.3.4
# 或
naabu -host 1.2.3.4 -top-ports 100 -rate 50 -silent
```

**5.2 危险/敏感端口**

覆盖远程管理、数据库、缓存、消息、容器/编排、文件共享等高价值端口：

```bash
nmap -sT -Pn -T3 --max-retries 2 \
  -p 21,22,23,25,53,111,135,139,161,389,445,512,513,514,873,1025,1433,1521,2049,2181,2375,2376,3306,3389,5432,5672,5900,5984,6379,7001,7002,8080,8081,8443,8888,9000,9090,9200,9300,11211,27017 \
  -oG /tmp/nmap-danger.gnmap 1.2.3.4
# 或
naabu -host 1.2.3.4 -p 21,22,23,25,53,111,135,139,161,389,445,1433,1521,2049,2181,2375,2376,3306,3389,5432,5672,5900,6379,7001,8080,8443,8888,9000,9090,9200,11211,27017 -rate 50 -silent
```

**5.3 全端口（1–65535，仅限明确授权）**

```bash
nmap -sT -Pn -T3 --max-retries 1 --min-rate 50 --max-rate 50 -p- -oG /tmp/nmap-full.gnmap 1.2.3.4
# 或
naabu -host 1.2.3.4 -p - -rate 50 -silent
```

- 全端口只对**单个 IP**执行；`/24` 及更大网段不做全端口，先用常见/危险端口筛出存活主机再逐个评估。
- 全端口耗时更长，必须保持单主机串行、速率 ≤ 50 pps，并盯住 filtered/timeout 比例。

**5.4 无工具时的兜底（仅常见端口）**

```bash
for p in 21 22 23 25 53 80 110 143 443 445 1433 1521 2049 2375 3306 3389 5432 6379 7001 8080 8443 9000 9090 9200 11211 27017; do
  timeout 2 bash -c "echo >/dev/tcp/1.2.3.4/$p" 2>/dev/null && echo "1.2.3.4 $p open"
done
```

- 只对授权 IP 执行；出现大量超时立即停止。
- 这一档只用于应急确认，不能代替上面的端口集，也不做全端口循环。

### 6. TLS 证书信息

```bash
echo | timeout 8 openssl s_client -connect 1.2.3.4:443 -servername www.example.com 2>/dev/null \
  | openssl x509 -noout -subject -issuer -dates -ext subjectAltName
```

- SAN 中出现的新域名，若在授权根域内，交给 `subdomain-recon` 登记；范围外只记录事实。
- 自签/过期/证书与主机名不匹配，作为服务特征登记，不做漏洞结论。
- 非 443 端口同样可用（如 8443），需 `-servername` 与目标主机名匹配。

## 写回

```json
{"assets":[
  {"type":"ip","ip":"1.2.3.4","bound_domains":["www.example.com","api.example.com"],
   "open_ports":[{"port":443,"service":"https"},{"port":22,"service":"ssh"}]},
  {"type":"ip","ip":"14.152.38.229","bound_domains":["www.lw.gov.cn"],
   "technologies":["CDN","WAF","saaswaf"]},
  {"type":"service","service_name":"ssh","port":22,"ip":"1.2.3.4"},
  {"type":"service","url":"https://www.example.com/","status_code":200,"technologies":["Nginx"]}
]}
```

- HTTP/HTTPS 服务用 `service` 的 `url` 形式（字段同 `web-fingerprint`）；其余协议用 `service_name + port + ip`。
- `bound_domains` 只填已验证解析到该 IP 的域名。企业范围通常只写域名，系统靠 `bound_domains` 把 IP 归入企业；为空时若采集该 IP 的公司任务只关联一个企业，IP 仍归入该任务企业（域名/IP/CIDR/ICP 证据优先，多企业任务保持未归属）。
- `technologies` 在 IP 上表示网络角色/厂商标签（CDN、WAF、云厂商），在 service 上表示技术栈；两者都按追加合并写入，重复登记不会丢标签。
- 每个 IP 的探测来源（被动/主动）写入 `record_fact`，主动探测还要记录档位（`top100` / `top1000` / `danger` / `full`）、工具与速率，便于复现与判断置信度。

## 错误处理

| 现象 | 处理 |
|---|---|
| nmap/naabu 未安装 | 用 `/dev/tcp` 小端口集替代，不安装新工具 |
| 大量 filtered/timeout | 目标可能限流或防火墙，停止主动探测，改用被动来源 |
| 测绘引擎未配置 | 提示在 系统配置 → 测绘 API 配置，不猜 Key |
| 同一 IP 绑定大量域名 | 记录为共享/虚拟主机；仍写 `bound_domains`（归属只按目标域名继承），但不据此新增企业范围或把其他域名算进目标 |
| CNAME 有 CDN 特征但厂商不明 | 至少标注 `["CDN"]`，厂商标签可省略；不要猜具体厂商 |
