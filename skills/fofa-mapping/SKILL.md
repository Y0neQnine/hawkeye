---
name: fofa-mapping
description: 用 FOFA 测绘引擎收集域名、子域名、IP、端口与服务指纹时调用。
---

# FOFA 资产测绘

在**已授权**前提下，用 FOFA 补充目标的外部暴露面：子域名、IP、开放端口、站点标题、服务指纹。

## 前置配置

FOFA 邮箱与 API Key 在 **系统配置 → 测绘 API** 中填写（邮箱必填，二者缺一不可）。
Key 由服务端保存并代发请求，**不要**在命令、文件或对话里粘贴 Key。

## 调用方式

使用内置工具 `recon_search`：

```
recon_search(provider="fofa", query='domain="example.com"', page=1, size=20)
```

- `page` 从 1 开始；`size` 最大 100。
- `fields` 可选（仅 FOFA 生效），默认 `host,ip,port,domain,title,server`。
- 返回 `{provider, query, page, size, total, items, note}`；`items` 为 FOFA 原始结果。

## 语法速查

| 目的 | 查询 |
|---|---|
| 根域名及其子域 | `domain="example.com"` |
| 精确主机 | `host="www.example.com"` |
| 证书包含 | `cert="example.com"` |
| 站点标题 | `title="管理后台"` |
| 指定 IP | `ip="1.2.3.4"` |
| 指定端口 | `port="8443"` |
| 协议 | `protocol="https"` |
| 组合 | `domain="example.com" && port="443"` |

`&&` 表示与、`||` 表示或、`!=` 表示排除。查询词含空格/引号时按 FOFA 语法转义。

## 工作流

1. 先用根域名查询（`domain="example.com"`），拿到 `host/ip/port/title/server`。
2. 对返回的 IP 可二次查询 `ip="x.x.x.x"` 看同 IP 其他站点（注意共享 IP 误报）。
3. 分页拉取直到 `page * size >= total`，但避免对同一查询反复翻页消耗配额。
4. 将确认属于目标范围的资产整理后写回资产图：
   - 子域/根域 → `insert_assets` 的 `subdomain` / `root_domain`
   - IP 与端口 → `ip`（`open_ports`）或 `service`（HTTP 用 `url`）
   - 站点标题/技术栈 → `service` 的 `page_title` / `technologies`
5. 拿不准归属的条目先记录在探索图事实里，不要直接写入资产图。

## 边界与错误处理

- **只做资产发现**，不据此做漏洞验证、口令爆破或入侵。
- 遇到 `FOFA 未配置`：提示用户在 系统配置 → 测绘 API 填写邮箱与 Key，不要自行猜测。
- 遇到配额/权限错误：原样保留返回信息，换用 Quake/Hunter 等其它引擎交叉补充，不要重复重试同一查询。
- 结果为空是正常情况，可调整语法（如改用 `cert=` 或 `host=`）再试，最多两次。
