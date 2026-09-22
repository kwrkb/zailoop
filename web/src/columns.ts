// 一覧・断絶ビューの金額列の出し分け。DOM に触れない純粋関数（node:test で確かめる）。
import type { SortKey } from "./filter.ts";

export type AmountCol = "request" | "initial" | "current" | "executed" | "execRate" | "unused" | "nextRequest";

/** 表示順。 */
export const AMOUNT_COLS: AmountCol[] = ["request", "initial", "current", "executed", "execRate", "unused", "nextRequest"];

/** 既定で出す列。残り（要求・現額・不用相当・次年度要求）は「詳細列」で出す。 */
export const DEFAULT_COLS: AmountCol[] = ["initial", "executed", "execRate"];

/** 表示する金額列。詳細列を出すときは全列、そうでなければ既定の列と、並べ替えに使っている列。 */
export function visibleColumns(showAll: boolean, sortKey: SortKey): AmountCol[] {
  if (showAll) return AMOUNT_COLS;
  return AMOUNT_COLS.filter((c) => DEFAULT_COLS.includes(c) || c === sortKey);
}
