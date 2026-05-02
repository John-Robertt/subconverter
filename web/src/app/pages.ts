import {
  Download,
  Filter,
  GitBranch,
  Layers,
  ListChecks,
  Network,
  Route,
  Settings,
  ShieldCheck,
  SlidersHorizontal,
  SquareStack,
  Activity
} from "lucide-react";
import type { LucideIcon } from "lucide-react";

export type PageSection = "config" | "runtime" | "system";

export interface PageDefinition {
  path: string;
  label: string;
  section: PageSection;
  title: string;
  subtitle: string;
  icon: LucideIcon;
}

export const pages: PageDefinition[] = [
  { path: "/sources", label: "订阅来源", section: "config", title: "订阅来源", subtitle: "管理上游订阅、单节点池与自定义代理", icon: Network },
  { path: "/filters", label: "过滤器", section: "config", title: "过滤器", subtitle: "用正则排除流量信息节点和广告条目", icon: Filter },
  { path: "/groups", label: "节点分组", section: "config", title: "节点分组", subtitle: "将节点按地区或属性聚合为可路由的分组", icon: Layers },
  { path: "/routing", label: "路由策略", section: "config", title: "路由策略", subtitle: "组装服务组，将分组、特殊关键字和规则集串起来", icon: Route },
  { path: "/rulesets", label: "规则集", section: "config", title: "规则集", subtitle: "为每个服务组挂载远端规则列表", icon: SquareStack },
  { path: "/rules", label: "内联规则", section: "config", title: "内联规则", subtitle: "直接在配置里写的内联路由规则", icon: ListChecks },
  { path: "/settings", label: "其他配置", section: "config", title: "其他配置", subtitle: "fallback / base_url / 模板等基础设置", icon: SlidersHorizontal },
  { path: "/validate", label: "静态校验", section: "config", title: "静态校验", subtitle: "保存前的全面校验，错误会集中列出", icon: ShieldCheck },
  { path: "/nodes", label: "节点预览", section: "runtime", title: "节点预览", subtitle: "查看所有来源拉取到的真实节点", icon: Activity },
  { path: "/preview/groups", label: "分组预览", section: "runtime", title: "分组预览", subtitle: "查看每个分组与服务组实际包含的节点", icon: GitBranch },
  { path: "/download", label: "生成下载", section: "runtime", title: "生成下载", subtitle: "导出 Clash Meta 与 Surge 配置文件", icon: Download },
  { path: "/status", label: "系统状态", section: "system", title: "系统状态", subtitle: "后端进程、配置加载与热重载历史", icon: Settings }
];

export const sectionLabels: Record<PageSection, string> = {
  config: "配置",
  runtime: "预览",
  system: "系统"
};

export function findPage(pathname: string) {
  return pages.find((page) => page.path === pathname) ?? pages[0];
}
