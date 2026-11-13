---
name: company-attribution
description: 判断域名/IP/应用归属哪个企业或备案主体时调用，综合 ICP 备案、证书、WHOIS 与页面共同特征给出证据分级。
---

# 企业与备案归属

在**已授权**前提下，用公开信息判断一批域名、IP、应用归属于哪个企业（备案主体），把**有证据**的范围登记到公司名下。

## 边界

- 只使用公开可查信息：页面页脚、ICP 备案、证书、WHOIS、公开 DNS、页面统计 ID。
- **不做**社工、不注册第三方账号、不绕过登录/验证码、不批量抓取商业数据库。
- 共享 CDN、共享云 IP、同类技术栈**不构成**归属证据。
- 证据不足时只写事实，不创建公司、不扩大任务范围。

## 证据分级

| 级别 | 证据 | 处理 |
|---|---|---|
| 强 | ICP 备案主体与目标一致；EV/OV 证书 Organization 一致；官网明确声明 | 可直接加入公司范围 |
| 中 | 多站点共用同一统计 ID / 客服电话 / 邮箱；应用市场开发者一致 | 加入范围，reason 写明推断链 |
| 弱 | 同 IP/同 CDN、同 CMS、同模板、域名相似 | 只记录事实，不据此归属 |

## 步骤

### 1. ICP 备案（最强证据之一）

抓官网首页与「关于我们/联系我们」页脚：

```bash
curl -skL -m 10 "https://www.example.com/" -o /tmp/home.html
grep -oE '[京津沪渝冀豫云辽黑湘皖鲁新苏浙赣鄂桂甘晋蒙陕吉闽贵粤青藏川宁琼使领A-Za-z]{1,2}ICP备[0-9]{4,}[0-9A-Za-z-]*号?(-[0-9]+)?' /tmp/home.html | sort -u
grep -oE '(公安备案|公网安备)[^<]{0,40}' /tmp/home.html | head -3
```

- 备案号形如 `京ICP备12345678号-3`；后缀 `-3` 是网站序号，主体按号段前缀判断。
- 页脚缺失时看 `/about`、`/contact`、`/footer` 等常见页面（只访问页面中真实存在的链接）。
- 需要权威核对时，仅在可正常访问的公开备案查询页人工查看，不写自动绕过脚本。

### 2. 证书主体

```bash
echo | timeout 8 openssl s_client -connect www.example.com:443 -servername www.example.com 2>/dev/null \
  | openssl x509 -noout -subject -issuer -ext subjectAltName
```

- `subject` 中的 `O=`（Organization）与 `OU=` 是主体线索；EV 证书可信度最高。
- 免费 DV 证书通常只有 CN/SAN，不含组织信息，不能作为归属证据。
- SAN 中的其他域名同属一张证书，可作为**关联线索**，仍需其他证据确认归属。

### 3. WHOIS 与 DNS 足迹

```bash
whois example.com | grep -iE 'registrant|org|organization|country|email|name server' | head -20
dig +short NS example.com
dig +short MX example.com
```

- 隐私保护开启时 registrant 为空，属正常；不要据此推断。
- NS/MX 指向云厂商只说明托管方，不说明归属。

### 4. 页面共同特征（中等证据）

```bash
# 统计 ID
grep -oE '(UA-[0-9]{4,}-[0-9]+|G-[A-Z0-9]{6,}|GTM-[A-Z0-9]+|hm\.baidu\.com/hm\.js\?[a-f0-9]{10,})' /tmp/home.html | sort -u
# 联系方式
grep -oE '1[3-9][0-9]{9}|0[0-9]{2,3}-[0-9]{7,8}|[A-Za-z0-9._%+-]+@[A-Za-z0-9.-]+\.[A-Za-z]{2,}' /tmp/home.html | sort -u | head -20
# 版权与公司名
grep -oE '(©|Copyright|版权所有)[^<]{0,60}' /tmp/home.html | head -5
```

同一统计 ID、同一 400 电话、同一邮箱出现在多个站点，说明**很可能**同一运营方；仍需至少两条独立证据。

### 5. 应用类归因

应用商店页面或官网下载页中的开发者名称、`app_icp`、Bundle ID 前缀可作为证据；应用无域名可挂靠，需在写资产时显式关联 `company_id`。

- 只有**商店开发者、认证主体、备案主体与目标主体一致**，或目标官方页面明确声明自有时，才登记为目标 `app` 资产。
- 目标材料里「要求用户去某平台办理」（第三方政务 / 医疗 / 支付 / 预约平台）只是办理入口，**不构成归属**：不登记资产，也不填 `company_id`；需要留痕时用 `record_fact` 记录入口关系。
- 开发者或认证主体是第三方（如「XX 省政务服务和数据管理局」「XX 科技有限公司」）时，即使目标官网/公众号在指引里反复提到，也不能归到目标名下。

## 写回

**规划/主控/Auto/Pentest** 可用 `add_company_scope` 建立或扩展公司范围（worker 无此工具，只写事实与资产）：

```
list_companies(search="公司名")          # 先查重，避免同名重复创建
add_company_scope(company="某某科技有限公司",
  scope="example.com\nwww.example.com\n京ICP备12345678号\n1.2.3.0/24",
  reason="官网页脚 ICP 京ICP备12345678号-3，证书 O=某某科技有限公司；同统计 ID UA-123456-1")
```

- `scope` 一行一条：根域、URL、IP、CIDR、ICP、关键词；系统自动认领命中资产。
- 拒绝裸 TLD 与过宽网段（IPv4 仅 `/16`–`/32`），非法行会在 `errors` 返回。
- 应用资产：`insert_assets` 时传 `{"type":"app","app_name":"...","app_icp":"...","company_id":<id>}`，`company_id` 取自 `add_company_scope` 返回或 `list_companies`。
- 中等证据必须写清推断链；弱证据只 `record_fact`，例如：`a.example.com 与 b.example.net 共用 UA-123456-1，疑似同一运营方，未确认备案主体`。

## 错误处理

| 现象 | 处理 |
|---|---|
| 页脚无 ICP | 检查关于/联系页；仍无则记录「未发现备案号」，改用证书与统计 ID |
| 证书无 O 字段 | DV 证书，降级为线索，不单独归属 |
| WHOIS 隐私保护 | 跳过 registrant，不推断 |
| 同名公司已存在 | 先 `list_companies` 取 `company_id`，复用并把新范围并入，不重复建公司 |
| 证据冲突 | 以 ICP/证书为准；冲突本身写入事实，等待规划者判断 |
| 应用开发者是第三方，但目标材料要求用户使用 | 不归属；只 `record_fact` 记入口关系 |
