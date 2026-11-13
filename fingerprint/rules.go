package fingerprint

import (
	"fmt"
	"html"
	"net/http"
	"regexp"
	"strings"
)

// hdrCond is one response-header condition. An empty contains means the header
// only has to be present.
type hdrCond struct {
	name     string
	contains string
}

// rule is one product/system signature. It matches when ANY of its conditions
// (header / cookie / body substring / body regex) matches; multiple entries
// inside one group are also alternatives.
type rule struct {
	product  string
	category string
	conf     string // high | medium (empty = high)
	headers  []hdrCond
	cookies  []string // cookie name prefix, case-insensitive
	body     []string // lowercase page substring
	bodyRe   *regexp.Regexp
	verRe    *regexp.Regexp // applied to header block, then body; group 1 = version
}

func h(name, contains string) hdrCond { return hdrCond{name: name, contains: strings.ToLower(contains)} }

var rules = []rule{
	// ---------------- 协同办公 / OA / ERP / 报表 ----------------
	{
		product: "泛微 e-cology", category: "OA", conf: "high",
		cookies: []string{"ecology_jsessionid"},
		body:    []string{"weaver", "e-cology", "泛微"},
		verRe:   regexp.MustCompile(`(?i)e-?cology\s*(?:v|version)?\s*([0-9]+(?:\.[0-9]+)*)`),
	},
	{
		product: "泛微 e-office", category: "OA", conf: "medium",
		body:  []string{"e-office", "eoffice"},
		verRe: regexp.MustCompile(`(?i)e-?office\s*v?([0-9]+(?:\.[0-9]+)*)`),
	},
	{
		product: "致远 A8", category: "OA", conf: "high",
		body:  []string{"seeyon", "致远"},
		verRe: regexp.MustCompile(`(?i)seeyon\s*v?([0-9]+(?:\.[0-9]+)*)`),
	},
	{
		product: "用友 NC", category: "ERP", conf: "high",
		body: []string{"nccloud", "yonyou", "用友"},
	},
	{
		product: "用友 U8 Cloud", category: "ERP", conf: "high",
		body: []string{"u8cloud", "u8c"},
	},
	{
		product: "金蝶 EAS/K3 Cloud", category: "ERP", conf: "high",
		body:  []string{"k3cloud", "easweb", "kingdee", "金蝶"},
		verRe: regexp.MustCompile(`(?i)k3\s*cloud\s*v?([0-9]+(?:\.[0-9]+)*)`),
	},
	{
		product: "通达 OA", category: "OA", conf: "high",
		body: []string{"通达oa", "tdoa", "/general/login/index.php"},
	},
	{
		product: "蓝凌 EKP", category: "协同办公", conf: "high",
		body: []string{"landray", "/ekp/", "蓝凌"},
	},
	{
		product: "万户 OA", category: "OA", conf: "high",
		body: []string{"/defaultroot/", "万户oa", "wanhu"},
	},
	{
		product: "华天动力 OA", category: "OA", conf: "high",
		body: []string{"oa8000", "华天动力"},
	},
	{
		product: "红帆 iOffice", category: "OA", conf: "medium",
		body: []string{"ioffice", "红帆"},
	},
	{
		product: "金和 OA", category: "OA", conf: "medium",
		body: []string{"jinher", "金和oa"},
	},
	{
		product: "帆软 FineReport", category: "报表工具", conf: "high",
		body:  []string{"finereport", "/webreport/", "帆软"},
		verRe: regexp.MustCompile(`(?i)finereport\s*v?([0-9]+(?:\.[0-9]+)*)`),
	},
	{
		product: "帆软 FineBI", category: "报表工具", conf: "high",
		body: []string{"finebi", "/webroot/decision"},
	},
	{
		product: "润乾报表", category: "报表工具", conf: "medium",
		body: []string{"runqian", "raqsoft"},
	},
	{
		product: "若依 RuoYi", category: "开发框架", conf: "high",
		body: []string{"ruoyi", "若依"},
	},
	{
		product: "JeecgBoot", category: "开发框架", conf: "high",
		body: []string{"jeecg"},
	},

	// ---------------- CMS / 建站 ----------------
	{
		product: "WordPress", category: "CMS", conf: "high",
		body:  []string{"wp-content", "wp-includes", "wp-json"},
		verRe: regexp.MustCompile(`(?i)wordpress\s+([0-9]+(?:\.[0-9]+)*)`),
	},
	{
		product: "Drupal", category: "CMS", conf: "high",
		body:  []string{"drupal.settings", "sites/default/files", "drupal"},
		verRe: regexp.MustCompile(`(?i)drupal\s+([0-9]+(?:\.[0-9]+)*)`),
	},
	{
		product: "Joomla", category: "CMS", conf: "high",
		body:  []string{"joomla", "/media/system/js/"},
		verRe: regexp.MustCompile(`(?i)joomla!?\s*([0-9]+(?:\.[0-9]+)*)`),
	},
	{
		product: "DedeCMS 织梦", category: "CMS", conf: "high",
		body:  []string{"dedecms", "/dede/", "templets", "power by dedecms"},
		verRe: regexp.MustCompile(`(?i)dedecms\s*v?([0-9]+(?:\.[0-9]+)*)`),
	},
	{
		product: "PHPCMS", category: "CMS", conf: "high",
		body:  []string{"phpcms", "/phpcms/"},
		verRe: regexp.MustCompile(`(?i)phpcms\s*v?([0-9]+(?:\.[0-9]+)*)`),
	},
	{
		product: "EmpireCMS 帝国", category: "CMS", conf: "high",
		body:  []string{"empirecms", "e/data/"},
		verRe: regexp.MustCompile(`(?i)empirecms\s*v?([0-9]+(?:\.[0-9]+)*)`),
	},
	{
		product: "Discuz!", category: "论坛", conf: "high",
		body:  []string{"discuz", "forum.php"},
		verRe: regexp.MustCompile(`(?i)discuz!?\s*([0-9]+(?:\.[0-9]+)*)`),
	},
	{
		product: "Typecho", category: "博客", conf: "high",
		body: []string{"typecho"},
	},
	{
		product: "Z-BlogPHP", category: "博客", conf: "high",
		body: []string{"zblogphp"},
	},
	{
		product: "PbootCMS", category: "CMS", conf: "high",
		body: []string{"pbootcms"},
	},
	{
		product: "EyouCMS", category: "CMS", conf: "high",
		body: []string{"eyoucms"},
	},
	{
		product: "苹果CMS", category: "CMS", conf: "high",
		body: []string{"maccms", "苹果cms"},
	},
	{
		product: "迅睿CMS", category: "CMS", conf: "high",
		body: []string{"xunruicms"},
	},
	{
		product: "SiteServer", category: "CMS", conf: "high",
		body: []string{"siteserver"},
	},
	{
		product: "齐博CMS", category: "CMS", conf: "medium",
		body: []string{"qibocms"},
	},
	{
		product: "海洋CMS", category: "CMS", conf: "medium",
		body: []string{"seacms"},
	},

	// ---------------- 运维 / 中间件管理面 ----------------
	{
		product: "宝塔面板", category: "运维面板", conf: "high",
		body: []string{"宝塔", "bt-panel", "btwaf"},
	},
	{
		product: "1Panel", category: "运维面板", conf: "high",
		body: []string{"1panel"},
	},
	{
		product: "phpMyAdmin", category: "数据库管理", conf: "high",
		cookies: []string{"pma_lang", "phpmyadmin"},
		body:    []string{"phpmyadmin"},
	},
	{
		product: "Adminer", category: "数据库管理", conf: "medium",
		body: []string{"adminer"},
	},
	{
		product: "Jenkins", category: "持续集成", conf: "high",
		headers: []hdrCond{h("X-Jenkins", "")},
		body:    []string{"welcome to jenkins", "dashboard [jenkins]"},
		verRe:   regexp.MustCompile(`(?i)jenkins[/ ]v?([0-9]+(?:\.[0-9]+)*)`),
	},
	{
		product: "GitLab", category: "代码托管", conf: "high",
		cookies: []string{"_gitlab_session"},
		body:    []string{"gitlab"},
	},
	{
		product: "Jira", category: "项目管理", conf: "high",
		headers: []hdrCond{h("X-AREQUESTID", "")},
		body:    []string{"jira", "atlassian"},
	},
	{
		product: "Confluence", category: "知识库", conf: "high",
		body: []string{"confluence"},
	},
	{
		product: "Grafana", category: "监控系统", conf: "high",
		body: []string{"grafana"},
	},
	{
		product: "Kibana", category: "日志分析", conf: "high",
		body: []string{"kibana"},
	},
	{
		product: "Zabbix", category: "监控系统", conf: "high",
		cookies: []string{"zbx_session"},
		body:    []string{"zabbix"},
	},
	{
		product: "Prometheus", category: "监控系统", conf: "high",
		body: []string{"prometheus"},
	},
	{
		product: "Nacos", category: "配置中心", conf: "high",
		body: []string{"nacos"},
	},
	{
		product: "MinIO", category: "对象存储", conf: "high",
		body: []string{"minio"},
	},
	{
		product: "Elasticsearch", category: "搜索服务", conf: "high",
		body: []string{"you know, for search"},
	},
	{
		product: "RabbitMQ Management", category: "消息队列", conf: "high",
		body: []string{"rabbitmq management"},
	},
	{
		product: "Docker Registry", category: "容器镜像仓库", conf: "high",
		headers: []hdrCond{h("Docker-Distribution-Api-Version", "")},
	},
	{
		product: "Harbor", category: "容器镜像仓库", conf: "medium",
		body: []string{"harbor"},
	},
	{
		product: "SonarQube", category: "代码质量", conf: "high",
		body: []string{"sonarqube"},
	},
	{
		product: "Nexus Repository", category: "制品仓库", conf: "high",
		body: []string{"nexus repository", "nexus3"},
	},
	{
		product: "Apache Solr", category: "搜索服务", conf: "medium",
		body: []string{"solr"},
	},
	{
		product: "Druid Stat", category: "监控系统", conf: "medium",
		body: []string{"druid stat"},
	},
	{
		product: "Kubernetes Dashboard", category: "容器管理", conf: "high",
		body: []string{"kubernetes dashboard", "k8s dashboard"},
	},
	{
		product: "Hadoop", category: "大数据平台", conf: "medium",
		body: []string{"hadoop", "namenode"},
	},

	// ---------------- WAF / CDN / 安全设备 ----------------
	{
		product: "Cloudflare", category: "CDN/WAF", conf: "high",
		headers: []hdrCond{h("cf-ray", "")},
	},
	{
		product: "Akamai", category: "CDN", conf: "high",
		headers: []hdrCond{h("Server", "akamaighost")},
	},
	{
		product: "Fastly", category: "CDN", conf: "high",
		headers: []hdrCond{h("X-Fastly-Request-ID", "")},
	},
	{
		product: "Amazon CloudFront", category: "CDN", conf: "high",
		headers: []hdrCond{h("X-Amz-Cf-Id", "")},
	},
	{
		product: "Vercel", category: "CDN", conf: "high",
		headers: []hdrCond{h("X-Vercel-Id", ""), h("Server", "vercel")},
	},
	{
		product: "阿里云 CDN", category: "CDN", conf: "high",
		headers: []hdrCond{h("ali-swift-global-savetime", ""), h("eagleid", "")},
	},
	{
		product: "腾讯云 CDN", category: "CDN", conf: "high",
		headers: []hdrCond{h("X-Nws-Log-Uuid", ""), h("Server", "nws_sp")},
	},
	{
		product: "百度云加速", category: "CDN", conf: "high",
		headers: []hdrCond{h("Server", "yunjiasu")},
	},
	{
		product: "七牛云 CDN", category: "CDN", conf: "high",
		headers: []hdrCond{h("X-Qiniu-Zone", "")},
	},
	{
		product: "阿里云 WAF", category: "WAF", conf: "medium",
		cookies: []string{"acw_tc"},
	},
	{
		product: "创宇盾", category: "WAF", conf: "high",
		cookies: []string{"yunsuo_session_verify"},
		body:    []string{"yunsuo"},
	},
	{
		product: "安全狗", category: "WAF", conf: "high",
		cookies: []string{"safedog-flow-item"},
		body:    []string{"safedog"},
	},
	{
		product: "360 网站卫士", category: "WAF", conf: "high",
		cookies: []string{"wzws_sessionid"},
		body:    []string{"360wzws"},
	},
	{
		product: "Imperva", category: "WAF", conf: "high",
		cookies: []string{"visid_incap", "incap_ses_"},
	},
	{
		product: "F5 BIG-IP", category: "负载均衡", conf: "high",
		cookies: []string{"bigipserver"},
	},
	{
		product: "深信服", category: "安全设备", conf: "high",
		body: []string{"sangfor"},
	},
	{
		product: "绿盟", category: "安全设备", conf: "high",
		body: []string{"nsfocus"},
	},
	{
		product: "天融信", category: "安全设备", conf: "high",
		body: []string{"topsec"},
	},
	{
		product: "启明星辰", category: "安全设备", conf: "high",
		body: []string{"venustech"},
	},
	{
		product: "奇安信", category: "安全设备", conf: "high",
		body: []string{"qianxin", "legendsec"},
	},
	{
		product: "安恒信息", category: "安全设备", conf: "high",
		body: []string{"dbappsecurity", "安恒"},
	},
	{
		product: "Fortinet FortiGate", category: "安全设备", conf: "high",
		body: []string{"fortinet", "fortigate", "remote/login"},
	},
	{
		product: "Citrix NetScaler", category: "VPN", conf: "high",
		cookies: []string{"nsc_"},
		body:    []string{"citrix", "/vpn/"},
	},
	{
		product: "Pulse Secure", category: "VPN", conf: "high",
		body: []string{"pulse secure", "/dana-na/"},
	},
	{
		product: "Palo Alto GlobalProtect", category: "VPN", conf: "high",
		body: []string{"globalprotect"},
	},

	// ---------------- Web 服务器 / 中间件 ----------------
	{
		product: "Nginx", category: "Web 服务器", conf: "high",
		headers: []hdrCond{h("Server", "nginx")},
		verRe:   regexp.MustCompile(`(?i)nginx/([0-9][0-9a-z.]*)`),
	},
	{
		product: "OpenResty", category: "Web 服务器", conf: "high",
		headers: []hdrCond{h("Server", "openresty")},
		verRe:   regexp.MustCompile(`(?i)openresty/([0-9][0-9a-z.]*)`),
	},
	{
		product: "Tengine", category: "Web 服务器", conf: "high",
		headers: []hdrCond{h("Server", "tengine")},
		verRe:   regexp.MustCompile(`(?i)tengine/([0-9][0-9a-z.]*)`),
	},
	{
		product: "Apache HTTP Server", category: "Web 服务器", conf: "high",
		headers: []hdrCond{h("Server", "apache/")},
		verRe:   regexp.MustCompile(`(?i)apache/([0-9.]+)`),
	},
	{
		product: "Microsoft IIS", category: "Web 服务器", conf: "high",
		headers: []hdrCond{h("Server", "microsoft-iis")},
		verRe:   regexp.MustCompile(`(?i)microsoft-iis/([0-9.]+)`),
	},
	{
		product: "LiteSpeed", category: "Web 服务器", conf: "high",
		headers: []hdrCond{h("Server", "litespeed")},
	},
	{
		product: "Caddy", category: "Web 服务器", conf: "high",
		headers: []hdrCond{h("Server", "caddy")},
	},
	{
		product: "Apache Tomcat", category: "中间件", conf: "high",
		headers: []hdrCond{h("Server", "apache-coyote"), h("Server", "tomcat")},
		body:    []string{"apache tomcat"},
	},
	{
		product: "Jetty", category: "中间件", conf: "high",
		headers: []hdrCond{h("Server", "jetty")},
	},
	{
		product: "WildFly / JBoss", category: "中间件", conf: "high",
		headers: []hdrCond{h("Server", "wildfly"), h("Server", "jboss")},
	},
	{
		product: "Undertow", category: "中间件", conf: "high",
		headers: []hdrCond{h("X-Powered-By", "undertow"), h("Server", "undertow")},
	},
	{
		product: "Oracle WebLogic", category: "中间件", conf: "high",
		headers: []hdrCond{h("Server", "weblogic")},
	},
	{
		product: "Kestrel", category: "Web 服务器", conf: "high",
		headers: []hdrCond{h("Server", "kestrel")},
	},
	{
		product: "Werkzeug", category: "Web 框架", conf: "high",
		headers: []hdrCond{h("Server", "werkzeug")},
	},
	{
		product: "Gunicorn", category: "Web 服务器", conf: "high",
		headers: []hdrCond{h("Server", "gunicorn")},
	},
	{
		product: "Uvicorn", category: "Web 服务器", conf: "high",
		headers: []hdrCond{h("Server", "uvicorn")},
	},

	// ---------------- 语言 / 框架 / 前端 ----------------
	{
		product: "PHP", category: "开发语言", conf: "high",
		headers: []hdrCond{h("X-Powered-By", "php")},
		cookies: []string{"phpsessid"},
		verRe:   regexp.MustCompile(`(?i)php/([0-9.]+)`),
	},
	{
		product: "ASP.NET", category: "开发语言", conf: "high",
		headers: []hdrCond{h("X-AspNet-Version", ""), h("X-AspNetMvc-Version", "")},
		cookies: []string{"asp.net_sessionid", ".aspxauth"},
		verRe:   regexp.MustCompile(`(?i)x-aspnet-version:\s*([0-9.]+)`),
	},
	{
		product: "ASP.NET Core", category: "开发语言", conf: "high",
		cookies: []string{".aspnetcore."},
	},
	{
		product: "Java", category: "开发语言", conf: "medium",
		cookies: []string{"jsessionid"},
	},
	{
		product: "Spring Boot", category: "Web 框架", conf: "high",
		body: []string{"whitelabel error page"},
	},
	{
		product: "Apache Shiro", category: "Web 框架", conf: "medium",
		cookies: []string{"rememberme"},
	},
	{
		product: "ThinkPHP", category: "Web 框架", conf: "high",
		headers: []hdrCond{h("X-Powered-By", "thinkphp")},
		cookies: []string{"think_var", "think_lang"},
		body:    []string{"thinkphp"},
		verRe:   regexp.MustCompile(`(?i)thinkphp\s*v?([0-9]+(?:\.[0-9]+)*)`),
	},
	{
		product: "Laravel", category: "Web 框架", conf: "high",
		cookies: []string{"laravel_session"},
	},
	{
		product: "Django", category: "Web 框架", conf: "medium",
		cookies: []string{"csrftoken"},
	},
	{
		product: "Next.js", category: "前端框架", conf: "high",
		body: []string{"__next_data__", "/_next/static/"},
	},
	{
		product: "Nuxt", category: "前端框架", conf: "high",
		body: []string{"__nuxt__", "/_nuxt/"},
	},
	{
		product: "Angular", category: "前端框架", conf: "high",
		body: []string{"ng-version="},
	},
	{
		product: "Vue.js", category: "前端框架", conf: "medium",
		body: []string{"data-v-", "__vue__"},
	},
	{
		product: "React", category: "前端框架", conf: "medium",
		body: []string{"data-reactroot", "react.production.min.js", "react-dom"},
	},
	{
		product: "Layui", category: "前端框架", conf: "medium",
		body: []string{"layui"},
	},
	{
		product: "Bootstrap", category: "前端框架", conf: "medium",
		body: []string{"bootstrap.min.css", "bootstrap.css"},
	},
	{
		product: "jQuery", category: "前端框架", conf: "medium",
		body: []string{"jquery"},
	},
}

