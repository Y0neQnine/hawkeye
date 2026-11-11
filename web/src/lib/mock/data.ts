// Rich mock dataset for the Vercel demo (NEXT_PUBLIC_MOCK=1). Shapes track the
// current backend contract in lib/api.ts + lib/types.ts. All data is static and
// self-consistent (one company "Acme", a few tasks, exploration graph,
// sessions/activity, traffic, agents, etc.) so every page looks populated.

import type {
  Activity,
  Agent,
  AgentDetail,
  Asset,
  AssetRef,
  Audit,
  Company,
  Conversation,
  ConvTokenSummary,
  DailyTokenBucket,
  Edge,
  InterceptApprovalRow,
  InterceptAudit,
  InterceptPending,
  InterceptRule,
  LLMPoolStatus,
  LLMProfile,
  LLMTask,
  MCPServer,
  MCPTool,
  MissingSkill,
  PromptVar,
  PromptVersion,
  SessionTokenUsage,
  Settings,
  SkillCall,
  SkillItem,
  Stats,
  Task,
  TaskCategory,
  TaskNode,
  TaskTemplate,
  TokenTotal,
  TokenUsage,
  Tool,
  TrafficDetail,
  TrafficHost,
  TrafficResp,
} from "@/lib/types";

const T = (iso: string) => iso; // readability helper for timestamps

// ── Tasks ──────────────────────────────────────────────────────────────────
export const ACTIVE_TASK = "t-acme-web";

export const tasks: Task[] = [
  {
    id: "t-acme-web",
    category_id: 1,
    category_name: "外部攻击面",
    description: "Acme 官网攻击面采集与持续监控（acme.com）",
    goal: "采集 acme.com 全部子域、站点与端点，并建立每日变更监控。",
    status: "running",
    created_at: T("2026-07-24T09:12:00Z"),
    created_unix: 1785489120,
    last_activity: T("2026-07-26T03:58:22Z"),
    last_activity_unix: 1785643102,
    paused: false,
    active: true,
    in_flight: 3,
    stalled: false,
    goals_total: 5,
    goals_met: 2,
    engine_mode: "exploring",
    llm_profile_id: 1,
    llm_profile_ids: [1, 2],
    active_llm_profile_id: 1,
    llm_failover_state: "ready",
    source_task_ids: [],
    tokens: { input_tokens: 1284500, output_tokens: 96320, cache_read_tokens: 890400, cache_write_tokens: 132000 },
  },
  {
    id: "t-acme-api",
    category_id: 2,
    category_name: "API 资产",
    description: "api.acme.com 接口清单与参数采集",
    goal: "采集 api.acme.com 订单/用户接口、参数与响应结构。",
    status: "running",
    created_at: T("2026-07-25T14:05:00Z"),
    created_unix: 1785592500,
    last_activity: T("2026-07-26T03:40:10Z"),
    last_activity_unix: 1785642010,
    paused: false,
    active: false,
    in_flight: 2,
    stalled: false,
    goals_total: 4,
    goals_met: 1,
    engine_mode: "exploring",
    llm_profile_id: 1,
    llm_profile_ids: [1, 2],
    active_llm_profile_id: 1,
    llm_failover_state: "ready",
    source_task_ids: ["t-acme-web"],
    tokens: { input_tokens: 642300, output_tokens: 51200, cache_read_tokens: 401000, cache_write_tokens: 60000 },
  },
  {
    id: "t-shop-pay",
    category_id: 2,
    category_name: "API 资产",
    description: "shop.acme.com 支付与订单链路资产采集",
    goal: "采集支付与订单链路涉及的域名、接口与第三方依赖。",
    status: "paused",
    created_at: T("2026-07-22T08:30:00Z"),
    created_unix: 1785313800,
    last_activity: T("2026-07-23T18:40:10Z"),
    last_activity_unix: 1785436810,
    paused: true,
    active: false,
    in_flight: 0,
    stalled: false,
    goals_total: 4,
    goals_met: 1,
    engine_mode: "paused",
    llm_profile_id: 2,
    llm_profile_ids: [2, 1],
    active_llm_profile_id: 2,
    llm_failover_state: "ready",
    source_task_ids: ["t-acme-web", "t-acme-api"],
    tokens: { input_tokens: 233000, output_tokens: 18700, cache_read_tokens: 120000, cache_write_tokens: 22000 },
  },
  {
    id: "t-vpn-edge",
    category_id: 1,
    category_name: "外部攻击面",
    description: "对外暴露面采集（VPN / 边界服务）",
    goal: "识别 acme.com 对外暴露的边界服务与指纹。",
    status: "done",
    created_at: T("2026-07-18T08:00:00Z"),
    created_unix: 1784966400,
    completed_at: T("2026-07-20T22:11:00Z"),
    completed_unix: 1785190260,
    last_activity: T("2026-07-20T22:11:00Z"),
    last_activity_unix: 1785190260,
    paused: false,
    active: false,
    in_flight: 0,
    stalled: false,
    goals_total: 3,
    goals_met: 3,
    engine_mode: "idle",
    llm_profile_id: 1,
    llm_profile_ids: [1],
    active_llm_profile_id: 1,
    llm_failover_state: "ready",
    source_task_ids: [],
    tokens: { input_tokens: 512000, output_tokens: 40100, cache_read_tokens: 300000, cache_write_tokens: 41000 },
  },
];

export const taskCategories: TaskCategory[] = [
  {
    id: 1,
    name: "外部攻击面",
    task_count: 2,
    created_at: T("2026-07-18T08:00:00Z"),
    updated_at: T("2026-07-25T08:00:00Z"),
  },
  {
    id: 2,
    name: "API 资产",
    task_count: 2,
    created_at: T("2026-07-19T08:00:00Z"),
    updated_at: T("2026-07-24T08:00:00Z"),
  },
];

export const taskTemplates: TaskTemplate[] = [
  {
    id: 1,
    name: "外部 Web 资产采集",
    description: "对目标互联网暴露面开展子域、站点与端点采集，覆盖常见管理入口。",
    goal: "形成完整的站点、接口与端点资产清单，并持续跟踪变更。",
    created_at: T("2026-07-20T08:00:00Z"),
    updated_at: T("2026-07-25T08:00:00Z"),
  },
  {
    id: 2,
    name: "API 资产汇总",
    description: "围绕目标 API 的域名、端点、参数与认证方式开展系统采集。",
    goal: "输出 API 端点、参数与归属关系，作为持续监控基线。",
    created_at: T("2026-07-19T08:00:00Z"),
    updated_at: T("2026-07-24T08:00:00Z"),
  },
];

export const getTask = (id: string) => tasks.find((t) => t.id === id);

const grandTotal: TokenTotal = tasks.reduce(
  (a, t) => ({
    input_tokens: a.input_tokens + (t.tokens?.input_tokens ?? 0),
    output_tokens: a.output_tokens + (t.tokens?.output_tokens ?? 0),
    cache_read_tokens: a.cache_read_tokens + (t.tokens?.cache_read_tokens ?? 0),
    cache_write_tokens: a.cache_write_tokens + (t.tokens?.cache_write_tokens ?? 0),
  }),
  { input_tokens: 0, output_tokens: 0, cache_read_tokens: 0, cache_write_tokens: 0 },
);

// ── Stats ──────────────────────────────────────────────────────────────────
export function stats(
  taskId?: string,
  state: { tasks?: readonly Task[]; activeTask?: string } = {},
): Stats & { active_task?: unknown } {
  const taskList = state.tasks ?? tasks;
  const base: Stats = {
    assets: assets.length,
    engine_mode: "exploring",
    llm_configured: true,
    roe_enabled: true,
  };
  const selectedTaskID = taskId ?? state.activeTask ?? ACTIVE_TASK;
  const t = taskList.find((item) => item.id === selectedTaskID);
  if (!t) return base;
  return {
    ...base,
    engine_mode: t.engine_mode ?? "idle",
    active_task: {
      id: t.id,
      in_flight: t.in_flight,
      goals_total: t.goals_total,
      goals_met: t.goals_met,
      engine_mode: t.engine_mode,
      paused: t.paused,
    },
  };
}

// ── Company + Assets (new unified model) ─────────────────────────────────────
export const companies: Company[] = [
  {
    id: 1,
    name: "Acme Corp",
    logo: "",
    asset_count: 18,
    scope: [
      { id: 1, company_id: 1, kind: "domain", domain: "acme.com", raw: "acme.com", reason: "主域" },
      { id: 2, company_id: 1, kind: "cidr", net: "203.0.113.0/24", raw: "203.0.113.0/24" },
      { id: 3, company_id: 1, kind: "ip", net: "198.51.100.20", raw: "198.51.100.20" },
    ],
  },
];

