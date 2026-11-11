import type { MetadataRoute } from "next";

import { APP_CONFIG } from "@/config/app-config";

export const dynamic = "force-static";

// 仅登录页、初始化页对外开放；业务页面需要认证且随构建静态导出，
// 对爬虫没有可索引内容，一并挡掉以免消耗抓取配额。
export default function robots(): MetadataRoute.Robots {
  return {
    rules: [
      {
        userAgent: "*",
        allow: "/",
        disallow: ["/api/", "/dashboard/", "/chat/", "/function/", "/system/"],
      },
    ],
    sitemap: `${APP_CONFIG.meta.siteUrl}/sitemap.xml`,
    host: APP_CONFIG.meta.siteUrl,
  };
}
