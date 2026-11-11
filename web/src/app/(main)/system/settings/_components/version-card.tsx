"use client";

import * as React from "react";

import { InfoIcon } from "lucide-react";

import { Badge } from "@/components/ui/badge";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { api } from "@/lib/api";

export function VersionCard() {
  const [version, setVersion] = React.useState("");

  React.useEffect(() => {
    api
      .health()
      .then((h) => setVersion(h.version ?? ""))
      .catch(() => setVersion(""));
  }, []);

  return (
    <Card className="mb-4 break-inside-avoid md:mb-6">
      <CardHeader>
        <CardTitle className="flex items-center gap-2 text-base">
          <InfoIcon className="size-4" />
          版本信息
        </CardTitle>
        <CardDescription>Hawkeye 私有构建，不提供在线更新。</CardDescription>
      </CardHeader>
      <CardContent className="space-y-3">
        <div className="flex flex-wrap items-center gap-2 text-sm">
          <span className="text-muted-foreground">当前版本</span>
          <Badge variant="secondary" className="font-mono">
            {version || "…"}
          </Badge>
        </div>
        <p className="text-xs text-muted-foreground">
          升级方式：替换程序二进制后重启服务。请通过 <span className="font-mono">start.sh</span>（Windows 为
          <span className="font-mono"> start.bat</span>）启动 Hawkeye，程序退出后由守护脚本按退出码决定是否重新拉起。
        </p>
      </CardContent>
    </Card>
  );
}