export const assets: Asset[] = [
  {
    id: 1,
    type: "root_domain",
    company_id: 1,
    task_ids: [1],
    domain: "acme.com",
    root_domain: "acme.com",
    icp: "京ICP备2021xxxx号",
    record_type: "A",
    record_value: ["203.0.113.10"],
    last_seen: T("2026-07-26T02:00:00Z"),
  },
  {
    id: 2,
    type: "subdomain",
    company_id: 1,
    task_ids: [1],
    domain: "www.acme.com",
    root_domain: "acme.com",
    record_type: "CNAME",
    record_value: "acme.com",
    last_seen: T("2026-07-26T02:01:00Z"),
  },
  {
    id: 3,
    type: "subdomain",
    company_id: 1,
    task_ids: [2],
    domain: "api.acme.com",
    root_domain: "acme.com",
    record_type: "A",
    record_value: ["203.0.113.11"],
    last_seen: T("2026-07-26T02:02:00Z"),
  },
  {
    id: 4,
    type: "subdomain",
    company_id: 1,
    task_ids: [3],
    domain: "shop.acme.com",
    root_domain: "acme.com",
    record_type: "A",
    record_value: ["203.0.113.12"],
    last_seen: T("2026-07-26T02:03:00Z"),
  },
  {
    id: 5,
    type: "subdomain",
    company_id: 1,
    task_ids: [4],
    domain: "vpn.acme.com",
    root_domain: "acme.com",
    record_type: "A",
    record_value: ["198.51.100.20"],
    last_seen: T("2026-07-20T02:00:00Z"),
  },
  {
    id: 6,
    type: "subdomain",
    company_id: 1,
    task_ids: [1],
    domain: "admin.acme.com",
    root_domain: "acme.com",
    record_type: "A",
    record_value: ["203.0.113.10"],
    last_seen: T("2026-07-26T02:04:00Z"),
  },
  {
    id: 7,
    type: "ip",
    company_id: 1,
    task_ids: [1, 2],
    ip: "203.0.113.10",
    c_segment: "203.0.113.0",
    open_ports: [
      { port: 80, service: "http" },
      { port: 443, service: "https" },
      { port: 22, service: "ssh" },
    ],
    last_seen: T("2026-07-26T02:05:00Z"),
  },
  {
    id: 8,
    type: "ip",
    company_id: 1,
    task_ids: [2],
    ip: "203.0.113.11",
    c_segment: "203.0.113.0",
    open_ports: [{ port: 443, service: "https" }],
    last_seen: T("2026-07-26T02:06:00Z"),
  },
  {
    id: 9,
    type: "ip",
    company_id: 1,
    task_ids: [4],
    ip: "198.51.100.20",
    c_segment: "198.51.100.0",
    open_ports: [
      { port: 443, service: "https" },
      { port: 500, service: " isakmp" },
    ],
    last_seen: T("2026-07-20T02:00:00Z"),
  },
  {
    id: 10,
    type: "service",
    company_id: 1,
    task_ids: [1],
    ip: "203.0.113.10",
    port: 443,
    service_type: "https",
    service_name: "nginx/1.24.0",
    last_seen: T("2026-07-26T02:07:00Z"),
  },
  {
    id: 11,
    type: "service",
    company_id: 1,
    task_ids: [4],
    ip: "198.51.100.20",
    port: 22,
    service_type: "ssh",
    service_name: "OpenSSH 8.9p1",
    last_seen: T("2026-07-20T02:00:00Z"),
  },
  {
    id: 12,
    type: "app",
    company_id: 1,
    task_ids: [1],
    url: "https://www.acme.com",
    app_name: "Acme 官网",
    category: "corp",
    status_code: 200,
    content_length: 48213,
    page_title: "Acme Corp — 企业官网",
    technologies: ["Nginx", "React", "Cloudflare"],
    favicon_mmh3: "-1580860059",
    last_seen: T("2026-07-26T02:08:00Z"),
  },
  {
    id: 13,
    type: "app",
    company_id: 1,
    task_ids: [1],
    url: "https://admin.acme.com",
    app_name: "后台管理",
    category: "admin",
    status_code: 200,
    content_length: 12044,
    page_title: "Acme Admin 登录",
    technologies: ["Nginx", "Vue", "Element-UI"],
    last_seen: T("2026-07-26T02:09:00Z"),
  },
  {
    id: 14,
    type: "app",
    company_id: 1,
    task_ids: [2],
    url: "https://api.acme.com",
    app_name: "REST API",
    category: "api",
    status_code: 401,
    content_length: 45,
    page_title: "",
    technologies: ["Kong", "Node.js"],
    last_seen: T("2026-07-26T02:10:00Z"),
  },
  {
    id: 15,
    type: "endpoint",
    company_id: 1,
    task_ids: [2],
    url: "https://api.acme.com/v1/orders",
    method: "GET",
    status_code: 200,
    params: [
      { name: "id", in: "query" },
      { name: "page", in: "query" },
    ],
    last_seen: T("2026-07-26T02:11:00Z"),
  },
  {
    id: 16,
    type: "endpoint",
    company_id: 1,
    task_ids: [2],
    url: "https://api.acme.com/v1/users/me",
    method: "GET",
    status_code: 200,
    last_seen: T("2026-07-26T02:12:00Z"),
  },
  {
    id: 17,
    type: "endpoint",
    company_id: 1,
    task_ids: [1],
    url: "https://admin.acme.com/login",
    method: "POST",
    status_code: 200,
    params: [{ name: "username" }, { name: "password" }],
    last_seen: T("2026-07-26T02:13:00Z"),
  },
  {
    id: 18,
    type: "endpoint",
    company_id: 1,
    task_ids: [1],
    url: "https://www.acme.com/search",
    method: "GET",
    status_code: 200,
    params: [{ name: "q", in: "query" }],
    last_seen: T("2026-07-26T02:14:00Z"),
  },
  // 内网资产（DMZ→内网采集发现）
  {
    id: 19,
    type: "ip",
    company_id: 1,
    task_ids: [1],
    ip: "10.0.20.15",
    c_segment: "10.0.20.0",
    open_ports: [
      { port: 80, service: "http" },
      { port: 22, service: "ssh" },
    ],
    last_seen: T("2026-07-25T15:40:00Z"),
  },
  {
    id: 20,
    type: "ip",
    company_id: 1,
    task_ids: [1],
    ip: "10.10.10.10",
    c_segment: "10.10.10.0",
    open_ports: [
      { port: 88, service: "kerberos" },
      { port: 389, service: "ldap" },
      { port: 445, service: "smb" },
    ],
    last_seen: T("2026-07-25T21:00:00Z"),
  },
  {
    id: 21,
    type: "ip",
    company_id: 1,
    task_ids: [1],
    ip: "10.10.10.5",
    c_segment: "10.10.10.0",
    open_ports: [{ port: 445, service: "smb" }],
    last_seen: T("2026-07-25T21:00:00Z"),
  },
  {
    id: 22,
    type: "ip",
    company_id: 1,
    task_ids: [1],
    ip: "10.10.10.20",
    c_segment: "10.10.10.0",
    open_ports: [{ port: 8080, service: "http-proxy" }],
    last_seen: T("2026-07-25T21:00:00Z"),
  },
  {
    id: 23,
    type: "service",
    company_id: 1,
    task_ids: [1],
    ip: "10.10.10.20",
    port: 8080,
    service_type: "http",
    service_name: "Jenkins 2.289",
    last_seen: T("2026-07-25T21:35:00Z"),
  },
  {
    id: 24,
    type: "service",
    company_id: 1,
    task_ids: [1],
    ip: "10.10.10.10",
    port: 445,
    service_type: "smb",
    service_name: "Windows Server 2019 DC",
    last_seen: T("2026-07-25T23:55:00Z"),
  },
  {
    id: 25,
    type: "app",
    company_id: 1,
    task_ids: [1],
    url: "http://10.10.10.20:8080",
    app_name: "Jenkins (内部)",
    category: "ci",
    status_code: 200,
    content_length: 14200,
    page_title: "Jenkins Dashboard",
    technologies: ["Jenkins", "Java"],
    last_seen: T("2026-07-25T21:35:00Z"),
  },
  {
    id: 26,
    type: "endpoint",
    company_id: 1,
    task_ids: [1],
    url: "http://10.10.10.20:8080/script",
    method: "GET",
    status_code: 200,
    last_seen: T("2026-07-25T21:35:00Z"),
  },
];

export const assetCounts: Record<string, number> = assets.reduce<Record<string, number>>((m, a) => {
  m[a.type] = (m[a.type] ?? 0) + 1;
  return m;
}, {});

// assetRef 把资产 id 变成探索节点上挂的资产引用(label 与后端 coverageNodeLabel
// 的取值顺序一致:URL > 域名 > IP > 应用名)。
function assetRef(id: number): AssetRef {
  const asset = assets.find((candidate) => candidate.id === id);
  if (!asset) throw new Error(`mock assetRef: unknown asset ${id}`);
  return {
    id: String(id),
    type: asset.type,
    label: asset.url || asset.domain || asset.ip || asset.app_name || String(id),
  };
}

// ── Exploration graph (active task) ──────────────────────────────────────────
// 现行模型：根是 fact/state=origin（渲染为「起点」）；payload 为 JSON 字符串，
// goal 取 text、其余取 summary。结构：根→目标(spawns)→意图(spawns)→事实(yields)，
// 事实/意图派生新意图，提示→意图(derived_from)。
const P = (o: Record<string, string>) => JSON.stringify(o);

