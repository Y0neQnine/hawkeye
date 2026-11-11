---
name: quake-mapping
description: 用 Quake（360 测绘）收集域名、IP、端口、服务与证书指纹时调用。
---

# Quake 资产测绘

在**已授权**前提下，用 Quake 补充目标暴露面：域名/子域、IP、端口、服务组件、证书与站点标题。

## 前置配置

Quake API Key 在 **系统配置 → 测绘 API** 中填写。Key 由服务端保存并代发请求，
**不要**在命令、文件或对话里粘贴 Key。

## 调用方式

```
recon_search(provider="quake", query='domain: "example.com"', page=1, size=20)
```

- `page` 从 1 开始；服务端按 `start=(page-1)*size` 分页；`size` 最大 100。
- 返回 `{provider, query, page, size, total, items, note}`；`items` 为 Quake 原始 `data`。

## 语法速查

| 目的 | 查询 |
|---|---|
| 根域名及子域 | `domain: "example.com"` |
| 精确主机 | `domain: "www.example.com"` |
| 指定 IP | `ip: "1.2.3.4"` |
| 网段 | `ip: "1.2.3.0/24"` |
| 证书 | `cert: "example.com"` |
| 站点标题 | `title: "管理后台"` |
| 服务/组件 | `service: "http"` / `app: "nginx"` |
| 端口 | `port: 443` |
| 组合 | `domain: "example.com" AND port: 443` |

逻辑运算用 `AND` / `OR` / `NOT`；字符串值一般加双引号。

## 工作流

1. `domain: "example.com"` 拉取子域与对应 IP、端口、服务。
2. 用 `cert: "example.com"` 找证书关联资产（常能发现未解析到主域的站点）。
3. 对关键 IP 用 `ip: "x.x.x.x"` 查看同 IP 服务，标注是否为共享托管。
4. 用 `service` / `app` / `title` 判断资产类型，再写回资产图：
   - 域名类 → `subdomain` / `root_domain`
   - IP、端口、协议 → `ip.open_ports` 或 `service`
   - HTTP 服务 → `service.url` + `page_title` + `technologies`
5. 仅写确认属于目标范围的资产；归属不明的内容先作为事实记录。

## 边界与错误处理

- **只做资产发现**，不进行漏洞验证、利用或破坏性操作。
- `Quake 未配置`：引导用户去 **系统配置 → 测绘 API** 填写 Key。
- 返回 code 非 0 时保留服务端 message（常见为配额、语法或权限问题），不要盲目重试。
- 结果为空可换 `cert:` / `title:` 等条件，或改用 FOFA/Hunter 交叉验证。
