import type { MetadataRoute } from "next";

import { APP_CONFIG } from "@/config/app-config";

export const dynamic = "force-static";

export default function sitemap(): MetadataRoute.Sitemap {
  const base = APP_CONFIG.meta.siteUrl;
  return [
    {
      url: `${base}/`,
      changeFrequency: "weekly",
      priority: 1,
      lastModified: new Date(),
    },
    {
      url: `${base}/login/`,
      changeFrequency: "monthly",
      priority: 0.6,
    },
    {
      url: `${base}/setup/`,
      changeFrequency: "monthly",
      priority: 0.4,
    },
  ];
}