export const explorationGraph: { nodes: TaskNode[]; edges: Edge[] } = {
  nodes: [
    {
      id: "root",
      type: "fact",
      payload: P({ summary: "根：acme.com 攻击面采集与持续监控" }),
      priority: 0,
      state: "origin",
      origin: "system",
      ts: T("2026-07-24T09:12:00Z"),
    },

    // ── 目标（由 goals agent 拆解）──
    {
      id: "g1",
      type: "goal",
      payload: P({ text: "采集 acme.com 全部子域名与解析记录" }),
      priority: 9,
      state: "met",
      origin: "goals",
      ts: T("2026-07-24T09:13:00Z"),
    },
    {
      id: "g2",
      type: "goal",
      payload: P({ text: "采集对外 Web 服务与端点清单" }),
      priority: 8,
      state: "open",
      origin: "goals",
      ts: T("2026-07-24T09:13:00Z"),
    },
    {
      id: "g3",
      type: "goal",
      payload: P({ text: "建立每日快照与变更监控基线" }),
      priority: 8,
      state: "open",
      origin: "goals",
      ts: T("2026-07-24T09:13:00Z"),
    },

    // ── 第 1 层 · 子域与解析 ──
    {
      id: "i1",
      type: "intent",
      payload: P({ summary: "子域枚举：证书透明 + 通用字典" }),
      priority: 6,
      state: "done",
      origin: "planner",
      ts: T("2026-07-24T09:20:00Z"),
    },
    {
      id: "i2",
      type: "intent",
      payload: P({ summary: "子域存活解析与 C 段归属核对" }),
      priority: 7,
      state: "done",
      origin: "planner",
      ts: T("2026-07-24T10:05:00Z"),
    },
    {
      id: "i3",
      type: "intent",
      payload: P({ summary: "端口与服务指纹采集" }),
      priority: 7,
      state: "done",
      origin: "planner",
      ts: T("2026-07-24T11:10:00Z"),
    },
    {
      id: "i4",
      type: "intent",
      payload: P({ summary: "站点爬取：www / admin / api 端点" }),
      priority: 8,
      state: "running",
      origin: "planner",
      ts: T("2026-07-25T09:30:00Z"),
    },
    {
      id: "i5",
      type: "intent",
      payload: P({ summary: "企业备案与证书归属核对" }),
      priority: 7,
      state: "open",
      origin: "planner",
      ts: T("2026-07-25T15:00:00Z"),
    },
    {
      id: "i6",
      type: "intent",
      payload: P({ summary: "端点参数归一化与去重" }),
      priority: 6,
      state: "running",
      origin: "planner",
      ts: T("2026-07-26T02:30:00Z"),
    },
    {
      id: "i7",
      type: "intent",
      payload: P({ summary: "生成首轮资产快照（diff 基线）" }),
      priority: 8,
      state: "open",
      origin: "planner",
      ts: T("2026-07-26T03:10:00Z"),
    },
    {
      id: "i8",
      type: "intent",
      payload: P({ summary: "配置每日快照与变更汇总" }),
      priority: 7,
      state: "open",
      origin: "planner",
      ts: T("2026-07-26T03:20:00Z"),
    },

    // ── 事实 ──
    {
      id: "fa1",
      type: "fact",
      payload: P({ summary: "证书透明 + 字典共发现 42 个子域，本轮新增 18 个" }),
      priority: 0,
      state: "open",
      origin: "work#1",
      ts: T("2026-07-24T09:58:00Z"),
    },
    {
      id: "fa2",
      type: "fact",
      payload: P({ summary: "子域解析到 3 个 C 段，归属 Acme 与云厂商" }),
      priority: 0,
      state: "open",
      origin: "work#2",
      ts: T("2026-07-24T10:32:00Z"),
    },
    {
      id: "fa3",
      type: "fact",
      payload: P({ summary: "开放 80/443/8080/8443，服务指纹 nginx / tomcat" }),
      priority: 0,
      state: "open",
      origin: "work#3",
      ts: T("2026-07-24T11:40:00Z"),
    },
    {
      id: "fa4",
      type: "fact",
      payload: P({ summary: "www 窹取到 316 个端点，含 87 个参数位" }),
      priority: 0,
      state: "open",
      origin: "work#4",
      ts: T("2026-07-25T10:20:00Z"),
    },
    {
      id: "fa5",
      type: "fact",
      payload: P({ summary: "admin 后台为 Vue2 + Spring Boot，存在 /api/v1 接口前缀" }),
      priority: 0,
      state: "open",
      origin: "work#4",
      ts: T("2026-07-25T10:40:00Z"),
    },
    {
      id: "fa6",
      type: "fact",
      payload: P({ summary: "端点参数归一化后去重为 208 个" }),
      priority: 0,
      state: "open",
      origin: "work#5",
      ts: T("2026-07-26T02:55:00Z"),
    },

    // ── 提示（主 agent 注入）──
    {
      id: "h1",
      type: "hint",
      payload: P({ summary: "优先用证书透明日志，历史子域命中率高" }),
      priority: 5,
      state: "consumed",
      origin: "mainagent",
      ts: T("2026-07-24T09:18:00Z"),
    },
    {
      id: "h2",
      type: "hint",
      payload: P({ summary: "把 8080/8443 端口纳入每轮端口采集" }),
      priority: 6,
      state: "consumed",
      origin: "mainagent",
      ts: T("2026-07-25T11:00:00Z"),
    },

    // ── 摘要节点 ──
    {
      id: "d1",
      type: "digest",
      payload: P({ summary: "首轮采集：资产 238 / 子域 42 / 端点 316" }),
      priority: 3,
      state: "done",
      origin: "planner",
      ts: T("2026-07-26T03:30:00Z"),
    },
  ],
  edges: [
    { src: "root", dst: "g1", rel: "spawns" },
    { src: "root", dst: "g2", rel: "spawns" },
    { src: "root", dst: "g3", rel: "spawns" },
    { src: "g1", dst: "i1", rel: "spawns" },
    { src: "g1", dst: "i2", rel: "spawns" },
    { src: "g2", dst: "i3", rel: "spawns" },
    { src: "g2", dst: "i4", rel: "spawns" },
    { src: "g2", dst: "i6", rel: "spawns" },
    { src: "g3", dst: "i7", rel: "spawns" },
    { src: "g3", dst: "i8", rel: "spawns" },
    { src: "g1", dst: "i5", rel: "spawns" },
    { src: "i1", dst: "fa1", rel: "yields" },
    { src: "i2", dst: "fa2", rel: "yields" },
    { src: "i3", dst: "fa3", rel: "yields" },
    { src: "i4", dst: "fa4", rel: "yields" },
    { src: "i4", dst: "fa5", rel: "yields" },
    { src: "i6", dst: "fa6", rel: "yields" },
    { src: "fa2", dst: "i3", rel: "derived_from" },
    { src: "fa3", dst: "i4", rel: "derived_from" },
    { src: "fa4", dst: "i6", rel: "derived_from" },
    { src: "h1", dst: "i1", rel: "derived_from" },
    { src: "h2", dst: "i3", rel: "derived_from" },
    { src: "fa1", dst: "g1", rel: "proves" },
    { src: "fa6", dst: "d1", rel: "covers" },
  ],
};

export const intents: TaskNode[] = explorationGraph.nodes.filter((n) => n.type === "intent");
export const frontier: TaskNode[] = explorationGraph.nodes.filter(
  (n) => n.type === "intent" && (n.state === "open" || n.state === "running"),
);

// ── Activity（执行过程）──────────────────────────────────────────────
// 会话由 sessions-tab 从「意图」派生（session.id = intent.id），transcript = 匹配
// intent_id 的活动流。
export const activity: Activity[] = [
  { seq: 1000, worker: "planner", ts: T("2026-07-24T09:12:10Z"), kind: "round", summary: "第 0 轮 · 目标拆解" },
  {
    seq: 1001,
    worker: "planner",
    ts: T("2026-07-24T09:12:25Z"),
    kind: "thinking",
    summary:
      "graph_overview：图基本空白，采集范围为 acme.com 与 198.51.100.0/24。先把目标拆成子域/Web 服务/持续监控三条线，再按域名采集顺序依次派意图。",
  },
  {
    seq: 1002,
    worker: "planner",
    ts: T("2026-07-24T09:12:40Z"),
    kind: "tool_use",
    tool: "add_goal",
    tool_use_id: "p0-ag",
    summary: "3 goals created → ids=[g1, g2, g3]",
  },
  {
    seq: 1003,
    worker: "planner",
    ts: T("2026-07-24T09:13:05Z"),
    kind: "tool_use",
    tool: "add_intent",
    tool_use_id: "p0-ai",
    summary: "2 intents created → ids=[i1, i2]",
  },
  {
    seq: 1100,
    worker: "planner",
    ts: T("2026-07-24T09:20:10Z"),
    kind: "tool_use",
    tool: "add_intent",
    tool_use_id: "p1-ai",
    summary: "1 intent created → ids=[i3]",
  },
  {
    seq: 1101,
    worker: "planner",
    ts: T("2026-07-24T12:10:00Z"),
    kind: "text",
    summary: "子域与端口基线已完成，把采集重心转向 Web 端点爬取，同时让 i5 核对备案归属。",
  },
  {
    seq: 1102,
    worker: "planner",
    ts: T("2026-07-24T12:10:20Z"),
    kind: "tool_use",
    tool: "add_intent",
    tool_use_id: "p1-ai2",
    summary: "3 intents created → ids=[i4, i5, i6]",
  },
  { seq: 2000, worker: "work#1", intent_id: "i1", ts: T("2026-07-24T09:21:00Z"), kind: "intent", summary: "子域枚举：证书透明 + 通用字典" },
  {
    seq: 2001,
    worker: "work#1",
    intent_id: "i1",
    ts: T("2026-07-24T09:22:10Z"),
    kind: "tool_use",
    tool: "bash",
    tool_use_id: "w1-1",
    summary: "subfinder -d acme.com -all -silent",
  },
  {
    seq: 2002,
    worker: "work#1",
    intent_id: "i1",
    ts: T("2026-07-24T09:40:00Z"),
    kind: "tool_result",
    tool: "bash",
    tool_use_id: "w1-1",
    summary: "收到 42 个子域名，去重后 18 个为本轮新增",
  },
  {
    seq: 2003,
    worker: "work#1",
    intent_id: "i1",
    ts: T("2026-07-24T09:41:00Z"),
    kind: "tool_use",
    tool: "insert_assets",
    tool_use_id: "w1-2",
    summary: "写入 42 个 subdomain 资产",
  },
  {
    seq: 2004,
    worker: "work#1",
    intent_id: "i1",
    ts: T("2026-07-24T09:42:00Z"),
    kind: "result",
    summary: "子域枚举完成，共 42 个（新增 18）",
    input_tokens: 18240,
    output_tokens: 1320,
    cache_read_tokens: 9200,
    cache_write_tokens: 1800,
  },
  { seq: 2100, worker: "work#2", intent_id: "i2", ts: T("2026-07-24T10:06:00Z"), kind: "intent", summary: "子域存活解析与 C 段归属核对" },
  {
    seq: 2101,
    worker: "work#2",
    intent_id: "i2",
    ts: T("2026-07-24T10:07:00Z"),
    kind: "tool_use",
    tool: "bash",
    tool_use_id: "w2-1",
    summary: "dnsx -l subs.txt -a -resp",
  },
  {
    seq: 2102,
    worker: "work#2",
    intent_id: "i2",
    ts: T("2026-07-24T10:31:00Z"),
    kind: "tool_result",
    tool: "bash",
    tool_use_id: "w2-1",
    summary: "解析出 31 个 A 记录，汇总为 3 个 C 段",
  },
  {
    seq: 2103,
    worker: "work#2",
    intent_id: "i2",
    ts: T("2026-07-24T10:32:00Z"),
    kind: "result",
    summary: "归属核对完成：3 个 C 段（Acme 自有 + 云厂商）",
    input_tokens: 15400,
    output_tokens: 980,
    cache_read_tokens: 7600,
    cache_write_tokens: 1200,
  },
  { seq: 2200, worker: "work#3", intent_id: "i3", ts: T("2026-07-24T11:11:00Z"), kind: "intent", summary: "端口与服务指纹采集" },
  {
    seq: 2201,
    worker: "work#3",
    intent_id: "i3",
    ts: T("2026-07-24T11:12:00Z"),
    kind: "tool_use",
    tool: "bash",
    tool_use_id: "w3-1",
    summary: "naabu -list ips.txt -top-ports 1000 -silent",
  },
  {
    seq: 2202,
    worker: "work#3",
    intent_id: "i3",
    ts: T("2026-07-24T11:38:00Z"),
    kind: "tool_result",
    tool: "bash",
    tool_use_id: "w3-1",
    summary: "80/443/8080/8443 开放，服务指纹 nginx / tomcat",
  },
  {
    seq: 2203,
    worker: "work#3",
    intent_id: "i3",
    ts: T("2026-07-24T11:40:00Z"),
    kind: "result",
    summary: "4 个端口与服务指纹已入库",
    input_tokens: 22100,
    output_tokens: 1610,
    cache_read_tokens: 12100,
    cache_write_tokens: 2400,
  },
  { seq: 2300, worker: "work#4", intent_id: "i4", ts: T("2026-07-25T09:31:00Z"), kind: "intent", summary: "站点爬取：www / admin / api 端点" },
  {
    seq: 2301,
    worker: "work#4",
    intent_id: "i4",
    ts: T("2026-07-25T09:32:00Z"),
    kind: "tool_use",
    tool: "bash",
    tool_use_id: "w4-1",
    summary: "katana -u https://www.acme.com -jc -d 3 -silent",
  },
  {
    seq: 2302,
    worker: "work#4",
    intent_id: "i4",
    ts: T("2026-07-25T10:19:00Z"),
    kind: "tool_result",
    tool: "bash",
    tool_use_id: "w4-1",
    summary: "www 爬取 316 个端点，含 87 个参数位",
  },
  {
    seq: 2303,
    worker: "work#4",
    intent_id: "i4",
    ts: T("2026-07-25T10:40:00Z"),
    kind: "result",
    summary: "www/admin/api 端点已写入，后台技术栈为 Vue2 + Spring Boot",
    input_tokens: 38600,
    output_tokens: 2410,
    cache_read_tokens: 20100,
    cache_write_tokens: 3800,
  },
  { seq: 2400, worker: "work#5", intent_id: "i6", ts: T("2026-07-26T02:31:00Z"), kind: "intent", summary: "端点参数归一化与去重" },
  {
    seq: 2401,
    worker: "work#5",
    intent_id: "i6",
    ts: T("2026-07-26T02:32:00Z"),
    kind: "tool_use",
    tool: "bash",
    tool_use_id: "w5-1",
    summary: "sort -u endpoints.txt > normalized.txt",
  },
  {
    seq: 2402,
    worker: "work#5",
    intent_id: "i6",
    ts: T("2026-07-26T02:54:00Z"),
    kind: "result",
    summary: "端点去重后 208 个，已同步到资产库",
    input_tokens: 12800,
    output_tokens: 760,
    cache_read_tokens: 6100,
    cache_write_tokens: 900,
  },
  {
    seq: 3000,
    worker: "mainagent",
    intent_id: "i1",
    ts: T("2026-07-24T09:18:00Z"),
    kind: "user",
    summary: "优先用证书透明日志，历史子域命中率高",
  },
  {
    seq: 3001,
    worker: "mainagent",
    intent_id: "i3",
    ts: T("2026-07-24T11:20:00Z"),
    kind: "user",
    summary: "把 8080/8443 端口纳入每轮端口采集",
  },
];

