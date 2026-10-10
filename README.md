# hawkeye
鹰眼 Hawkeye · 外部暴露面持续监控平台。Go 编译成单个可执行文件，前端、POC 库、技术指纹库、爆破字典全部打进去，运行时不需要 Go、Node、Python，也没有任何配置文件。它把 FOFA、Hunter、Quake、ZoomEye、Shodan、Censys 与搜索引擎的结果持续落库去重，认出资产跑的是什么组件，再定向执行 POC 判定并由 AI 复核风险（未配 Key 自动降级为本地规则）；新出现的资产、改动的指纹、消失的服务都会变成一条变更记录，当天推到钉钉 / 飞书 / 企业微信 / Telegram / 邮件。
