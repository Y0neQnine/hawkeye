package agent

import (
	"strings"

	"github.com/Autumn-27/artex/db"
)

// constraintBlock renders this task's operation constraints (task_constraints) as a
// high-priority block appended to the planner/worker system prompt. allow/deny are
// grouped; empty string when there are no constraints (or ts is nil). The framing
// deliberately puts these ABOVE the exploration/expansion heuristics so a declared
// boundary wins the tug-of-war against "chase another entry surface".
func constraintBlock(ts *db.ExplorationStore) string {
	if ts == nil {
		return ""
	}
	rows, err := ts.ListConstraints()
	if err != nil || len(rows) == 0 {
		return ""
	}
	var allow, deny []string
	for _, c := range rows {
		text := strings.TrimSpace(c.Text)
		if text == "" {
			continue
		}
		if c.Kind == "allow" {
			allow = append(allow, "- "+text)
		} else {
			deny = append(deny, "- "+text)
		}
	}
	if len(allow) == 0 && len(deny) == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString("\n\n【操作约束（最高优先级，凌驾于下方一切探索/拓面启发式；每生成一条意图、每执行一个动作前都必须先自检是否违反，违反即不得进行）】：")
	if len(allow) > 0 {
		b.WriteString("\n允许的操作：\n")
		b.WriteString(strings.Join(allow, "\n"))
	}
	if len(deny) > 0 {
		b.WriteString("\n禁止的操作：\n")
		b.WriteString(strings.Join(deny, "\n"))
	}
	b.WriteString("\n（发现约束之外的新目标/新端口/新主机，不等于获得授权：除非它落在上述允许范围内，否则记为 out-of-scope 事实并跳过，不得为其派生意图或执行动作。）")
	return b.String()
}

// assetsOnlyBlock renders the task-level "assets only" collection mode (the New
// Task toggle, tasks.assets_only). On, it tells planner/worker to collect the
// asset inventory only — domains/DNS/IP/ports/services — and never request or
// enumerate URL paths or endpoints. Framed like constraintBlock so it wins over
// the expansion heuristics; empty string when the mode is off.
func assetsOnlyBlock(enabled bool) string {
	if !enabled {
		return ""
	}
	return "\n\n【采集模式：仅资产（创建任务时开启，最高优先级）】：" +
		"\n- 只采集：根域名、子域名、DNS 记录、IP、端口、服务（可含证书、指纹等技术属性）。" +
		"\n- 禁止采集、请求或枚举任何 URL 路径、URI、接口/端点、目录、参数与页面内容；禁止目录扫描、爬虫式路径发现、从页面/JS 中提取路径。" +
		"\n- 每条意图、每个执行动作都必须落在上述资产类型内；必须访问路径才能取得的线索，记为 out-of-scope 事实并跳过，不得为其派生意图或发起请求。"
}