// ── Token stats ──────────────────────────────────────────────────────────────
export const tokenWorkers: TokenUsage[] = [
  {
    worker: "planner",
    input_tokens: 320000,
    output_tokens: 24000,
    cache_read_tokens: 210000,
    cache_write_tokens: 33000,
  },
  {
    worker: "work#1",
    input_tokens: 288000,
    output_tokens: 21000,
    cache_read_tokens: 190000,
    cache_write_tokens: 28000,
  },
  {
    worker: "work#2",
    input_tokens: 402000,
    output_tokens: 31000,
    cache_read_tokens: 280000,
    cache_write_tokens: 41000,
  },
  {
    worker: "work#3",
    input_tokens: 274500,
    output_tokens: 20320,
    cache_read_tokens: 210400,
    cache_write_tokens: 30000,
  },
  { worker: "workG", input_tokens: 88000, output_tokens: 5200, cache_read_tokens: 60000, cache_write_tokens: 8000 },
  { worker: "work#6", input_tokens: 64000, output_tokens: 4100, cache_read_tokens: 42000, cache_write_tokens: 6000 },
  { worker: "work#7", input_tokens: 132000, output_tokens: 8600, cache_read_tokens: 90000, cache_write_tokens: 12000 },
  { worker: "work#8", input_tokens: 74000, output_tokens: 4700, cache_read_tokens: 50000, cache_write_tokens: 7000 },
  { worker: "work#9", input_tokens: 82000, output_tokens: 5100, cache_read_tokens: 56000, cache_write_tokens: 8000 },
  { worker: "work#10", input_tokens: 46000, output_tokens: 3200, cache_read_tokens: 30000, cache_write_tokens: 4500 },
  { worker: "work#11", input_tokens: 96000, output_tokens: 6200, cache_read_tokens: 64000, cache_write_tokens: 9500 },
  { worker: "work#12", input_tokens: 108000, output_tokens: 6800, cache_read_tokens: 72000, cache_write_tokens: 10000 },
  { worker: "work#13", input_tokens: 144000, output_tokens: 9200, cache_read_tokens: 96000, cache_write_tokens: 14000 },
];

// Mirrors TokenStatsBySession: completed usage is keyed by the stable UI session
// (main | plan | intent:<id>) instead of the reusable work#N executor name.
export const tokenSessions: SessionTokenUsage[] = (() => {
  const totals = new Map<string, SessionTokenUsage>();
  for (const item of activity) {
    if (item.kind !== "result") continue;
    let session = "";
    if (item.worker === "mainagent") session = "main";
    else if (item.worker === "planner") session = "plan";
    else if (item.intent_id) session = `intent:${item.intent_id}`;
    if (!session) continue;
    const current = totals.get(session) ?? {
      session,
      input_tokens: 0,
      output_tokens: 0,
      cache_read_tokens: 0,
      cache_write_tokens: 0,
    };
    current.input_tokens += item.input_tokens ?? 0;
    current.output_tokens += item.output_tokens ?? 0;
    current.cache_read_tokens += item.cache_read_tokens ?? 0;
    current.cache_write_tokens += item.cache_write_tokens ?? 0;
    totals.set(session, current);
  }
  return [...totals.values()];
})();

export const tokenTotal: TokenTotal = grandTotal;

export const dailyTokens: DailyTokenBucket[] = Array.from({ length: 30 }, (_, i) => {
  const d = new Date("2026-07-26T00:00:00Z");
  d.setUTCDate(d.getUTCDate() - (29 - i));
  const wave = 40000 + Math.round(30000 * Math.abs(Math.sin(i / 3)));
  return {
    date: d.toISOString().slice(0, 10),
    input_tokens: wave,
    output_tokens: Math.round(wave * 0.08),
    cache_read_tokens: Math.round(wave * 0.7),
    cache_write_tokens: Math.round(wave * 0.12),
  };
});

export const convTokens: ConvTokenSummary[] = [
  {
    llm_profile_id: 1,
    created_at: T("2026-07-25T10:00:00Z"),
    input_tokens: 42000,
    output_tokens: 3800,
    cache_read_tokens: 26000,
    cache_write_tokens: 5200,
  },
  {
    llm_profile_id: 2,
    created_at: T("2026-07-24T16:00:00Z"),
    input_tokens: 18000,
    output_tokens: 1600,
    cache_read_tokens: 9000,
    cache_write_tokens: 2100,
  },
];

// ── Traffic ──────────────────────────────────────────────────────────────────
const exchanges = [
  ["x-1", "www.acme.com", "GET", "https://www.acme.com/", 200, "text/html", 48213],
  ["x-2", "www.acme.com", "GET", "https://www.acme.com/search?q=test", 200, "text/html", 5120],
  ["x-3", "www.acme.com", "GET", "https://www.acme.com/.git/HEAD", 200, "text/plain", 23],
  ["x-4", "admin.acme.com", "GET", "https://admin.acme.com/login", 200, "text/html", 12044],
  ["x-5", "admin.acme.com", "POST", "https://admin.acme.com/login", 302, "text/html", 0],
  ["x-6", "admin.acme.com", "GET", "https://admin.acme.com/dashboard", 200, "text/html", 33120],
  ["x-7", "admin.acme.com", "GET", "https://admin.acme.com/api/users?page=1", 200, "application/json", 8842],
  ["x-8", "api.acme.com", "GET", "https://api.acme.com/v1/orders?id=1001", 200, "application/json", 640],
  ["x-9", "api.acme.com", "GET", "https://api.acme.com/v1/orders?id=1002", 200, "application/json", 655],
  ["x-10", "api.acme.com", "GET", "https://api.acme.com/v1/users/me", 200, "application/json", 312],
  ["x-11", "api.acme.com", "GET", "https://api.acme.com/v1/admin", 401, "application/json", 45],
  ["x-12", "shop.acme.com", "GET", "https://shop.acme.com/checkout", 200, "text/html", 21990],
  ["x-13", "shop.acme.com", "POST", "https://shop.acme.com/pay", 200, "application/json", 128],
  ["x-14", "www.acme.com", "GET", "https://www.acme.com/robots.txt", 200, "text/plain", 88],
  ["x-15", "shop.acme.com", "POST", "https://shop.acme.com/api/import", 500, "application/json", 2048],
  ["x-16", "shop.acme.com", "POST", "https://shop.acme.com/api/import", 200, "application/json", 0],
  ["x-17", "10.10.10.20", "GET", "http://10.10.10.20:8080/", 200, "text/html", 14200],
  ["x-18", "10.10.10.20", "GET", "http://10.10.10.20:8080/api/json", 200, "application/json", 86],
  ["x-19", "10.10.10.10", "GET", "https://10.10.10.10/", 200, "text/html", 512],
] as const;

export const traffic: TrafficResp = {
  enabled: true,
  proxy: ":8788",
  count: exchanges.length,
  total: exchanges.length,
  page: 0,
  size: 100,
  exchanges: exchanges.map(([id, host, method, url, status, ct, len], i) => ({
    id,
    host,
    method,
    url,
    status,
    content_type: ct,
    resp_len: len,
    ts: T(`2026-07-26T0${1 + (i % 3)}:${String(10 + i).padStart(2, "0")}:00Z`),
  })),
};

// Distinct hosts with counts, derived from the exchanges above (target picker).
const hostCounts: Record<string, number> = {};
for (const [, host] of exchanges) hostCounts[host] = (hostCounts[host] ?? 0) + 1;
export const trafficHosts: TrafficHost[] = Object.entries(hostCounts).map(([host, count]) => ({ host, count }));

export const trafficDetail: TrafficDetail = {
  req: `GET /v1/orders?id=1002 HTTP/1.1
Host: api.acme.com
Authorization: Bearer eyJhbGciOiJIUzI1NiJ9...
User-Agent: hawkeye-worker/1.0
Accept: application/json

`,
  resp: `HTTP/1.1 200 OK
Content-Type: application/json
Content-Length: 655

{
  "order_id": 1002,
  "user_id": 77,
  "amount": 1299.00,
  "address": "北京市朝阳区 ****",
  "phone": "138****6021",
  "items": [{ "sku": "A-100", "qty": 1 }]
}`,
};

