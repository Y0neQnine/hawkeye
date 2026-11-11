"use client";

import * as React from "react";

import {
  ArrowDownRightIcon,
  ArrowRightIcon,
  ArrowUpRightIcon,
  CameraIcon,
  ChevronLeftIcon,
  ChevronRightIcon,
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
import { Input } from "@/components/ui/input";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table";
import { api } from "@/lib/api";
import type { Company, MonitorChange, MonitorOverview, MonitorRun, MonitorTrendPoint } from "@/lib/types";
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
  { value: "service", label: "服务" },
  { value: "endpoint", label: "接口" },
];

const PAGE_SIZE = 20;

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

  const loadBase = React.useCallback(async () => {
    setLoading(true);
    try {
      const [ov, runPage, points, companyList] = await Promise.all([
        api.monitorOverview(),
        api.monitorRuns(30, 0),
        api.monitorTrend(14),
        api.companies(),
      ]);
      setOverview(ov);
      setRuns(runPage.runs);
      setTrend(points);
      setCompanies(companyList);
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
    </div>
  );
}
