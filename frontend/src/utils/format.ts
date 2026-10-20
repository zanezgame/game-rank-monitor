/**
 * 通用格式化工具：日期（Asia/Shanghai 时区）、人数、上榜理由文案。
 */
import type { ReasonCode } from "@/api/client";

/** 上榜理由码 -> 中文文案（与后端契约一致） */
export const REASON_TEXT: Record<ReasonCode, string> = {
  new_entry: "首次上榜",
  re_entry: "重回榜单",
  surge: "排名大幅上升",
  drop: "排名明显下跌",
  stable: "排名稳定",
};

/** Asia/Shanghai 时区的今天，返回 YYYY-MM-DD */
export function todayStr(timeZone = "Asia/Shanghai"): string {
  // en-CA 的日期格式即 YYYY-MM-DD
  return new Intl.DateTimeFormat("en-CA", {
    timeZone,
    year: "numeric",
    month: "2-digit",
    day: "2-digit",
  }).format(new Date());
}

/** "2026-10-18" -> "10月18日" */
export function formatDateShort(dateStr: string): string {
  const parts = dateStr.split("-").map(Number);
  if (parts.length < 3 || parts.some(Number.isNaN)) return dateStr;
  return `${parts[1]}月${parts[2]}日`;
}

/** ISO 时间字符串 -> "10-18 14:30"（Asia/Shanghai 时区） */
export function formatDateTime(iso: string): string {
  const d = new Date(iso);
  if (Number.isNaN(d.getTime())) return iso;
  return new Intl.DateTimeFormat("zh-CN", {
    timeZone: "Asia/Shanghai",
    month: "2-digit",
    day: "2-digit",
    hour: "2-digit",
    minute: "2-digit",
    hour12: false,
  }).format(d);
}

/** 人数格式化：1181577 -> "118.2万"，null/undefined -> "—" */
export function formatCount(n: number | null | undefined): string {
  if (n === null || n === undefined) return "—";
  if (n >= 100_000_000) return `${(n / 100_000_000).toFixed(1)}亿`;
  if (n >= 10_000) return `${(n / 10_000).toFixed(1)}万`;
  return String(Math.round(n));
}

/** 纯日历日期加减天数：("2026-10-18", -6) -> "2026-10-12" */
export function addDays(dateStr: string, days: number): string {
  const [y, m, d] = dateStr.split("-").map(Number);
  const dt = new Date(Date.UTC(y, m - 1, d) + days * 86_400_000);
  return dt.toISOString().slice(0, 10);
}