// ── Security / Audit ─────────────────────────────────────────────────────────
export const audit: Audit = {
  attributions: { allow: 312, block: 9 },
  entries: [
    {
      ts: T("2026-07-26T03:33:00Z"),
      tool: "bash",
      action: "allow",
      command: "curl -s -d 'username=admin...' https://admin.acme.com/login",
    },
    {
      ts: T("2026-07-26T03:57:00Z"),
      tool: "bash",
      action: "allow",
      command: "curl -s -b 'session=...' https://admin.acme.com/api/users",
    },
    {
      ts: T("2026-07-26T02:10:00Z"),
      tool: "bash",
      action: "block",
      reason: "目标越界：out.evil.example 不在 scope 内",
      command: "curl https://out.evil.example/exfil",
    },
    {
      ts: T("2026-07-25T22:03:00Z"),
      tool: "bash",
      action: "allow",
      command: "sqlmap -u 'https://www.acme.com/search?q=1' --batch",
    },
    {
      ts: T("2026-07-25T21:00:00Z"),
      tool: "bash",
      action: "block",
      reason: "破坏性命令门控：rm -rf 被拒绝",
      command: "rm -rf /var/www",
    },
    {
      ts: T("2026-07-25T20:30:00Z"),
      tool: "bash",
      action: "allow",
      command: "git-dumper https://www.acme.com/.git/ /tmp/acme",
    },
    {
      ts: T("2026-07-25T15:20:00Z"),
      tool: "bash",
      action: "allow",
      command:
        "curl -s -H 'Content-Type: application/json' -d '{\"@type\":\"com.sun.rowset.JdbcRowSetImpl\"...}' https://shop.acme.com/api/import",
    },
    {
      ts: T("2026-07-25T21:40:00Z"),
      tool: "bash",
      action: "allow",
      command: "curl -s http://10.10.10.20:8080/api/json",
    },
    {
      ts: T("2026-07-26T00:00:00Z"),
      tool: "bash",
      action: "allow",
      command: "curl -sI https://10.10.10.10/",
    },
    {
      ts: T("2026-07-26T00:05:00Z"),
      tool: "bash",
      action: "block",
      reason: "破坏性操作门控：删除生产数据需人工审批",
      command: "rm -rf /srv/data",
    },
    {
      ts: T("2026-07-26T00:06:00Z"),
      tool: "bash",
      action: "allow",
      command: "nmap -sV -p 80,443,8080 10.10.10.20  # 只读服务指纹",
    },
  ],
};

// ── LLM profiles ─────────────────────────────────────────────────────────────
export const llmProfiles: LLMProfile[] = [
  {
    id: "1",
    name: "Claude Opus 4.8",
    format: "anthropic",
    model: "claude-opus-4-8",
    api_key_hint: "…a3f2",
    rate_per_second: 0,
    rate_per_minute: 0,
    context_window_k: 1000,
    thinking_type: "enabled",
    reasoning_effort: "high",
    is_default: true,
    priority: 0,
    pool_exclude: false,
    // anthropic 的字段名固定，故 max_tokens_field 恒为空。
    max_tokens: 0,
    max_tokens_field: "",
  },
  {
    id: "2",
    name: "DeepSeek V4",
    format: "openai",
    base_url: "https://api.deepseek.com",
    model: "deepseek-v4-flash",
    api_key_hint: "…9c11",
    rate_per_second: 0,
    rate_per_minute: 60,
    context_window_k: 128,
    thinking_type: "",
    reasoning_effort: "",
    is_default: false,
    priority: 10,
    pool_exclude: false,
    // openai 格式 + 推理模型：上限走 max_completion_tokens。
    max_tokens: 8192,
    max_tokens_field: "max_completion_tokens",
  },
];

export const llmConfig = {
  configured: true,
  provider: "anthropic",
  model: "claude-opus-4-8",
  base_url: "",
  proxy: "",
  key_set: true,
  rate_per_second: 0,
  rate_per_minute: 0,
  context_window_k: 1000,
  thinking_type: "enabled",
  reasoning_effort: "high",
};

// ── Agents ───────────────────────────────────────────────────────────────────
export const agents: Agent[] = [
  {
    id: "1",
    key: "goals",
    name: "目标拆解器",
    role: "goals",
    builtin: true,
    enabled: true,
    description: "把任务目标拆成可探索的子目标",
    max_turns: 8,
    mcp_count: 0,
    skill_count: 1,
    tool_count: 3,
  },
  {
    id: "2",
    key: "planner",
    name: "规划者",
    role: "planner",
    builtin: true,
    enabled: true,
    description: "读探索路线、判断目标达成、生成意图",
    max_turns: 0,
    mcp_count: 1,
    skill_count: 2,
    tool_count: 9,
  },
  {
    id: "3",
    key: "mainagent",
    name: "主 Agent",
    role: "mainagent",
    builtin: true,
    enabled: true,
    description: "人在环路对话，注入 hint / 高优意图",
    max_turns: 0,
    web_search: true,
    mcp_count: 2,
    skill_count: 3,
    tool_count: 11,
  },
  {
    id: "4",
    key: "worker",
    name: "Work Agent",
    role: "worker",
    builtin: true,
    enabled: true,
    description: "claim 意图 → 跑 Kali 工具 → 图写回",
    max_turns: 40,
    run_seconds: 1800,
    web_search: false,
    interactive_shell: true,
    mcp_count: 2,
    skill_count: 3,
    tool_count: 14,
  },
  {
    id: "5",
    key: "recon-bot",
    name: "资产测绘机器人（自定义）",
    role: "custom",
    builtin: false,
    enabled: true,
    description: "定时对新资产做被动测绘",
    max_turns: 12,
    mcp_count: 1,
    skill_count: 1,
    tool_count: 4,
  },
];

const promptVars: PromptVar[] = [
  { name: "Goal", description: "当前任务目标", example: "采集 acme.com 的全部子域与 Web 端点", source: "runtime" },
  { name: "AssetSummary", description: "资产图概览", example: "6 子域 / 3 IP / 4 应用 / 4 端点", source: "distilled" },
  { name: "RouteHint", description: "采集路线提示", example: "优先覆盖新注册子域与隐藏端点", source: "exploration" },
];

const promptVersions: PromptVersion[] = [
  { version: 3, ts: T("2026-07-25T10:00:00Z"), note: "强化增量采集引导", template_text: "你是 Hawkeye 的规划者……" },
  { version: 2, ts: T("2026-07-20T10:00:00Z"), note: "初版微调", template_text: "你是 Hawkeye 的规划者(v2)……" },
];

export function agentDetail(key: string): AgentDetail {
  const a = agents.find((x) => x.key === key) ?? agents[1];
  return {
    agent: a,
    prompt: `你是 Hawkeye 的「${a.name}」。\n目标：{{.Goal}}\n资产概览：{{.AssetSummary}}\n采集提示：{{.RouteHint}}\n请基于以上信息推进采集，并通过工具把结果写回图。`,
    variables: promptVars,
    versions: promptVersions,
    visibility: { mcp: [1, 2], skill: ["api-recon", "playwright-cli"] },
    wrapup_prompt: "",
    wrapup_default: "时间/步数将尽，请总结已确认发现并标记意图终态。",
    wrapup_max_turns: 0,
    wrapup_max_turns_default: 3,
    task_timeout_wrapup_supported: a.key === "worker" || a.key === "planner",
    task_timeout_wrapup_default: "任务超时，请立即收尾并落库当前结论。",
    task_timeout_wrapup_max_turns_default: 2,
  };
}

// ── MCP ──────────────────────────────────────────────────────────────────────
export const mcpServers: MCPServer[] = [
  {
    id: 1,
    name: "shodan",
    transport: "http",
    url: "https://mcp.shodan.io/sse",
    args: [],
    env: {},
    enabled: true,
    tools: ["host_info", "search", "dns_resolve"],
  },
  {
    id: 2,
    name: "playwright",
    transport: "stdio",
    command: "npx",
    args: ["-y", "@playwright/mcp"],
    env: {},
    enabled: true,
    tools: ["browser_navigate", "browser_click", "browser_snapshot"],
  },
];

export const mcpToolsById: Record<number, MCPTool[]> = {
  1: [
    { name: "host_info", description: "查询某 IP 的 Shodan 主机信息" },
    { name: "search", description: "Shodan 搜索" },
    { name: "dns_resolve", description: "DNS 解析" },
  ],
  2: [
    { name: "browser_navigate", description: "浏览器打开 URL" },
    { name: "browser_click", description: "点击元素" },
    { name: "browser_snapshot", description: "抓取可访问性快照" },
  ],
};

// ── Skills ───────────────────────────────────────────────────────────────────
export const skills: SkillItem[] = [
  {
    name: "api-recon",
    description: "对 REST/GraphQL API 做接口与参数面采集；发现新 API 端点时使用。",
    license: "MIT",
    mcps: [],
    files: ["SKILL.md", "scripts/enum.py"],
    calls: 24,
    tasks: 6,
    usage_agents: ["worker", "planner"],
    last_used: "2026-08-20T09:12:00Z",
  },
  {
    name: "playwright-cli",
    description: "用 Playwright 驱动浏览器做动态爬取与截图；需要渲染 JS 站点时使用。",
    mcps: ["playwright"],
    files: ["SKILL.md"],
    calls: 7,
    tasks: 3,
    usage_agents: ["worker"],
    last_used: "2026-08-19T21:40:00Z",
  },
  {
    name: "scopesentry",
    description: "从 ScopeSentry 拉取资产并归并到公司范围；批量导入资产时使用。",
    files: ["SKILL.md", "assets/mapping.md"],
    calls: 0,
    tasks: 0,
    usage_agents: [],
  },
];

export const skillCalls: SkillCall[] = [
  { ts: "2026-08-20T09:12:00Z", agent_key: "worker", task_id: 42, session_id: "", args_len: 128 },
  { ts: "2026-08-20T08:03:00Z", agent_key: "planner", task_id: 42, session_id: "", args_len: 0 },
  { ts: "2026-08-19T17:55:00Z", agent_key: "worker", task_id: 37, session_id: "", args_len: 64 },
];

export const missingSkills: MissingSkill[] = [
  { skill: "jwt-forge", calls: 3, agents: ["worker"], last_used: "2026-08-20T07:20:00Z" },
];

// ── Tools ────────────────────────────────────────────────────────────────────
export const tools: Tool[] = [
  {
    key: "bash",
    system: true,
    description: "在 Kali 环境执行 shell 命令（受 scope/破坏性门控）",
    schema: { type: "object", properties: { command: { type: "string" } }, required: ["command"] },
    agents: ["worker", "mainagent"],
    enabled: true,
    kind: "builtin",
    calls: 1842,
  },
  {
    key: "upsert_asset",
    system: true,
    description: "写入/更新资产节点",
    schema: { type: "object", properties: { type: { type: "string" }, value: { type: "string" } } },
    agents: ["worker"],
    enabled: true,
    kind: "builtin",
    calls: 417,
  },
  {
    key: "list_goals",
    system: true,
    description: "列出当前任务的目标与达成状态",
    schema: { type: "object", properties: {} },
    agents: ["planner"],
    enabled: true,
    kind: "builtin",
    calls: 238,
  },
  {
    key: "subfinder",
    system: false,
    description: "自定义：用 subfinder 枚举子域名",
    schema: {
      type: "object",
      properties: { target: { type: "string" } },
      required: ["target"],
    },
    agents: ["worker"],
    enabled: true,
    kind: "command",
    exec: { command: "subfinder", args: ["-d", "{{target}}"] },
    calls: 31,
  },
];