// Identify matches a response against the rule table. Results are ordered by
// rule specificity (the table above is ordered specific → generic) and deduped
// by product name.
func Identify(header http.Header, cookies []*http.Cookie, body []byte) []Match {
	ctx := &matchCtx{
		header: header,
		body:   strings.ToLower(string(body)),
	}
	for _, c := range cookies {
		ctx.cookies = append(ctx.cookies, strings.ToLower(c.Name))
	}

	var out []Match
	seen := map[string]bool{}
	add := func(m Match) {
		if m.Product == "" || seen[strings.ToLower(m.Product)] {
			return
		}
		seen[strings.ToLower(m.Product)] = true
		if m.Confidence == "" {
			m.Confidence = "high"
		}
		out = append(out, m)
	}

	for i := range rules {
		if m, ok := rules[i].match(ctx); ok {
			add(m)
		}
	}
	// A <meta name="generator"> value is often the only hint for smaller CMSes.
	if gen := generator(body); gen != "" {
		add(Match{
			Product:    gen,
			Category:   "站点生成器",
			Confidence: "medium",
			Evidence:   "<meta name=\"generator\"> = " + gen,
		})
	}
	return out
}

// Technologies flattens matches into the asset `technologies` chips: product
// (with version) plus a category tag for application-level systems, skipping
// infrastructure categories that the product name already conveys.
func Technologies(matches []Match) []string {
	var out []string
	seen := map[string]bool{}
	push := func(s string) {
		s = strings.TrimSpace(s)
		if s == "" || seen[strings.ToLower(s)] {
			return
		}
		seen[strings.ToLower(s)] = true
		out = append(out, s)
	}
	for _, m := range matches {
		name := m.Product
		if m.Version != "" {
			name += " " + m.Version
		}
		push(name)
		if emitCategory(m.Category) {
			push(m.Category)
		}
		if len(out) >= 16 {
			break
		}
	}
	return out
}

