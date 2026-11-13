"use client";

import * as React from "react";

import {
  CheckCircle2Icon,
  DownloadIcon,
  ExternalLinkIcon,
  InfoIcon,
  Loader2Icon,
  RefreshCwIcon,
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
  AlertDialogTrigger,
} from "@/components/ui/alert-dialog";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { api } from "@/lib/api";

type UpdateCheck = Awaited<ReturnType<typeof api.updateCheck>>;
type Phase = "idle" | "applying" | "restarting";

export function VersionCard() {
  const [version, setVersion] = React.useState("");
  const [check, setCheck] = React.useState<UpdateCheck | null>(null);
  const [checking, setChecking] = React.useState(false);
  const [checkErr, setCheckErr] = React.useState("");
  const [phase, setPhase] = React.useState<Phase>("idle");
  const [applyErr, setApplyErr] = React.useState("");

  const runCheck = React.useCallback(async () => {
    setChecking(true);
    setCheckErr("");
    try {
      setCheck(await api.updateCheck());
    } catch (e) {
      setCheck(null);
      setCheckErr((e as Error).message);
    } finally {
      setChecking(false);
    }
  }, []);

  React.useEffect(() => {
    api
      .health()
      .then((h) => setVersion(h.version ?? ""))
      .catch(() => setVersion(""))
      .finally(() => void runCheck());
  }, [runCheck]);

  // 更新链路会连续重启两次：下载完成后退出（75）→ 守护脚本拉起旧程序换装 →
  // 再次退出 → 新版本接管。轮询到版本号变化即刷新页面。
  const waitForRestart = React.useCallback((oldVersion: string) => {
    const start = Date.now();
    let sawDown = false;
    const tick = async () => {
      try {
        const h = await api.health();
        if (h?.ok) {
          if ((h.version && h.version !== oldVersion) || (sawDown && Date.now() - start > 15_000)) {
            toast.success(`已更新到 v${h.version || "新版本"}`);
            window.location.reload();
            return;
          }
          sawDown = false;
        }
      } catch {
        sawDown = true;
      }
      if (Date.now() - start > 120_000) {
        setPhase("idle");
        setApplyErr("服务未在 2 分钟内完成重启，请查看 hawkeye 日志或手动执行 ./update.sh");
        return;
      }
      window.setTimeout(tick, 1500);
    };
    window.setTimeout(tick, 1500);
  }, []);

  const apply = async () => {
    const oldVersion = version;
    setPhase("applying");
    setApplyErr("");
    try {
      const res = await api.updateApply();
      toast.success(`正在安装 v${res.latest}，服务即将重启…`);
      setPhase("restarting");
      waitForRestart(oldVersion);
    } catch (e) {
      setPhase("idle");
      setApplyErr((e as Error).message);
      toast.error("更新失败：" + (e as Error).message);
    }
  };

  const busy = phase !== "idle";
  const hasError = checkErr || applyErr;

  return (
    <Card className="mb-4 break-inside-avoid md:mb-6">
      <CardHeader>
        <CardTitle className="flex items-center gap-2 text-base">
          <InfoIcon className="size-4" />
          版本信息
        </CardTitle>
        <CardDescription>在线检查 GitHub Release，一键下载并更新当前程序。</CardDescription>
      </CardHeader>
      <CardContent className="space-y-3">
        <div className="flex flex-wrap items-center gap-2 text-sm">
          <span className="text-muted-foreground">当前版本</span>
          <Badge variant="secondary" className="font-mono">
            {version || "…"}
          </Badge>
          {check?.available && (
            <Badge className="font-mono" variant="outline">
              可更新至 v{check.latest}
            </Badge>
          )}
          <Button
            variant="ghost"
            size="icon-sm"
            title="检查更新"
            disabled={checking || busy}
            onClick={() => void runCheck()}
          >
            <RefreshCwIcon className={checking ? "size-3.5 animate-spin" : "size-3.5"} />
          </Button>
        </div>

        {checking && !check && (
          <p className="text-muted-foreground flex items-center gap-2 text-xs">
            <Loader2Icon className="size-3.5 animate-spin" />
            正在检查更新…
          </p>
        )}

        {checkErr && (
          <div className="space-y-1">
            <p className="text-destructive text-xs">{checkErr}</p>
            <p className="text-muted-foreground text-xs">
              可稍后重试；或在服务器执行 <span className="font-mono">./update.sh</span> 更新。
            </p>
          </div>
        )}

        {check && !checkErr && !check.available && (
          <p className="text-muted-foreground flex items-center gap-1.5 text-xs">
            <CheckCircle2Icon className="size-3.5" />
            已是最新版本
          </p>
        )}

        {check && !checkErr && check.available && (
          <div className="space-y-2">
            <div className="flex flex-wrap items-center gap-2 text-sm">
              <span>
                发现新版本 <span className="font-mono font-medium">v{check.latest}</span>
              </span>
              <a
                href={check.html_url}
                target="_blank"
                rel="noreferrer"
                className="text-muted-foreground hover:text-foreground inline-flex items-center gap-0.5 text-xs underline-offset-2 hover:underline"
              >
                更新说明
                <ExternalLinkIcon className="size-3" />
              </a>
            </div>
            {check.notes && (
              <pre className="bg-muted/40 text-muted-foreground max-h-40 overflow-auto rounded-md border p-2 text-xs whitespace-pre-wrap break-words">
                {check.notes}
              </pre>
            )}
            {check.platform_asset ? (
              <AlertDialog>
                <AlertDialogTrigger asChild>
                  <Button size="sm" disabled={busy}>
                    <DownloadIcon className="size-4" />
                    立即更新
                  </Button>
                </AlertDialogTrigger>
                <AlertDialogContent>
                  <AlertDialogHeader>
                    <AlertDialogTitle>更新到 v{check.latest}？</AlertDialogTitle>
                    <AlertDialogDescription>
                      将下载当前平台安装包、校验 SHA256 后替换程序，服务会自动重启，进行中的任务会中断。
                      Docker 部署更新的是容器内程序；重建容器后回到镜像版本，可用{" "}
                      <span className="font-mono">./update.sh</span> 拉取新镜像。
                    </AlertDialogDescription>
                  </AlertDialogHeader>
                  <AlertDialogFooter>
                    <AlertDialogCancel>取消</AlertDialogCancel>
                    <AlertDialogAction onClick={() => void apply()}>更新并重启</AlertDialogAction>
                  </AlertDialogFooter>
                </AlertDialogContent>
              </AlertDialog>
            ) : (
              <p className="text-destructive text-xs">
                该 Release 没有当前平台（{check.asset}）的安装包，请手动更新。
              </p>
            )}
          </div>
        )}

        {busy && (
          <p className="flex items-center gap-2 text-xs">
            <Loader2Icon className="size-3.5 animate-spin" />
            {phase === "applying"
              ? "正在下载并校验安装包…"
              : "正在重启，服务恢复后页面会自动刷新…"}
          </p>
        )}

        {!hasError && !busy && (
          <p className="text-muted-foreground text-xs">
            更新由 <span className="font-mono">start.sh</span> /{" "}
            <span className="font-mono">start.bat</span> 守护完成；直接运行二进制时更新后需手动重启。
          </p>
        )}
      </CardContent>
    </Card>
  );
}