// ── Settings ─────────────────────────────────────────────────────────────────
export const settings: Settings = {
  traffic_capture: true,
  llm_record: false,
  web_search_enabled: true,
  web_search_backend: "ddgs",
  brave_key_set: false,
  tavily_key_set: true,
  web_search_proxy: "",
  global_proxy: "",
  python_interpreter: "/usr/bin/python3",
  workers: 3,
  llm_pool_enabled: true,
  llm_pool_bind_fallback: false,
  noa_compaction: false,
};

// ── LLM 轮询（故障转移）──────────────────────────────────────────────────────
// demo：激活配置正常，备用配置刚因余额不足熔断，正在冷却。
export const llmPool: LLMPoolStatus = {
  enabled: true,
  bind_fallback: false,
  chain: [
    {
      profile_id: "1",
      name: "Claude Opus 4.8",
      model: "claude-opus-4-8",
      format: "anthropic",
      priority: 0,
      active: true,
      excluded: false,
      state: "ok",
      fails: 0,
      trips: 0,
      cooldown_secs: 0,
    },
    {
      profile_id: "2",
      name: "DeepSeek V4",
      model: "deepseek-v4-flash",
      format: "openai",
      priority: 10,
      active: false,
      excluded: false,
      state: "tripped",
      fails: 0,
      trips: 1,
      cooldown_secs: 42,
      last_error: "openai: status 402: insufficient balance",
      last_at: T("2026-07-26T00:10:00Z"),
    },
  ],
};

// ── Intercept ────────────────────────────────────────────────────────────────
export const interceptRules: InterceptRule[] = [
  {
    id: 1,
    name: "破坏性命令需审批",
    enabled: true,
    priority: 10,
    match_target: "tool_input",
    match_type: "regex",
    pattern: "rm\\s+-rf|mkfs|dd\\s+if=",
    action: "ask",
    message: "检测到破坏性命令，需人工审批",
    timeout_enabled: true,
    timeout_seconds: 120,
    timeout_action: "deny",
    created_at: T("2026-07-20T10:00:00Z"),
    updated_at: T("2026-07-20T10:00:00Z"),
  },
  {
    id: 2,
    name: "外发流量直接拒绝",
    enabled: true,
    priority: 20,
    match_target: "tool_input",
    match_type: "string",
    pattern: "exfil",
    action: "deny",
    message: "疑似外发数据",
    timeout_enabled: false,
    timeout_seconds: 0,
    timeout_action: "deny",
    created_at: T("2026-07-21T10:00:00Z"),
    updated_at: T("2026-07-21T10:00:00Z"),
  },
];

export const interceptPending: InterceptPending[] = [
  {
    id: 101,
    rule_id: 1,
    task_id: "t-acme-web",
    agent_name: "work#3",
    tool_name: "bash",
    tool_input: { command: "mysqldump -h 203.0.113.10 -uroot acme_prod > /tmp/dump.sql" },
    status: "pending",
    reason: "数据外泄门控：整库导出需人工审批",
    created_at: T("2026-07-26T03:58:00Z"),
  },
];

export const interceptHistory: InterceptApprovalRow[] = [
  {
    id: 90,
    rule_id: 1,
    task_id: "t-acme-web",
    agent_name: "work#1",
    tool_name: "bash",
    tool_input: { command: "dd if=/dev/zero of=/tmp/x" },
    status: "denied",
    reason: "破坏性命令需审批",
    created_at: T("2026-07-25T20:00:00Z"),
    decided_at: T("2026-07-25T20:01:00Z"),
    conv_title: "",
    conv_agent_key: "",
    rule_name: "破坏性命令需审批",
  },
  {
    id: 91,
    task_id: "t-acme-api",
    agent_name: "work#2",
    tool_name: "bash",
    tool_input: { command: "rm -rf /var/www/html" },
    status: "denied",
    reason: "[模型] 删除目标生产文件(D4)",
    created_at: T("2026-07-25T18:00:00Z"),
    decided_at: T("2026-07-25T18:00:05Z"),
    conv_title: "",
    conv_agent_key: "",
    rule_name: "",
  },
  {
    id: 92,
    rule_id: 2,
    task_id: "t-acme-web",
    agent_name: "work#13",
    tool_name: "bash",
    tool_input: { command: "nmap -sV -p 80,443,8080 10.10.10.20" },
    status: "allowed",
    reason: "只读采集放行",
    created_at: T("2026-07-26T00:05:00Z"),
    decided_at: T("2026-07-26T00:06:00Z"),
    conv_title: "",
    conv_agent_key: "",
    rule_name: "外发流量直接拒绝",
  },
  {
    id: 93,
    task_id: "t-acme-web",
    agent_name: "work#7",
    tool_name: "bash",
    tool_input: { command: 'mysql -e "DROP TABLE users_bak_0921"' },
    status: "pending",
    reason: "[模型] 疑似备份表,无法确定是否生产数据",
    created_at: T("2026-07-25T15:14:00Z"),
    conv_title: "",
    conv_agent_key: "",
    rule_name: "",
  },
];

// Approval detail fixtures use harmless report-writing examples.
interceptHistory.unshift({
  id: 94,
  task_id: "t-acme-web",
  agent_name: "work#1",
  tool_name: "Write",
  tool_input: { path: "reports/summary.md", content: "# 检查摘要\n\n本轮验证已完成，整理已有证据与后续建议。" },
  status: "allowed",
  decision_source: "model",
  reason: "[模型] 将已有检查结论写入本地报告，不修改业务数据。",
  created_at: T("2026-07-26T08:00:00Z"),
  decided_at: T("2026-07-26T08:00:02Z"),
  conv_title: "",
  conv_agent_key: "",
  rule_name: "",
});

export const interceptDetails: Record<number, InterceptAudit> = {
  94: {
    run_id: "demo-run-report",
    tool_use_id: "call-write-report",
    correlation: "exact",
    input_digest: "a3b458eca3b458eca3b458eca3b458eca3b458eca3b458eca3b458eca3b458ec1234",
    user_message: "请整理已经完成的检查，将结论和证据索引写入 reports/summary.md。\n保留待验证项，不修改业务数据。",
    context: [
      { kind: "user", text: "汇总本轮已有证据，生成检查摘要。" },
      {
        kind: "tool_use",
        tool: "Read",
        tool_use_id: "call-read-evidence",
        text: '{"path":"reports/evidence-index.json"}',
      },
      {
        kind: "tool_result",
        tool: "Read",
        tool_use_id: "call-read-evidence",
        text: "已读取 3 条证据索引。\n记录包含请求时间、结果摘要和本地文件位置。",
      },
    ],
    captured_at: T("2026-07-26T08:00:00Z"),
    initial_action: "allow",
    initial_reason: "[模型] 将已有检查结论写入本地报告，不修改业务数据。",
    effective_action: "allow",
    config_digest: "b4c569fdb4c569fdb4c569fdb4c569fdb4c569fdb4c569fdb4c569fdb4c569fd1234",
    profile_id: 1,
    execution_status: "succeeded",
    output: "Successfully wrote reports/summary.md\n共写入 68 个字符。",
    execution_ended_at: T("2026-07-26T08:00:03Z"),
  },
  93: {
    run_id: "demo-run-review",
    tool_use_id: "call-pending-review",
    correlation: "exact",
    input_digest: "c5d670aec5d670aec5d670aec5d670aec5d670aec5d670aec5d670aec5d670ae1234",
    user_message: "检查备份表的用途，涉及删除时需要先确认。",
    context: [],
    captured_at: T("2026-07-25T15:14:00Z"),
    initial_action: "ask",
    initial_reason: "[模型] 疑似备份表，无法确定是否生产数据。",
    profile_id: 1,
    execution_status: "not_started",
  },
};

// ── Conversations (chat) ─────────────────────────────────────────────────────
export const conversations: Conversation[] = [
  {
    id: 1,
    agent_key: "mainagent",
    title: "acme 后台切入点讨论",
    llm_profile_id: 1,
    pinned: true,
    pinned_at: T("2026-07-26T04:00:00Z"),
    created_at: T("2026-07-25T10:00:00Z"),
    updated_at: T("2026-07-26T03:40:00Z"),
  },
  {
    id: 2,
    agent_key: "mainagent",
    title: "API 接口与参数面梳理",
    llm_profile_id: 2,
    created_at: T("2026-07-24T16:00:00Z"),
    updated_at: T("2026-07-24T17:20:00Z"),
  },
  {
    id: 3,
    agent_key: "mainagent",
    title: "内网资产测绘 · 端口与服务补全",
    llm_profile_id: 1,
    created_at: T("2026-07-25T15:45:00Z"),
    updated_at: T("2026-07-26T00:25:00Z"),
  },
];

