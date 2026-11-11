---
name: hunter-mapping
description: 用 Hunter（奇安信鹰图）收集 Web/非 Web 资产、域名、IP 与站点指纹时调用。
---

# Hunter 资产测绘

在**已授权**前提下，用 Hunter 补充目标暴露面：Web 站点、非 Web 服务、域名、IP、端口与标题指纹。

## 前置配置

Hunter API Key 在 **系统配置 → 测绘 API** 中填写。Key 由服务端保存并代发请求，
**不要**在命令、文件或对话里粘贴 Key。

## 调用方式

```
recon_search(provider="hunter", query='domain.suffix="example.com"', page=1, size=20)
```

- `page` 从 1 开始；`size` 最大 100。
- 固定 `is_web=3`，即同时检索 Web 与非 Web 资产。
- 返回 `{provider, query, page, size, total, items, note}`；`items` 为 Hunter 原始 `data.arr`。

## 语法速查

| 目的 | 查询 |
|---|---|
| 域名后缀（含子域） | `domain.suffix="example.com"` |
| 精确域名 | `domain="www.example.com"` |
| 指定 IP | `ip="1.2.3.4"` |
| 网段 | `ip="1.2.3.0/24"` |
| IP + 端口 | `ip.port="1.2.3.4:443"` |
| 证书 | `cert="example.com"` |
| 站点标题 | `web.title="管理后台"` |
| 响应特征 | `web.body="xxx"` |
| 状态码 | `status_code=200` |
| 仅域名资产 | `is_domain=true` |

空格表示 AND，`||` 表示 OR；字段支持 `=` / `!=` / `*=` 等比较方式。

## 工作流

1. `domain.suffix="example.com"` 拉取子域与站点，`size` 控制在 20-50 减少配额消耗。
2. 需要非 Web 资产时按 `ip=` / `ip.port=` / `protocol=` 查询。
3. 用 `web.title` / `status_code` / `cert` 判断资产归属与类型。
4. 结果写回资产图：
   - 域名 → `subdomain` / `root_domain`
   - 站点 → `service`（`url`、`page_title`、`status_code`、`technologies`）
   - IP/端口 → `ip.open_ports` 或非 HTTP `service`
5. 共享 IP、CDN、云托管产生的记录要标注不确定性，不要直接归为自有资产。

## 边界与错误处理

- **只做资产发现**，不做漏洞扫描、利用或口令猜测。
- `Hunter 未配置`：引导用户去 **系统配置 → 测绘 API** 填写 Key。
- code 非 200 时保留返回 message（配额/语法/权限），换条件或换引擎，不要连续重试。
- 结果为空属正常；最多调整两次语法，仍为空则换 FOFA/Quake 交叉补充。