func emitCategory(c string) bool {
	switch c {
	case "", "Web 服务器", "中间件", "开发语言", "Web 框架", "前端框架":
		return false
	}
	return true
}

type matchCtx struct {
	header  http.Header
	cookies []string
	body    string
}

func (c *matchCtx) hasCookiePrefix(prefix string) bool {
	for _, name := range c.cookies {
		if strings.HasPrefix(name, prefix) {
			return true
		}
	}
	return false
}

func (r *rule) match(c *matchCtx) (Match, bool) {
	var evidence []string
	matched := false
	for _, cond := range r.headers {
		vals := c.header.Values(cond.name)
		if len(vals) == 0 {
			continue
		}
		joined := strings.ToLower(strings.Join(vals, " "))
		if cond.contains != "" && !strings.Contains(joined, cond.contains) {
			continue
		}
		matched = true
		evidence = append(evidence, fmt.Sprintf("响应头 %s: %s", cond.name, truncate(strings.Join(vals, " "), 60)))
	}
	for _, prefix := range r.cookies {
		if c.hasCookiePrefix(prefix) {
			matched = true
			evidence = append(evidence, "Cookie "+prefix)
		}
	}
	for _, needle := range r.body {
		if strings.Contains(c.body, needle) {
			matched = true
			evidence = append(evidence, "页面含 \""+needle+"\"")
		}
	}
	if r.bodyRe != nil {
		if s := r.bodyRe.FindString(c.body); s != "" {
			matched = true
			evidence = append(evidence, "页面匹配 "+truncate(s, 40))
		}
	}
	if !matched {
		return Match{}, false
	}

	m := Match{Product: r.product, Category: r.category, Confidence: r.conf}
	if r.verRe != nil {
		m.Version = versionOf(r.verRe, headerText(c.header), c.body)
	}
	m.Evidence = truncate(strings.Join(dedupe(evidence), "；"), 200)
	return m, true
}

