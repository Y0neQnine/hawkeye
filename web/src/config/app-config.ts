import packageJson from "../../package.json";

const currentYear = new Date().getFullYear();

// 站点对外地址：部署时通过 NEXT_PUBLIC_SITE_URL 指定，用于 canonical / Open Graph /
// sitemap 的绝对链接。未设置时回退到本地默认端口，便于开发与离线构建。
const siteUrl = (process.env.NEXT_PUBLIC_SITE_URL ?? "http://localhost:8000").replace(/\/$/, "");

export const APP_CONFIG = {
  name: "Hawkeye",
  version: packageJson.version,
  copyright: `© ${currentYear}, Hawkeye.`,
  meta: {
    siteUrl,
    title: "Hawkeye — 攻击面收集与持续监控",
    description:
      "Hawkeye 是面向资产攻击面测绘与持续监控的平台：自动收集域名、子域、IP、服务与端点资产，对比变更并持续跟踪暴露面。",
    keywords: [
      "攻击面管理",
      "攻击面测绘",
      "资产收集",
      "资产监控",
      "子域名收集",
      "暴露面管理",
      "ASM",
      "External Attack Surface Management",
      "Hawkeye",
    ],
    ogImage: "/logo.png",
  },
};
