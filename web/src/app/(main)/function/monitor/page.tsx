"use client";

import * as React from "react";

import {
  ArrowDownRightIcon,
  ArrowRightIcon,
  ArrowUpRightIcon,
  CameraIcon,
  ChevronLeftIcon,
  ChevronRightIcon,
  PlayIcon,
  PlusIcon,
  RefreshCwIcon,
  Trash2Icon,
} from "lucide-react";
import { toast } from "sonner";

import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from "@/components/ui/alert-dialog";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Card } from "@/components/ui/card";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { Input } from "@/components/ui/input";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { Switch } from "@/components/ui/switch";
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table";
import { api } from "@/lib/api";
import type {
  Company,
  MonitorChange,
  MonitorOverview,
  MonitorRun,
  MonitorTarget,
  MonitorTrendPoint,
} from "@/lib/types";
import { cn } from "@/lib/utils";

const CHANGE_META = {
  new: { label: "新增", tone: "bg-emerald-100 text-emerald-700", icon: ArrowUpRightIcon },
  gone: { label: "消失", tone: "bg-slate-100 text-slate-600", icon: ArrowDownRightIcon },
  changed: { label: "变更", tone: "bg-amber-100 text-amber-700", icon: ArrowRightIcon },
} as const;

const ASSET_TYPES = [
  { value: "all", label: "全部类型" },
  { value: "root_domain", label: "根域名" },
  { value: "subdomain", label: "子域名" },
  { value: "ip", label: "IP" },
  { value: "app", label: "应用" },
  { value: "third_party", label: "第三方" },
  { value: "service", label: "服务" },
  { value: "endpoint", label: "接口" },
];

const PAGE_SIZE = 20;

const INTERVAL_OPTIONS = [
  { value: 360, label: "每 6 小时" },
  { value: 720, label: "每 12 小时" },
  { value: 1440, label: "每天" },
  { value: 4320, label: "每 3 天" },
  { value: 10080, label: "每周" },
  { value: 43200, label: "每 30 天" },
];

const FIELD_LABELS: Record<string, string> = {
  domain: "域名",
  root_domain: "根域名",
  ip: "IP",
  port: "端口",
  url: "URL",
  evidence_url: "佐证 URL",
  service_name: "服务名",
  service_type: "服务类型",
  status_code: "状态码",
  content_length: "响应长度",
  page_title: "页面标题",
  technologies: "技术栈",
  favicon_mmh3: "图标指纹",
  open_ports: "开放端口",
  record_type: "解析类型",
  record_value: "解析记录",
  bundle_id: "包名",
  app_name: "应用名称",
  category: "分类",
  icp: "备案",
  company_id: "归属企业",
};

function intervalLabel(minutes: number) {
  return INTERVAL_OPTIONS.find((item) => item.value === minutes)?.label ?? `每 ${Math.round(minutes / 60)} 小时`;
}

function changedFields(change: MonitorChange): string[] {
  const list = (change.detail as { changes?: Array<{ field?: string }> } | undefined)?.changes;
  if (!Array.isArray(list)) return [];
  return list
    .map((item) => item.field)
    .filter((field): field is string => !!field)
    .map((field) => FIELD_LABELS[field] ?? field);
}

function formatTime(value?: string) {
  if (!value) return "—";
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return value;
  return date.toLocaleString("zh-CN", { hour12: false });
}

function formatDuration(ms: number) {
  if (!ms) return "—";
  if (ms < 1000) return `${ms} ms`;
  return `${Math.round(ms / 1000)} s`;
}

