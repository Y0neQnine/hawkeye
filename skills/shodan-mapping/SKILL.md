---
name: shodan-mapping
description: 用 Shodan 收集境外及全球暴露的 IP、端口、服务与证书资产时调用。
---

# Shodan 资产测绘

在**已授权**前提下，用 Shodan 补充全球暴露面：IP、端口、服务 banner、TLS 证书与站点标题。

## 前置配置

Shodan API Key 在 **系统配置 → 测绘 API** 中填写。Key 由服务端保存并代发请求，
**不要**在命令、文件或对话里粘贴 Key。

## 调用方式

```
recon_search(provider="shodan", query='hostname:example.com', page=1)
```

- `page` 从 1 开始；Shodan 每页固定 100 条，`size` 参数不生效。
- 返回 `{provider, query, page, size, total, items, note}`；`items` 为 Shodan 原始 `matches`。

## 语法速查

| 目的 | 查询 |
|---|---|
| 主机名 | `hostname:example.com` |
| 域名 | `domain:example.com` |
| 指定 IP | `ip:1.2.3.4` |
| 网段 | `net:1.2.3.0/24` |
| 端口 | `port:443` |
| TLS 证书 | `ssl:"example.com"` |
| 站点标题 | `http.title:"管理后台"` |
| 组织 | `org:"Example Inc"` |
| 组合 | `hostname:example.com port:443` |

空格表示 AND；短语加双引号。

## 工作流

1. `hostname:example.com` 或 `domain:example.com` 获取主机与端口。
2. 用 `ssl:"example.com"` 找证书关联资产，注意 SAN 列表可扩展测绘范围。
3. 用 `http.title:` / `product:` / `org:` 辅助确认资产归属。
4. 结果写回资产图：
   - 域名 → `subdomain` / `root_domain`
   - IP、端口、协议 → `ip.open_ports` / `service`
   - HTTP 服务 → `service.url` + `page_title` + `technologies`
5. 海外云主机、共享 IP、CDN 节点需标注，避免误判为自有资产。

## 边界与错误处理

- **只做资产发现**，不进行漏洞验证、利用或暴力破解。
- `Shodan 未配置`：引导用户去 **系统配置 → 测绘 API** 填写 Key。
- 401/402 通常为 Key 无效或配额/会员权限不足，原样反馈，不要重复重试。
- 结果为空可换 `ssl:` / `net:` 条件，或改用 FOFA/Quake/Hunter 交叉补充。