export const conversationMessages: Record<number, Activity[]> = {
  1: [
    { seq: 1, worker: "user", ts: T("2026-07-25T10:00:00Z"), kind: "user", summary: "后台在哪，先从哪切入？" },
    {
      seq: 2,
      worker: "mainagent",
      ts: T("2026-07-25T10:00:04Z"),
      kind: "thinking",
      summary: "先确认 admin.acme.com 的登录页技术栈和防护（验证码/速率限制），再决定用默认口令还是字典。",
    },
    {
      seq: 3,
      worker: "mainagent",
      ts: T("2026-07-25T10:00:06Z"),
      kind: "tool_use",
      tool: "bash",
      tool_use_id: "c11",
      summary: "curl -s https://admin.acme.com/login | grep -Eo 'element-ui|captcha|csrf'",
    },
    {
      seq: 4,
      worker: "mainagent",
      ts: T("2026-07-25T10:00:08Z"),
      kind: "tool_result",
      tool: "bash",
      tool_use_id: "c11",
      summary: "element-ui\ncsrf",
      detail: "Element-UI 后台，有 CSRF token 但无验证码。",
    },
    {
      seq: 5,
      worker: "mainagent",
      ts: T("2026-07-25T10:00:20Z"),
      kind: "tool_use",
      tool: "bash",
      tool_use_id: "c12",
      summary:
        "curl -sI https://admin.acme.com/login",
    },
    {
      seq: 6,
      worker: "mainagent",
      ts: T("2026-07-25T10:00:24Z"),
      kind: "tool_result",
      tool: "bash",
      tool_use_id: "c12",
      summary: "HTTP/2 200\\nserver: nginx\\nx-powered-by: Express",
      detail: "登录页可访问；响应头识别到 nginx + Node/Express 技术栈。",
    },
    {
      seq: 7,
      worker: "mainagent",
      ts: T("2026-07-25T10:00:30Z"),
      kind: "text",
      summary:
        "结论：admin.acme.com 是 Element-UI 后台，登录页与静态资源可只读访问。已采集登录入口路径、表单字段名与技术指纹，并把「后台静态资源与接口路径采集」作为高优意图注入给 work agent。",
    },
    { seq: 8, worker: "user", ts: T("2026-07-26T03:40:00Z"), kind: "user", summary: "采集到哪些？" },
    {
      seq: 9,
      worker: "mainagent",
      ts: T("2026-07-26T03:40:05Z"),
      kind: "text",
      summary:
        "已登记：admin.acme.com/login（endpoint）、Element-UI + Express 指纹、静态资源 3 个；并补全 admin.acme.com → 10.10.10.20 的解析关系。下一步继续采集后台接口路径与参数，保持只读。",
    },
  ],
  2: [
    {
      seq: 1,
      worker: "user",
      ts: T("2026-07-24T16:00:00Z"),
      kind: "user",
      summary: "帮我梳理下 api.acme.com 的接口与参数面。",
    },
    {
      seq: 2,
      worker: "mainagent",
      ts: T("2026-07-24T16:00:05Z"),
      kind: "thinking",
      summary: "先摸 API 的入口、鉴权方式与关键端点的响应结构，再补全端点与参数。",
    },
    {
      seq: 3,
      worker: "mainagent",
      ts: T("2026-07-24T16:00:10Z"),
      kind: "tool_use",
      tool: "bash",
      tool_use_id: "c21",
      summary: "curl -s https://api.acme.com/v1/users/me -H 'Authorization: Bearer <self>' | jq '{id,role}'",
    },
    {
      seq: 4,
      worker: "mainagent",
      ts: T("2026-07-24T16:00:12Z"),
      kind: "tool_result",
      tool: "bash",
      tool_use_id: "c21",
      summary: '{ "id": 42, "role": "user" }',
      detail: "接口返回用户对象，含 id/role 字段。",
    },
    {
      seq: 5,
      worker: "mainagent",
      ts: T("2026-07-24T16:00:30Z"),
      kind: "tool_use",
      tool: "bash",
      tool_use_id: "c22",
      summary: "curl -s 'https://api.acme.com/v1/orders?id=1002' | jq 'keys'",
    },
    {
      seq: 6,
      worker: "mainagent",
      ts: T("2026-07-24T16:00:31Z"),
      kind: "tool_result",
      tool: "bash",
      tool_use_id: "c22",
      summary: '["id","status","amount","items"]',
      detail: "订单接口返回结构已采集。",
    },
    {
      seq: 7,
      worker: "mainagent",
      ts: T("2026-07-24T16:01:00Z"),
      kind: "text",
      summary:
        "接口面已梳理：① /v1/orders 使用 id 查询、返回订单字段；② /v1/users/me 返回当前用户对象。已生成两个意图：补全 orders 接口的方法与参数结构，补全 users 相关端点与返回特征。只采集已授权访问到的结构，不做参数枚举。",
    },
  ],
  3: [
    {
      seq: 1,
      worker: "user",
      ts: T("2026-07-25T15:45:00Z"),
      kind: "user",
      summary: "DMZ 主机端口已采到，接下来怎么补内网资产？",
    },
    {
      seq: 2,
      worker: "mainagent",
      ts: T("2026-07-25T15:45:05Z"),
      kind: "thinking",
      summary: "从已采到的 DMZ 主机出发，先用只读方式补全内网网段的存活主机与开放服务。",
    },
    {
      seq: 3,
      worker: "mainagent",
      ts: T("2026-07-25T15:45:10Z"),
      kind: "text",
      summary:
        "先从 DMZ 主机的路由与已采端口出发，做只读的内网存活与端口探测（nmap -sn / -sV），重点补全域控与 CI/CD 服务器的端口、服务与指纹。",
    },
    {
      seq: 4,
      worker: "user",
      ts: T("2026-07-25T21:05:00Z"),
      kind: "user",
      summary: "内网扫到了 3 台，8080 是 Jenkins，要继续采吗？",
    },
    {
      seq: 5,
      worker: "mainagent",
      ts: T("2026-07-25T21:05:08Z"),
      kind: "tool_use",
      tool: "bash",
      tool_use_id: "c31",
      summary: "curl -s -o /dev/null -w '%{http_code}' http://10.10.10.20:8080/api/json",
    },
    {
      seq: 6,
      worker: "mainagent",
      ts: T("2026-07-25T21:05:12Z"),
      kind: "tool_result",
      tool: "bash",
      tool_use_id: "c31",
      summary: "200",
      detail: "Jenkins API 可访问并返回 JSON；首页与 /api/json 已作为端点登记。",
    },
    {
      seq: 7,
      worker: "mainagent",
      ts: T("2026-07-25T21:05:20Z"),
      kind: "text",
      summary:
        "是 Jenkins。已把首页标题、版本与 JSON API 响应特征写入资产属性；再派一条意图补全 Jenkins 相关路径（/api/json、/manage、/job/*）与访问特征。只做只读采集，不触发任何构建或脚本执行。",
    },
    {
      seq: 8,
      worker: "user",
      ts: T("2026-07-25T23:25:00Z"),
      kind: "user",
      summary: "内网那台 DC 的端口也采到了，要继续补吗？",
    },
    {
      seq: 9,
      worker: "mainagent",
      ts: T("2026-07-25T23:25:10Z"),
      kind: "text",
      summary:
        "继续。DC01 的 389/636/445 已登记，下一轮补全 FS01/CI 主机的服务版本与证书信息；所有探测保持只读，遵守授权范围与速率约束。",
    },
    { seq: 10, worker: "user", ts: T("2026-07-26T00:22:00Z"), kind: "user", summary: "采集完成了，资产面总结一下。" },
    {
      seq: 11,
      worker: "mainagent",
      ts: T("2026-07-26T00:22:12Z"),
      kind: "text",
      summary:
        "本轮采集链：\n1. 外部子域扩散 → 发现 admin/shop/api 等 6 个子域\n2. 子域解析 → 3 个 IP，补全 DNS 记录与证书 SAN\n3. 主机端口/服务 → 80/443/8080/445 等服务与版本\n4. 站点指纹 → Nginx/Express/Element-UI 等技术栈\n5. 站点→端点/参数 → orders、users/me、login 等 8 个端点\n6. 内网段存活 → DC01/FS01/Jenkins 的端口与服务\n\n四个目标全部达成，共采集 26 项资产/关系（6 子域 / 3 IP / 9 服务 / 8 端点）。",
    },
  ],
};

// ── 资产测试覆盖度（/tasks/{id}/coverage）──
export const coverage = {
  enabled: true,
  scope_rows: 4,
  denominator: 26,
  tested: 11,
  pct: 11 / 26,
  by_type: [
    { type: "subdomain", total: 6, tested: 4 },
    { type: "service", total: 9, tested: 4 },
    { type: "endpoint", total: 8, tested: 2 },
    { type: "ip", total: 3, tested: 1 },
  ],
};

// ── 资产覆盖图（/tasks/{id}/coverage-graph）──
export const coverageGraph = {
  nodes: [
    { key: "c:1", kind: "company", label: "Acme Corp", tested: false, in_scope: false, company_id: 1 },
    {
      key: "a:1",
      kind: "root_domain",
      label: "acme.com",
      tested: false,
      in_scope: false,
      asset_id: 1,
      domain: "acme.com",
      company_id: 1,
    },
    {
      key: "a:2",
      kind: "subdomain",
      label: "www.acme.com",
      tested: true,
      in_scope: true,
      asset_id: 2,
      domain: "www.acme.com",
      root_domain: "acme.com",
    },
    {
      key: "a:3",
      kind: "subdomain",
      label: "api.acme.com",
      tested: true,
      in_scope: true,
      asset_id: 3,
      domain: "api.acme.com",
      root_domain: "acme.com",
    },
    {
      key: "a:4",
      kind: "subdomain",
      label: "admin.acme.com",
      tested: false,
      in_scope: true,
      asset_id: 4,
      domain: "admin.acme.com",
      root_domain: "acme.com",
    },
    {
      key: "a:5",
      kind: "service",
      label: "www.acme.com",
      tested: true,
      in_scope: true,
      asset_id: 5,
      domain: "www.acme.com",
      url: "https://www.acme.com",
      port: 443,
      page_title: "Acme 首页",
      status_code: 200,
    },
    {
      key: "a:6",
      kind: "endpoint",
      label: "https://www.acme.com/search",
      tested: true,
      in_scope: true,
      asset_id: 6,
      url: "https://www.acme.com/search?q=",
      domain: "www.acme.com",
    },
    {
      key: "a:7",
      kind: "service",
      label: "api.acme.com",
      tested: true,
      in_scope: true,
      asset_id: 7,
      domain: "api.acme.com",
      url: "https://api.acme.com",
      port: 443,
      page_title: "API Gateway",
      status_code: 401,
    },
    {
      key: "a:8",
      kind: "endpoint",
      label: "https://api.acme.com/v1/orders",
      tested: false,
      in_scope: true,
      asset_id: 8,
      url: "https://api.acme.com/v1/orders?id=",
      domain: "api.acme.com",
    },
    {
      key: "a:9",
      kind: "service",
      label: "admin.acme.com",
      tested: false,
      in_scope: true,
      asset_id: 9,
      domain: "admin.acme.com",
      url: "https://admin.acme.com",
      port: 443,
      page_title: "后台登录",
      status_code: 200,
    },
    {
      key: "a:10",
      kind: "ip",
      label: "203.0.113.10",
      tested: true,
      in_scope: true,
      asset_id: 10,
      ip: "203.0.113.10",
      company_id: 1,
    },
  ],
  edges: [
    { src: "a:1", dst: "c:1" },
    { src: "a:2", dst: "a:1" },
    { src: "a:3", dst: "a:1" },
    { src: "a:4", dst: "a:1" },
    { src: "a:5", dst: "a:2" },
    { src: "a:6", dst: "a:5" },
    { src: "a:7", dst: "a:3" },
    { src: "a:8", dst: "a:7" },
    { src: "a:9", dst: "a:4" },
    { src: "a:10", dst: "c:1" },
  ],
};