export default function MonitorPage() {
  const [overview, setOverview] = React.useState<MonitorOverview>();
  const [runs, setRuns] = React.useState<MonitorRun[]>([]);
  const [trend, setTrend] = React.useState<MonitorTrendPoint[]>([]);
  const [companies, setCompanies] = React.useState<Company[]>([]);
  const [changes, setChanges] = React.useState<MonitorChange[]>([]);
  const [totalChanges, setTotalChanges] = React.useState(0);
  const [loading, setLoading] = React.useState(true);
  const [changesLoading, setChangesLoading] = React.useState(false);
  const [snapshotting, setSnapshotting] = React.useState(false);
  const [runID, setRunID] = React.useState(0);
  const [changeType, setChangeType] = React.useState("all");
  const [assetType, setAssetType] = React.useState("all");
  const [companyID, setCompanyID] = React.useState(0);
  const [query, setQuery] = React.useState("");
  const [page, setPage] = React.useState(0);
  const [deleteTarget, setDeleteTarget] = React.useState<MonitorRun | null>(null);
  const [deleting, setDeleting] = React.useState(false);

  const [autoEnabled, setAutoEnabled] = React.useState(false);
  const [autoSaving, setAutoSaving] = React.useState(false);
  const [targets, setTargets] = React.useState<MonitorTarget[]>([]);
  const [targetBusy, setTargetBusy] = React.useState(0);
  const [createOpen, setCreateOpen] = React.useState(false);
  const [creating, setCreating] = React.useState(false);
  const [newCompanyID, setNewCompanyID] = React.useState(0);
  const [newDomain, setNewDomain] = React.useState("");
  const [newInterval, setNewInterval] = React.useState(1440);
  const [newAssetsOnly, setNewAssetsOnly] = React.useState(true);
  const [removeTarget, setRemoveTarget] = React.useState<MonitorTarget | null>(null);
  const [removingTarget, setRemovingTarget] = React.useState(false);

  const loadBase = React.useCallback(async () => {
    setLoading(true);
    try {
      const [ov, runPage, points, companyList, targetState] = await Promise.all([
        api.monitorOverview(),
        api.monitorRuns(30, 0),
        api.monitorTrend(14),
        api.companies(),
        api.monitorTargets(),
      ]);
      setOverview(ov);
      setRuns(runPage.runs);
      setTrend(points);
      setCompanies(companyList);
      setTargets(targetState.targets);
      setAutoEnabled(targetState.enabled);
      setNewCompanyID((current) => current || companyList[0]?.id || 0);
    } catch {
      toast.error("监控概览加载失败");
    } finally {
      setLoading(false);
    }
  }, []);

  const loadChanges = React.useCallback(async () => {
    setChangesLoading(true);
    try {
      const result = await api.monitorChanges({
        run_id: runID || undefined,
        change_type: changeType,
        asset_type: assetType,
        company_id: companyID || undefined,
        q: query.trim() || undefined,
        limit: PAGE_SIZE,
        offset: page * PAGE_SIZE,
      });
      setChanges(result.changes);
      setTotalChanges(result.total);
    } catch {
      toast.error("变更列表加载失败");
    } finally {
      setChangesLoading(false);
    }
  }, [runID, changeType, assetType, companyID, query, page]);

  React.useEffect(() => {
    void loadBase();
  }, [loadBase]);

  React.useEffect(() => {
    void loadChanges();
  }, [loadChanges]);

  const refresh = () => {
    void loadBase();
    void loadChanges();
  };

  const runSnapshot = async () => {
    setSnapshotting(true);
    try {
      const run = await api.monitorSnapshot();
      toast.success(`快照 #${run.id} 已完成：新增 ${run.new_count} / 消失 ${run.gone_count} / 变更 ${run.changed_count}`);
      setRunID(run.id);
      setPage(0);
      await loadBase();
    } catch {
      toast.error("快照执行失败");
    } finally {
      setSnapshotting(false);
    }
  };

  const confirmDeleteRun = async () => {
    if (!deleteTarget) return;
    const id = deleteTarget.id;
    setDeleting(true);
    try {
      const res = await api.deleteMonitorRun(id);
      if (res.deleted === 0) toast.warning(`快照批次 #${id} 已不存在`);
      else toast.success(`已删除快照批次 #${id}`);
      if (runID === id) {
        setRunID(0);
        setPage(0);
      }
      await loadBase();
      await loadChanges();
    } catch (e) {
      toast.error("删除快照批次失败：" + String((e as Error)?.message ?? e));
    } finally {
      setDeleting(false);
      setDeleteTarget(null);
    }
  };

  const toggleAuto = async (next: boolean) => {
    setAutoSaving(true);
    try {
      const res = await api.setMonitorAuto(next);
      setAutoEnabled(res.enabled);
      toast.success(res.enabled ? "已开启自动监控，将按目标周期发起增量采集" : "已关闭自动监控，不再自动发起采集");
    } catch (e) {
      toast.error("切换自动监控失败：" + String((e as Error)?.message ?? e));
    } finally {
      setAutoSaving(false);
    }
  };

  const patchTarget = async (target: MonitorTarget, body: { interval_minutes?: number; enabled?: boolean }) => {
    setTargetBusy(target.id);
    try {
      const updated = await api.updateMonitorTarget(target.id, body);
      setTargets((prev) => prev.map((item) => (item.id === updated.id ? updated : item)));
    } catch (e) {
      toast.error("更新监控目标失败：" + String((e as Error)?.message ?? e));
    } finally {
      setTargetBusy(0);
    }
  };

  const runTargetNow = async (target: MonitorTarget) => {
    setTargetBusy(target.id);
    try {
      const res = await api.runMonitorTarget(target.id);
      toast.success(`已启动增量采集任务 #${res.task_id}`);
      await loadBase();
    } catch (e) {
      toast.error("启动监控失败：" + String((e as Error)?.message ?? e));
    } finally {
      setTargetBusy(0);
    }
  };

  const submitCreateTarget = async () => {
    if (!newCompanyID) {
      toast.error("请选择企业");
      return;
    }
    if (!newDomain.trim()) {
      toast.error("请填写根域名");
      return;
    }
    setCreating(true);
    try {
      const target = await api.createMonitorTarget({
        company_id: newCompanyID,
        root_domain: newDomain.trim(),
        interval_minutes: newInterval,
        assets_only: newAssetsOnly,
        enabled: true,
      });
      toast.success(`已添加监控目标 ${target.root_domain}`);
      setCreateOpen(false);
      setNewDomain("");
      await loadBase();
    } catch (e) {
      toast.error("添加监控目标失败：" + String((e as Error)?.message ?? e));
    } finally {
      setCreating(false);
    }
  };

  const confirmRemoveTarget = async () => {
    if (!removeTarget) return;
    setRemovingTarget(true);
    try {
      const res = await api.deleteMonitorTarget(removeTarget.id);
      if (res.deleted === 0) toast.warning("监控目标已不存在");
      else toast.success(`已删除监控目标 ${removeTarget.root_domain}`);
      await loadBase();
    } catch (e) {
      toast.error("删除监控目标失败：" + String((e as Error)?.message ?? e));
    } finally {
      setRemovingTarget(false);
      setRemoveTarget(null);
    }
  };

  const totalPages = Math.max(1, Math.ceil(totalChanges / PAGE_SIZE));
  const latest = overview?.latest_run;
  const maxTrend = Math.max(1, ...trend.map((p) => p.new + p.gone + p.changed));

  return (
    <div className="flex min-h-0 flex-col gap-4">
      <div className="flex flex-wrap items-center justify-between gap-3">
        <div>
          <h1 className="text-xl font-semibold tracking-tight">监控中心</h1>
          <p className="text-sm text-muted-foreground">
            按批次对比资产指纹，持续跟踪新增、消失与变更。
            {latest ? ` 最近快照 ${formatTime(latest.finished_at ?? latest.started_at)}` : ""}
          </p>
        </div>
        <div className="flex items-center gap-2">
          <Button variant="outline" size="sm" onClick={refresh} disabled={loading}>
            <RefreshCwIcon className={cn("size-4", loading && "animate-spin")} /> 刷新
          </Button>
          <Button size="sm" onClick={runSnapshot} disabled={snapshotting}>
            <CameraIcon className={cn("size-4", snapshotting && "animate-pulse")} />
            {snapshotting ? "快照中…" : "立即快照"}
          </Button>
        </div>
      </div>

      <div className="grid gap-3 sm:grid-cols-2 xl:grid-cols-4">
        <Card className="gap-1 p-4">
          <div className="text-sm text-muted-foreground">资产总量</div>
          <div className="text-2xl font-semibold tabular-nums">{overview?.asset_total ?? "—"}</div>
          <div className="text-xs text-muted-foreground">共 {overview?.company_total ?? 0} 家企业</div>
        </Card>
        <Card className="gap-1 p-4">
          <div className="text-sm text-muted-foreground">快照批次</div>
          <div className="text-2xl font-semibold tabular-nums">{overview?.runs_total ?? "—"}</div>
          <div className="text-xs text-muted-foreground">
            {latest ? `最近一批耗时 ${formatDuration(latest.duration_ms)}` : "尚无快照"}
          </div>
        </Card>
        <Card className="gap-1 p-4">
          <div className="text-sm text-muted-foreground">近 24h 变更</div>
          <div className="text-2xl font-semibold tabular-nums">
            {(overview?.last_24h.new ?? 0) + (overview?.last_24h.gone ?? 0) + (overview?.last_24h.changed ?? 0)}
          </div>
          <div className="flex gap-2 text-xs text-muted-foreground">
            <span className="text-emerald-600">新增 {overview?.last_24h.new ?? 0}</span>
            <span>消失 {overview?.last_24h.gone ?? 0}</span>
            <span className="text-amber-600">变更 {overview?.last_24h.changed ?? 0}</span>
          </div>
        </Card>
        <Card className="gap-1 p-4">
          <div className="text-sm text-muted-foreground">近 7 天变更</div>
          <div className="text-2xl font-semibold tabular-nums">
            {(overview?.last_7d.new ?? 0) + (overview?.last_7d.gone ?? 0) + (overview?.last_7d.changed ?? 0)}
          </div>
          <div className="flex gap-2 text-xs text-muted-foreground">
            <span className="text-emerald-600">新增 {overview?.last_7d.new ?? 0}</span>
            <span>消失 {overview?.last_7d.gone ?? 0}</span>
            <span className="text-amber-600">变更 {overview?.last_7d.changed ?? 0}</span>
          </div>
        </Card>
      </div>

      <Card className="p-4">
        <div className="flex flex-wrap items-start justify-between gap-3">
          <div>
            <div className="flex items-center gap-2 text-sm font-medium">
              自动监控
              <Switch
                checked={autoEnabled}
                onCheckedChange={(value) => void toggleAuto(value)}
                disabled={autoSaving}
                aria-label="自动监控开关"
              />
              {autoEnabled ? (
                <Badge className="bg-emerald-100 text-emerald-700">已开启</Badge>
              ) : (
                <Badge variant="outline">已关闭</Badge>
              )}
            </div>
            <p className="mt-1 max-w-3xl text-xs text-muted-foreground">
              开启后按目标周期自动发起增量采集：先读已有资产作为基线，只补新发现的子域/服务；任务结束立即快照并对比。
              关闭后不自动采集，只保留手动快照与外部同步。
            </p>
          </div>
          <Button
            variant="outline"
            size="sm"
            onClick={() => {
              if (!newCompanyID) setNewCompanyID(companies[0]?.id ?? 0);
              setCreateOpen(true);
            }}
            disabled={companies.length === 0}
          >
            <PlusIcon className="size-4" /> 新增监控目标
          </Button>
        </div>
        <div className="mt-3 overflow-x-auto">
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>企业</TableHead>
                <TableHead>根域名</TableHead>
                <TableHead className="w-36">周期</TableHead>
                <TableHead className="w-24">范围</TableHead>
                <TableHead className="w-44">上次运行</TableHead>
                <TableHead className="w-20">启用</TableHead>
                <TableHead className="w-24 text-right">操作</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {targets.map((target) => (
                <TableRow key={target.id}>
                  <TableCell className="text-sm">{target.company_name || `#${target.company_id}`}</TableCell>
                  <TableCell className="font-mono text-xs">{target.root_domain}</TableCell>
                  <TableCell>
                    <Select
                      value={String(target.interval_minutes)}
                      onValueChange={(value) => void patchTarget(target, { interval_minutes: Number(value) })}
                      disabled={targetBusy === target.id}
                    >
                      <SelectTrigger className="h-8 w-[130px]" size="sm">
                        <SelectValue />
                      </SelectTrigger>
                      <SelectContent>
                        {INTERVAL_OPTIONS.map((item) => (
                          <SelectItem key={item.value} value={String(item.value)}>
                            {item.label}
                          </SelectItem>
                        ))}
                      </SelectContent>
                    </Select>
                  </TableCell>
                  <TableCell>
                    <Badge variant="secondary">{target.assets_only ? "仅资产" : "完整"}</Badge>
                  </TableCell>
                  <TableCell className="text-xs text-muted-foreground">
                    {formatTime(target.last_run_at)}
                    {target.last_task_id ? ` · 任务 #${target.last_task_id}` : ""}
                  </TableCell>
                  <TableCell>
                    <Switch
                      checked={target.enabled}
                      onCheckedChange={(value) => void patchTarget(target, { enabled: value })}
                      disabled={targetBusy === target.id}
                      aria-label={`启用 ${target.root_domain}`}
                    />
                  </TableCell>
                  <TableCell className="text-right">
                    <div className="flex justify-end gap-1">
                      <Button
                        variant="ghost"
                        size="icon"
                        className="size-7"
                        onClick={() => void runTargetNow(target)}
                        disabled={targetBusy === target.id}
                        aria-label={`立即运行 ${target.root_domain}`}
                      >
                        <PlayIcon className="size-3.5" />
                      </Button>
                      <Button
                        variant="ghost"
                        size="icon"
                        className="size-7 text-muted-foreground hover:text-destructive"
                        onClick={() => setRemoveTarget(target)}
                        aria-label={`删除监控目标 ${target.root_domain}`}
                      >
                        <Trash2Icon className="size-3.5" />
                      </Button>
                    </div>
                  </TableCell>
                </TableRow>
              ))}
              {targets.length === 0 && (
                <TableRow>
                  <TableCell colSpan={7} className="h-20 text-center text-sm text-muted-foreground">
                    还没有监控目标。新增后，开启自动监控即可按周期发现该企业根域名下新出现的系统。
                  </TableCell>
                </TableRow>
              )}
            </TableBody>
          </Table>
        </div>
      </Card>

      <Card className="p-4">
        <div className="mb-3 flex items-center justify-between">
          <div className="text-sm font-medium">近 14 天变更趋势</div>
          <div className="flex items-center gap-3 text-xs text-muted-foreground">
            <span className="flex items-center gap-1">
              <span className="size-2 rounded-sm bg-emerald-500" /> 新增
            </span>
            <span className="flex items-center gap-1">
              <span className="size-2 rounded-sm bg-slate-400" /> 消失
            </span>
            <span className="flex items-center gap-1">
              <span className="size-2 rounded-sm bg-amber-500" /> 变更
            </span>
          </div>
        </div>
        <div className="flex h-32 items-end gap-1">
          {trend.length === 0 ? (
            <div className="flex h-full w-full items-center justify-center text-sm text-muted-foreground">暂无趋势数据</div>
          ) : (
            trend.map((point) => (
              <div key={point.date} className="flex h-full flex-1 flex-col justify-end gap-1">
                <div className="flex w-full flex-col justify-end">
                  <div
                    className="w-full bg-amber-500"
                    title={`${point.date} 变更 ${point.changed}`}
                    style={{ height: `${(point.changed / maxTrend) * 96}px` }}
                  />
                  <div
                    className="w-full bg-slate-400"
                    title={`${point.date} 消失 ${point.gone}`}
                    style={{ height: `${(point.gone / maxTrend) * 96}px` }}
                  />
                  <div
                    className="w-full bg-emerald-500"
                    title={`${point.date} 新增 ${point.new}`}
                    style={{ height: `${(point.new / maxTrend) * 96}px` }}
                  />
                </div>
                <div className="text-center text-[10px] text-muted-foreground">{point.date.slice(5)}</div>
              </div>
            ))
          )}
        </div>
      </Card>

      <div className="grid min-h-0 gap-4 xl:grid-cols-[320px_minmax(0,1fr)]">
        <Card className="flex min-h-0 flex-col gap-0 overflow-hidden py-0">
          <div className="flex items-center justify-between border-b px-4 py-3">
            <div className="text-sm font-medium">快照批次</div>
            <Badge variant="secondary" className="tabular-nums">
              {runs.length}
            </Badge>
          </div>
          <div className="min-h-0 flex-1 overflow-auto">
            <button
              type="button"
              onClick={() => {
                setRunID(0);
                setPage(0);
              }}
              className={cn(
                "flex w-full items-center justify-between border-b px-4 py-3 text-left text-sm hover:bg-muted/50",
                runID === 0 && "bg-muted/60",
              )}
            >
              <span>全部批次</span>
              <span className="text-xs text-muted-foreground">聚合视图</span>
            </button>
            {runs.map((run) => (
              <div key={run.id} className={cn("group relative border-b", runID === run.id && "bg-muted/60")}>
                <button
                  type="button"
                  onClick={() => {
                    setRunID(run.id);
                    setPage(0);
                  }}
                  className="flex w-full flex-col gap-1 px-4 py-3 pr-10 text-left hover:bg-muted/50"
                >
                  <div className="flex items-center justify-between gap-2">
                    <span className="font-medium tabular-nums">#{run.id}</span>
                    <span className="text-xs text-muted-foreground">{formatTime(run.finished_at ?? run.started_at)}</span>
                  </div>
                  <div className="flex items-center gap-2 text-xs text-muted-foreground">
                    <span>{run.trigger === "manual" ? "手动" : "定时"}</span>
                    {run.baseline && <Badge variant="outline">基线</Badge>}
                    <span>资产 {run.asset_total}</span>
                  </div>
                  <div className="flex gap-2 text-xs">
                    <span className="text-emerald-600">+{run.new_count}</span>
                    <span className="text-slate-500">-{run.gone_count}</span>
                    <span className="text-amber-600">~{run.changed_count}</span>
                  </div>
                </button>
                <Button
                  variant="ghost"
                  size="icon"
                  className="absolute top-2 right-2 size-7 text-muted-foreground opacity-0 group-hover:opacity-100 hover:text-destructive focus-visible:opacity-100"
                  onClick={() => setDeleteTarget(run)}
                  aria-label={`删除快照批次 #${run.id}`}
                >
                  <Trash2Icon className="size-3.5" />
                </Button>
              </div>
            ))}
            {runs.length === 0 && <div className="px-4 py-8 text-center text-sm text-muted-foreground">暂无快照</div>}
          </div>
        </Card>

        <Card className="flex min-h-0 flex-col overflow-hidden py-0">
          <div className="flex flex-wrap items-center gap-2 border-b px-4 py-3">
            <div className="mr-auto text-sm font-medium">
              资产变更
              <span className="ml-2 text-xs font-normal text-muted-foreground">
                {totalChanges} 条{runID ? ` · 批次 #${runID}` : ""}
              </span>
            </div>
            <Select
              value={changeType}
              onValueChange={(value) => {
                setChangeType(value);
                setPage(0);
              }}
            >
              <SelectTrigger className="w-[110px]" size="sm">
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value="all">全部变更</SelectItem>
                <SelectItem value="new">新增</SelectItem>
                <SelectItem value="gone">消失</SelectItem>
                <SelectItem value="changed">变更</SelectItem>
              </SelectContent>
            </Select>
            <Select
              value={assetType}
              onValueChange={(value) => {
                setAssetType(value);
                setPage(0);
              }}
            >
              <SelectTrigger className="w-[120px]" size="sm">
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                {ASSET_TYPES.map((item) => (
                  <SelectItem key={item.value} value={item.value}>
                    {item.label}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
            <Select
              value={String(companyID)}
              onValueChange={(value) => {
                setCompanyID(Number(value));
                setPage(0);
              }}
            >
              <SelectTrigger className="w-[140px]" size="sm">
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value="0">全部企业</SelectItem>
                {companies.map((company) => (
                  <SelectItem key={company.id} value={String(company.id)}>
                    {company.name}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
            <Input
              value={query}
              onChange={(event) => {
                setQuery(event.target.value);
                setPage(0);
              }}
              placeholder="搜索资产标签…"
              className="h-8 w-[180px]"
            />
          </div>

          <div className="min-h-0 flex-1 overflow-auto">
            <Table>
              <TableHeader className="sticky top-0 z-10 bg-card">
                <TableRow>
                  <TableHead className="w-24">类型</TableHead>
                  <TableHead>资产</TableHead>
                  <TableHead className="w-36">企业</TableHead>
                  <TableHead className="w-20">批次</TableHead>
                  <TableHead className="w-44 text-right">时间</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {changes.map((change) => {
                  const meta = CHANGE_META[change.change_type];
                  const Icon = meta.icon;
                  const fields = changedFields(change);
                  return (
                    <TableRow key={change.id}>
                      <TableCell>
                        <span className={cn("inline-flex items-center gap-1 rounded px-1.5 py-0.5 text-xs", meta.tone)}>
                          <Icon className="size-3" />
                          {meta.label}
                        </span>
                      </TableCell>
                      <TableCell>
                        <div className="flex flex-col">
                          <span className="font-mono text-xs">{change.label}</span>
                          <span className="text-[11px] text-muted-foreground">{change.asset_type}</span>
                          {fields.length > 0 && (
                            <span className="text-[11px] text-amber-700">
                              字段：{fields.slice(0, 3).join("、")}
                              {fields.length > 3 ? ` 等 ${fields.length} 项` : ""}
                            </span>
                          )}
                        </div>
                      </TableCell>
                      <TableCell className="text-sm">{change.company ?? "—"}</TableCell>
                      <TableCell className="tabular-nums text-sm">#{change.run_id}</TableCell>
                      <TableCell className="text-right text-xs text-muted-foreground">
                        {formatTime(change.created_at)}
                      </TableCell>
                    </TableRow>
                  );
                })}
                {!changesLoading && changes.length === 0 && (
                  <TableRow>
                    <TableCell colSpan={5} className="h-32 text-center text-sm text-muted-foreground">
                      该条件下没有变更记录
                    </TableCell>
                  </TableRow>
                )}
              </TableBody>
            </Table>
          </div>

          <div className="flex items-center justify-between border-t px-4 py-2">
            <span className="text-xs text-muted-foreground">
              第 {page + 1} / {totalPages} 页
            </span>
            <div className="flex items-center gap-1">
              <Button
                variant="outline"
                size="icon"
                className="size-8"
                onClick={() => setPage((value) => Math.max(0, value - 1))}
                disabled={page === 0 || changesLoading}
              >
                <ChevronLeftIcon className="size-4" />
              </Button>
              <Button
                variant="outline"
                size="icon"
                className="size-8"
                onClick={() => setPage((value) => Math.min(totalPages - 1, value + 1))}
                disabled={page >= totalPages - 1 || changesLoading}
              >
                <ChevronRightIcon className="size-4" />
              </Button>
            </div>
          </div>
        </Card>
      </div>

      <AlertDialog
        open={!!deleteTarget}
        onOpenChange={(open) => {
          if (!open) setDeleteTarget(null);
        }}
      >
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>
              删除快照批次{deleteTarget ? ` · #${deleteTarget.id}` : ""}
            </AlertDialogTitle>
            <AlertDialogDescription>
              该批次的快照与变更记录会一并删除，不可撤销。删除最新批次后，下一次快照将与上一批次对比；若没有可对比的批次，将重新建立基线。
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel disabled={deleting}>取消</AlertDialogCancel>
            <AlertDialogAction
              onClick={(event) => {
                event.preventDefault();
                void confirmDeleteRun();
              }}
              disabled={deleting}
              className="bg-destructive text-destructive-foreground hover:bg-destructive/90"
            >
              {deleting ? "删除中…" : "确认删除"}
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>

      <Dialog open={createOpen} onOpenChange={setCreateOpen}>
        <DialogContent className="sm:max-w-[460px]">
          <DialogHeader>
            <DialogTitle>新增监控目标</DialogTitle>
            <DialogDescription>
              按企业 + 根域名周期性发起增量采集，任务结束自动快照并对比变更；新资产按根域名归属到该企业。
            </DialogDescription>
          </DialogHeader>
          <div className="grid gap-3">
            <div className="grid gap-1.5">
              <span className="text-sm">企业</span>
              <Select value={String(newCompanyID)} onValueChange={(value) => setNewCompanyID(Number(value))}>
                <SelectTrigger className="w-full">
                  <SelectValue placeholder="选择企业" />
                </SelectTrigger>
                <SelectContent>
                  {companies.map((company) => (
                    <SelectItem key={company.id} value={String(company.id)}>
                      {company.name}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
            </div>
            <div className="grid gap-1.5">
              <span className="text-sm">根域名</span>
              <Input
                value={newDomain}
                onChange={(event) => setNewDomain(event.target.value)}
                placeholder="example.com（可粘贴 URL，自动取根域名）"
              />
            </div>
            <div className="grid gap-1.5">
              <span className="text-sm">监控周期</span>
              <Select value={String(newInterval)} onValueChange={(value) => setNewInterval(Number(value))}>
                <SelectTrigger className="w-full">
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  {INTERVAL_OPTIONS.map((item) => (
                    <SelectItem key={item.value} value={String(item.value)}>
                      {item.label}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
            </div>
            <div className="flex items-center justify-between rounded-md border px-3 py-2">
              <div>
                <div className="text-sm">仅采集资产</div>
                <div className="text-xs text-muted-foreground">只补资产与稳定属性，不做更深的页面/端点采集</div>
              </div>
              <Switch checked={newAssetsOnly} onCheckedChange={setNewAssetsOnly} aria-label="仅采集资产" />
            </div>
          </div>
          <DialogFooter>
            <Button variant="outline" onClick={() => setCreateOpen(false)} disabled={creating}>
              取消
            </Button>
            <Button onClick={() => void submitCreateTarget()} disabled={creating}>
              {creating ? "添加中…" : "添加"}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      <AlertDialog
        open={!!removeTarget}
        onOpenChange={(open) => {
          if (!open) setRemoveTarget(null);
        }}
      >
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>删除监控目标{removeTarget ? ` · ${removeTarget.root_domain}` : ""}</AlertDialogTitle>
            <AlertDialogDescription>
              删除后不会再按周期自动采集该根域名，已采集的资产与历史快照不受影响。
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel disabled={removingTarget}>取消</AlertDialogCancel>
            <AlertDialogAction
              onClick={(event) => {
                event.preventDefault();
                void confirmRemoveTarget();
              }}
              disabled={removingTarget}
              className="bg-destructive text-destructive-foreground hover:bg-destructive/90"
            >
              {removingTarget ? "删除中…" : "确认删除"}
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </div>
  );
}
