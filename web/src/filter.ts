import type { Row } from "./data.ts";

/** 検索用の正規化: NFKC + 小文字 + 空白除去。全角英数や半角カナの揺れを吸収する。 */
export function normalize(s: string): string {
  return s.normalize("NFKC").toLowerCase().replace(/\s+/g, "");
}

export interface Filter {
  q?: string;
  ministry?: string;
  category?: string;
  /** 指定した signal code のいずれか（mode=or）／すべて（mode=and）を持つ行だけ */
  signals?: string[];
  mode?: "or" | "and";
}

export function applyFilter(rows: Row[], f: Filter): Row[] {
  const q = f.q ? normalize(f.q) : "";
  const sigs = f.signals ?? [];
  return rows.filter((r) => {
    if (f.ministry && r.ministry !== f.ministry) return false;
    if (f.category && r.category !== f.category) return false;
    if (q && r.id !== q && !normalize(r.name).includes(q)) return false;
    if (sigs.length > 0) {
      const has = sigs.map((c) => r.signalCodes.includes(c));
      if (f.mode === "and" ? !has.every(Boolean) : !has.some(Boolean)) return false;
    }
    return true;
  });
}

export type SortKey =
  | "id"
  | "request"
  | "initial"
  | "current"
  | "executed"
  | "execRate"
  | "unused"
  | "nextRequest"
  | "signals"
  | "minRate"
  | "maxRate";

export type SortDir = "asc" | "desc";

const SORT_KEYS: SortKey[] = ["id", "request", "initial", "current", "executed", "execRate", "unused", "nextRequest", "signals", "minRate", "maxRate"];

/** ソートキーの値。null は常に末尾。 */
export function sortValue(r: Row, key: SortKey): number | null {
  switch (key) {
    case "id":
      return Number(r.id);
    case "request":
      return r.request;
    case "initial":
      return r.initial;
    case "current":
      return r.current;
    case "executed":
      return r.executed;
    case "execRate":
      return r.execRate;
    case "unused":
      return r.unused;
    case "nextRequest":
      return r.nextRequest;
    case "signals":
      return r.signalCodes.length;
    case "minRate":
      return r.rates.length ? Math.min(...r.rates) : null;
    case "maxRate":
      return r.rates.length ? Math.max(...r.rates) : null;
  }
}

export function sortRows(rows: Row[], key: SortKey, dir: SortDir): Row[] {
  const sign = dir === "asc" ? 1 : -1;
  return rows
    .map((r, i) => ({ r, i, v: sortValue(r, key) }))
    .sort((a, b) => {
      if (a.v === null && b.v === null) return a.i - b.i;
      if (a.v === null) return 1;
      if (b.v === null) return -1;
      return a.v === b.v ? a.i - b.i : (a.v - b.v) * sign;
    })
    .map((x) => x.r);
}

/** 並べ替え指定 "initial:desc" を解析する。不正なら既定。 */
export function parseSort(s: string | null | undefined, def: SortKey = "id", defDir: SortDir = "asc"): { key: SortKey; dir: SortDir } {
  if (!s) return { key: def, dir: defDir };
  const [k, d] = s.split(":");
  const key = SORT_KEYS.includes(k as SortKey) ? (k as SortKey) : def;
  const dir: SortDir = d === "asc" || d === "desc" ? d : defDir;
  return { key, dir };
}