func headerText(h http.Header) string {
	var b strings.Builder
	for k, vs := range h {
		b.WriteString(k)
		b.WriteString(": ")
		b.WriteString(strings.Join(vs, " "))
		b.WriteString("\n")
	}
	return b.String()
}

func versionOf(re *regexp.Regexp, header, body string) string {
	if m := re.FindStringSubmatch(header); len(m) > 1 {
		return m[1]
	}
	if m := re.FindStringSubmatch(body); len(m) > 1 {
		return m[1]
	}
	return ""
}

var reGenerator = regexp.MustCompile(`(?is)<meta[^>]+name\s*=\s*["']generator["'][^>]*content\s*=\s*["']([^"']+)["']`)
var reGeneratorRev = regexp.MustCompile(`(?is)<meta[^>]+content\s*=\s*["']([^"']+)["'][^>]*name\s*=\s*["']generator["']`)

func generator(body []byte) string {
	m := reGenerator.FindSubmatch(body)
	if m == nil {
		m = reGeneratorRev.FindSubmatch(body)
	}
	if m == nil {
		return ""
	}
	return truncate(strings.TrimSpace(html.UnescapeString(string(m[1]))), 60)
}

func truncate(s string, n int) string {
	runes := []rune(strings.TrimSpace(s))
	if len(runes) <= n {
		return string(runes)
	}
	if n <= 1 {
		return "…"
	}
	return string(runes[:n-1]) + "…"
}

func dedupe(in []string) []string {
	seen := make(map[string]bool, len(in))
	out := in[:0]
	for _, s := range in {
		if seen[s] {
			continue
		}
		seen[s] = true
		out = append(out, s)
	}
	return out
}
