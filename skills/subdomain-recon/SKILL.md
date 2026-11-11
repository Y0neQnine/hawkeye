---
name: subdomain-recon
description: 收集目标根域名下的子域名时调用，覆盖证书透明度日志、被动 DNS、测绘引擎与授权范围内的低速率字典校验。
---

# 子域与证书日志收集

在**已授权**前提下，从证书透明度日志、被动 DNS 与测绘引擎收集目标根域下的子域名，并对解析结果做校验与去重。

## 边界

- 只收集任务范围（任务范围 / 公司范围）内的根域名，不扩大到未授权域名。
- 被动来源优先；主动字典枚举仅在**确认授权**、且已识别泛解析后谨慎使用（见第 4 步）。
- 不做 DNS 放大器、不并发轰炸权威 DNS；单次任务解析速率控制在每秒 10 次以内。
- 结果必须经 DNS 校验后再写资产，不把日志里的历史域名直接当存活资产。

## 输入

从当前任务范围或用户描述中取根域名（如 `example.com`），不要自行添加同主体其他根域。

## 步骤

### 1. 证书透明度日志（被动）

crt.sh 免费、无需 Key：

```bash
curl -s -m 60 "https://crt.sh/?q=%25.example.com&output=json" \
  | python3 -c 'import sys,json;d=json.load(sys.stdin);n=set()
for r in d:
  for x in (r.get("name_value") or "").split("\n"):
    x=x.strip().lower().lstrip("*.")
    if x.endswith("example.com") and "@" not in x: n.add(x)
print("\n".join(sorted(n)))' > /tmp/ct.txt
wc -l /tmp/ct.txt
```

certspotter（免费额度有限，失败不重试超过 1 次）：

```bash
curl -s -m 60 "https://api.certspotter.com/v1/issuances?domain=example.com&include_subdomains=true&expand=dns_names" \
  | python3 -c 'import sys,json;d=json.load(sys.stdin);n=set()
for r in d:
  for x in r.get("dns_names") or []:
    x=x.lower().lstrip("*.")
    if x.endswith("example.com") and "@" not in x: n.add(x)
print("\n".join(sorted(n)))' >> /tmp/ct.txt
```

两个接口都可能超时或限流：超时就缩小查询（如只查 `www.example.com`），不要循环重试。

### 2. 测绘引擎（被动）

使用内置工具 `recon_search`，一次查询即够，翻页按配额决定：

```
recon_search(provider="fofa",   query='domain="example.com"', size=100)
recon_search(provider="fofa",   query='cert="example.com"', size=100)
recon_search(provider="quake",  query='domain: "example.com"', size=100)
recon_search(provider="hunter", query='domain.suffix="example.com"', size=100)
recon_search(provider="shodan", query='hostname:example.com')
```

从返回的 `host` / `domain` / `hostname` 字段提取主机名；未配置的 provider 会返回明确提示，直接换下一个，不要猜 Key。

### 3. DNS 校验与泛解析识别

```bash
while read -r h; do
  printf '%s\t' "$h"
  dig +short A "$h" | paste -sd, -
done < /tmp/ct.txt > /tmp/resolved.tsv
```

泛解析检测（随机标签能解析 ⇒ 存在泛解析，后续爆破结果不可信）：

```bash
dig +short A "$(head -c 8 /dev/urandom | od -An -tx1 | tr -d ' \n').example.com"
```

- 返回记录 ⇒ 标记 `wildcard=true`，**跳过第 4 步**；仅保留有 CNAME 或与泛解析记录不同的主机。
- 无记录 ⇒ 可做小规模校验。

### 4. 授权范围内的低速率字典校验（可选）

优先用已安装工具，二选一：

```bash
subfinder -d example.com -silent            # 被动聚合，不产生权威 DNS 压力
# 或
dnsx -l /tmp/ct.txt -silent -a -resp        # 批量解析已有名单
```

无工具时用小型字典 + 串行解析（禁止大字典并发）：

```bash
for p in www mail oa vpn admin api test dev git; do
  h="$p.example.com"; r=$(dig +short A "$h")
  [ -n "$r" ] && printf '%s\t%s\n' "$h" "$(echo "$r" | paste -sd,)"
done
```

- 字典规模 ≤ 100 条；`wildcard=true` 时不做。
- 不做递归子域爆破（`a.b.c.example.com` 无限展开）。

### 5. 去重与过滤

- 小写、去尾部点、去 `*.` 前缀、去重复。
- 丢弃不在根域后缀内的主机与邮箱类 SAN。
- 保留解析证据（A/AAAA/CNAME），无解析的只记事实、不写资产。

## 写回

```json
{"assets":[
  {"type":"root_domain","domain":"example.com"},
  {"type":"subdomain","domain":"www.example.com","record_type":"A","record_value":["1.2.3.4"]},
  {"type":"subdomain","domain":"api.example.com","record_type":"CNAME","record_value":["api.cdn.example.net"]}
]}
```

批量一次提交（单次 ≤ 100 条），`record_value` 只填实际解析到的值，不臆造。

同时用 `record_fact` 记录来源与结论，例如：`CT 日志 + FOFA 共得 38 个子域，其中 21 个有 A 记录，存在泛解析`；未授权或超范围的候选域名只写事实，等待规划者决定是否扩大范围。

## 错误处理

| 现象 | 处理 |
|---|---|
| crt.sh 超时/502 | 缩小查询到单个主机或改用 certspotter，不重试超过 1 次 |
| 测绘引擎未配置 | 提示在 系统配置 → 测绘 API 配置，改用其他 provider |
| 泛解析为真 | 放弃爆破，只保留被动来源中解析结果与泛解析不同的主机 |
| 结果为空 | 记录事实，改用 `cert=` / `host=` 语法再试一次 |