// ── 资产在本任务关联的意图/事实（/tasks/{id}/asset-refs）──
// 播报板 demo:探索节点 → 其锚定资产。真实后端读 exploration_anchors,mock 里静态给几条。
const NODE_ASSETS: Record<string, number[]> = {
  i1: [2],
  i7: [4],
};

export function nodeAssetsFor(nodeId: string): AssetRef[] {
  return (NODE_ASSETS[nodeId] ?? []).flatMap((id) => {
    const asset = assets.find((candidate) => candidate.id === id);
    return asset ? [assetRef(id)] : [];
  });
}

export function assetRefsFor(_assetId: number) {
  return {
    intents: [
      { id: 12, kind: "intent", state: "done", summary: "采集 www.acme.com 搜索接口的参数与端点" },
      { id: 18, kind: "intent", state: "running", summary: "枚举 api.acme.com 的对象接口" },
    ],
    facts: [{ id: 34, kind: "fact", state: "confirmed", summary: "search?q= 参数存在反射，返回 MySQL 语法错误提示" }],
  };
}

// ── 工作空间文件管理器（/workspace/*，demo：静态示例树）──
const WS_TREE: Record<string, { name: string; dir: boolean; size: number; content?: string }[]> = {
  "": [
    { name: "t-001", dir: true, size: 0 },
    { name: "transcripts", dir: true, size: 0 },
    {
      name: "notes.md",
      dir: false,
      size: 96,
      content: "# 工作区笔记\n\n（demo）这是工作空间根目录下的示例文件，可在线编辑并保存。\n",
    },
  ],
  "t-001": [
    { name: "i12", dir: true, size: 0 },
    { name: "recon.txt", dir: false, size: 64, content: "acme.com\nwww.acme.com\napi.acme.com\nadmin.acme.com\n" },
  ],
  "t-001/i12": [
    {
      name: "exploit.py",
      dir: false,
      size: 220,
      content:
        "#!/usr/bin/env python3\n# (demo) SQLi PoC — www.acme.com/search?q=\nimport requests\n\nr = requests.get('https://www.acme.com/search', params={'q': \"1' OR '1'='1\"})\nprint(r.status_code, len(r.text))\n",
    },
    {
      name: "response.html",
      dir: false,
      size: 180,
      content: "<!-- (demo) 抓到的响应体片段 -->\n<html><body>MySQL error near ''1'='1'</body></html>\n",
    },
  ],
  transcripts: [
    { name: "exp1-worker-i12.jsonl", dir: false, size: 512, content: "（demo）原始 LLM 对话记录示例，此处省略。" },
  ],
};

const wsClean = (p: string) => p.replace(/^\/+|\/+$/g, "");

export function workspaceList(path: string) {
  const key = wsClean(path);
  const items = WS_TREE[key] ?? [];
  const now = Date.parse("2026-08-09T03:00:00Z");
  return {
    path: key,
    entries: items.map((it, i) => ({
      name: it.name,
      path: key ? `${key}/${it.name}` : it.name,
      dir: it.dir,
      size: it.size,
      mtime: now - i * 3_600_000,
    })),
  };
}

export function workspaceRead(path: string) {
  const key = wsClean(path);
  const slash = key.lastIndexOf("/");
  const parent = slash >= 0 ? key.slice(0, slash) : "";
  const name = slash >= 0 ? key.slice(slash + 1) : key;
  const f = (WS_TREE[parent] ?? []).find((x) => x.name === name && !x.dir);
  return { path: key, size: f?.size ?? 0, binary: false, content: f?.content ?? "" };
}

// ── 工具执行历史（/commands）──
export const commandRecords = [
  {
    id: 1,
    exploration_id: 1,
    worker: "worker-1",
    tool: "bash",
    command: JSON.stringify({ command: "curl -s 'https://www.acme.com/search?q=test'" }),
    output: "HTTP/1.1 200 OK\nContent-Length: 12034\n<html>… 搜索结果页 …</html>",
    is_error: false,
    created_at: T("2026-08-09T02:10:00Z"),
  },
  {
    id: 2,
    exploration_id: 1,
    worker: "worker-1",
    tool: "bash",
    command: JSON.stringify({ command: "sqlmap -u 'https://www.acme.com/search?q=1' --batch --dbs" }),
    output: "[*] available databases [2]:\n[*] acme_prod\n[*] information_schema\nback-end DBMS: MySQL >= 5.7",
    is_error: false,
    created_at: T("2026-08-09T02:12:30Z"),
  },
  {
    id: 3,
    exploration_id: 1,
    worker: "worker-2",
    tool: "bash",
    command: JSON.stringify({ command: "ffuf -u https://api.acme.com/FUZZ -w common.txt -mc 200,401,403" }),
    output: "v1       [Status: 200]\nhealth   [Status: 200]\norders   [Status: 401]\nadmin    [Status: 403]",
    is_error: false,
    created_at: T("2026-08-09T02:15:00Z"),
  },
  {
    id: 4,
    exploration_id: 1,
    worker: "worker-2",
    tool: "bash",
    command: JSON.stringify({ command: "nmap -sV -p- 203.0.113.10" }),
    output: "PORT    STATE SERVICE VERSION\n22/tcp  open  ssh     OpenSSH 8.9\n443/tcp open  https   nginx 1.24",
    is_error: false,
    created_at: T("2026-08-09T02:18:00Z"),
  },
  {
    id: 5,
    exploration_id: 1,
    worker: "worker-1",
    tool: "bash",
    command: JSON.stringify({ command: "curl -s https://admin.acme.com/login --data 'user=admin&pass=admin123'" }),
    output: "curl: (28) Operation timed out after 10000 ms",
    is_error: true,
    created_at: T("2026-08-09T02:20:00Z"),
  },
];

// ── LLM 录制（/llm/records、/llm/records/{id}）──
export const llmRecords = [
  {
    id: 1,
    ts: T("2026-08-09T02:10:02Z"),
    model: "claude-opus-4-8",
    profile_name: "默认",
    session_id: "exp1-worker-i12",
    task_id: "t-001",
    worker: "worker-1",
    latency_ms: 3210,
    input_tokens: 8421,
    output_tokens: 512,
    cache_read: 6000,
    cache_write: 1200,
    status: "ok",
  },
  {
    id: 2,
    ts: T("2026-08-09T02:11:40Z"),
    model: "claude-opus-4-8",
    profile_name: "默认",
    session_id: "exp1-planner",
    task_id: "t-001",
    worker: "planner",
    latency_ms: 4180,
    input_tokens: 12044,
    output_tokens: 733,
    cache_read: 9000,
    cache_write: 1500,
    status: "ok",
  },
  {
    id: 3,
    ts: T("2026-08-09T02:15:20Z"),
    model: "claude-opus-4-8",
    profile_name: "默认",
    session_id: "exp1-worker-i18",
    task_id: "t-001",
    worker: "worker-2",
    latency_ms: 2890,
    input_tokens: 7311,
    output_tokens: 421,
    cache_read: 5200,
    cache_write: 900,
    status: "ok",
  },
  {
    id: 4,
    ts: T("2026-08-09T02:16:05Z"),
    model: "claude-opus-4-8",
    profile_name: "默认",
    session_id: "exp1-worker-i18",
    task_id: "t-001",
    worker: "worker-2",
    latency_ms: 900,
    input_tokens: 7600,
    output_tokens: 0,
    cache_read: 5200,
    cache_write: 0,
    status: "error",
    error: "429 Too Many Requests（已退避重试）",
  },
];

// Distinct tasks with counts, derived from llmRecords above (task picker).
const llmTaskCounts: Record<string, number> = {};
for (const r of llmRecords) if (r.task_id) llmTaskCounts[r.task_id] = (llmTaskCounts[r.task_id] ?? 0) + 1;
export const llmTasks: LLMTask[] = Object.entries(llmTaskCounts).map(([task_id, count]) => ({ task_id, count }));

export function llmRecordDetail(id: number, records = llmRecords) {
  const item = records.find((r) => r.id === id) ?? records[0];
  return {
    ...item,
    request_body: JSON.stringify(
      {
        model: item.model,
        system: "你是 Hawkeye 的采集执行者……（省略）",
        messages: [{ role: "user", content: "开始执行 system 提示里的这条意图：只做它、只产生事实、做完即停。" }],
        tools: ["bash", "insert_assets", "record_fact"],
      },
      null,
      2,
    ),
    response_body: JSON.stringify(
      {
        stop_reason: item.status === "error" ? "error" : "tool_use",
        content: [
          { type: "text", text: "采集 search 接口的响应结构与参数位置。" },
          { type: "tool_use", name: "bash", input: { command: "curl -s 'https://www.acme.com/search?q=acme'" } },
        ],
        usage: { input_tokens: item.input_tokens, output_tokens: item.output_tokens },
      },
      null,
      2,
    ),
    // HTTP 原文：请求含被归一化视图丢弃的完整工具 schema，响应为原始 SSE 帧。
    raw_request: JSON.stringify({
      model: item.model,
      max_tokens: 8192,
      stream: true,
      system: [
        { type: "text", text: "你是 Hawkeye 的采集执行者……（省略）", cache_control: { type: "ephemeral" } },
      ],
      messages: [{ role: "user", content: "开始执行 system 提示里的这条意图：只做它、只产生事实、做完即停。" }],
      tools: [
        {
          name: "bash",
          description: "在目标环境中执行一条 shell 命令并返回其输出。",
          input_schema: { type: "object", properties: { command: { type: "string" } }, required: ["command"] },
        },
      ],
      thinking: { type: "enabled" },
    }),
    raw_response: [
      `event: message_start`,
      `data: {"type":"message_start","message":{"id":"msg_01mock","model":"${item.model}","usage":{"input_tokens":${item.input_tokens},"output_tokens":1}}}`,
      ``,
      `event: content_block_delta`,
      `data: {"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"采集 search 接口的参数名与返回结构。"}}`,
      ``,
      `event: content_block_start`,
      `data: {"type":"content_block_start","index":1,"content_block":{"type":"tool_use","id":"toolu_01mock","name":"bash"}}`,
      ``,
      `event: content_block_delta`,
      `data: {"type":"content_block_delta","index":1,"delta":{"type":"input_json_delta","partial_json":"{\\"command\\":\\"curl -s 'https://www.acme.com/search?q=1%27'\\"}"}}`,
      ``,
      `event: message_delta`,
      `data: {"type":"message_delta","delta":{"stop_reason":"tool_use"},"usage":{"output_tokens":${item.output_tokens}}}`,
      ``,
      `event: message_stop`,
      `data: {"type":"message_stop"}`,
      ``,
    ].join("\n"),
  };
}
