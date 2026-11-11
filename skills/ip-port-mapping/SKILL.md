---
name: ip-port-mapping
description: 测绘目标 IP、ASN 归属、开放端口与 TLS 证书时调用，被动数据优先，仅在授权主机上做低速率端口探测。
---

# IP / 端口 / ASN 测绘

在**已授权**前提下，把域名解析到 IP，补充 ASN 归属、反向解析、开放端口与服务、TLS 证书信息，形成 IP 维度的暴露面。

## 边界

- **只处理任务范围或公司范围中的 IP 与 CIDR**；范围外的 IP 不做主动探测。
- 被动来源优先（测绘引擎、证书、PTR）；主动端口探测仅对明确授权的单个 IP 或 `/24` 及更小网段，且必须低速率。
- 禁止互联网大段扫描、UDP 洪泛、漏洞 NSE 脚本、口令爆破、规避检测；`nmap` 仅用 `-sT`/`-sV` 基础探测。
- 单 IP 常用端口探测；单次任务主动探测目标数 ≤ 256 个 IP。

## 步骤

### 1. 域名到 IP

```bash
while read -r h; do
  ips=$(dig +short A "$h"; dig +short AAAA "$h")
  [ -n "$ips" ] && printf '%s\t%s\n' "$h" "$(echo "$ips" | paste -sd,)"
done < /tmp/hosts.txt | tee /tmp/host-ip.tsv
```

CDN 场景：同一域名解析到多个共享 CDN IP，不能据此认定源站；记录为 CDN 边缘即可。

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

优先已有工具：

```bash
# 常用端口，TCP connect，保守速率
nmap -sT -Pn -T3 --max-retries 2 --top-ports 100 -oG /tmp/nmap.gnmap 1.2.3.4
# 或
naabu -host 1.2.3.4 -top-ports 100 -rate 50 -silent
```

无工具时用 bash 连接测试（仅小端口集，串行）：

```bash
for p in 21 22 25 53 80 110 143 443 445 3306 3389 5432 6379 8080 8443 9000; do
  timeout 2 bash -c "echo >/dev/tcp/1.2.3.4/$p" 2>/dev/null && echo "1.2.3.4 $p open"
done
```

- 只对授权 IP 执行；出现大量超时立即停止。
- 不做全端口 1-65535 扫描；确需扩展端口集时逐个网段评估。

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
  {"type":"service","service_name":"ssh","port":22,"ip":"1.2.3.4"},
  {"type":"service","url":"https://www.example.com/","status_code":200,"technologies":["Nginx"]}
]}
```

- HTTP/HTTPS 服务用 `service` 的 `url` 形式（字段同 `web-fingerprint`）；其余协议用 `service_name + port + ip`。
- `bound_domains` 只填已验证解析到该 IP 的域名。
- 每个 IP 的探测来源（被动/主动）写入 `record_fact`，便于后续复现与判断置信度。

## 错误处理

| 现象 | 处理 |
|---|---|
| nmap/naabu 未安装 | 用 `/dev/tcp` 小端口集替代，不安装新工具 |
| 大量 filtered/timeout | 目标可能限流或防火墙，停止主动探测，改用被动来源 |
| 测绘引擎未配置 | 提示在 系统配置 → 测绘 API 配置，不猜 Key |
| 同一 IP 绑定大量域名 | 记录为共享/虚拟主机，归属判断降级，不当作企业资产直接写回 |
