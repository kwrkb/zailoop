import type { Row } from "./data.ts";

/** 達成率（%）のヒストグラムの区間。 */
export interface Bin {
  label: string;
  count: number;
  test: (v: number) => boolean;
}

export function rateBins(): Bin[] {
  return [
    { label: "<50", count: 0, test: (v) => v < 50 },
    { label: "50–80", count: 0, test: (v) => v >= 50 && v < 80 },
    { label: "80–100", count: 0, test: (v) => v >= 80 && v < 100 },
    { label: "100", count: 0, test: (v) => v === 100 },
    { label: "100–120", count: 0, test: (v) => v > 100 && v <= 120 },
    { label: "120–200", count: 0, test: (v) => v > 120 && v <= 200 },
    { label: ">200", count: 0, test: (v) => v > 200 },
  ];
}

/** 行の全達成率をヒストグラムに集計する。 */
export function histogram(rows: Row[]): Bin[] {
  const bins = rateBins();
  for (const r of rows) {
    for (const v of r.rates) {
      const b = bins.find((x) => x.test(v));
      if (b) b.count++;
    }
  }
  return bins;
}

/** signal code ごとの件数。 */
export function countSignals(rows: Row[], codes: string[]): Map<string, number> {
  const m = new Map<string, number>(codes.map((c) => [c, 0]));
  for (const r of rows) for (const c of r.signalCodes) m.set(c, (m.get(c) ?? 0) + 1);
  return m;
}

/** 達成率をもつ行だけ。 */
export function withRates(rows: Row[]): Row[] {
  return rows.filter((r) => r.rates.length > 0);
}

/** 前年シートの反映状況 × 翌年当初（FY S+1）の増減（減/同/増）のクロス表。比較基準は前年シートの FY S 当初。 */
export interface CrossCell {
  reflection: string;
  down: number;
  same: number;
  up: number;
}

export function crosstab(rows: Row[], order: string[]): CrossCell[] {
  const m = new Map<string, CrossCell>();
  for (const r of rows) {
    if (r.nextInitial === null || r.prevInitial === null) continue;
    const k = r.prevReflection || "(空)";
    const c = m.get(k) ?? { reflection: k, down: 0, same: 0, up: 0 };
    if (r.nextInitial < r.prevInitial) c.down++;
    else if (r.nextInitial > r.prevInitial) c.up++;
    else c.same++;
    m.set(k, c);
  }
  const keys = [...order.filter((k) => m.has(k)), ...[...m.keys()].filter((k) => !order.includes(k))];
  return keys.map((k) => m.get(k)!);
}

/** index 上部の概要。金額は基準のシート年度（実績年度が揃う行）だけで合計する。 */
export interface Overview {
  projects: number;
  /** 金額の合計に使った行（基準シートの行）の数 */
  onAxis: number;
  initial: number;
  current: number;
  executed: number;
  /** 執行額の合計 ÷ 歳出予算現額の合計（どちらも確定し、現額が正の行だけ）。対象がなければ null */
  execRate: number | null;
  withSignals: number;
  contradictions: number;
}

export function overview(rows: Row[], sheetYear: number): Overview {
  const o: Overview = { projects: rows.length, onAxis: 0, initial: 0, current: 0, executed: 0, execRate: null, withSignals: 0, contradictions: 0 };
  let rc = 0;
  let re = 0;
  for (const r of rows) {
    if (r.signalCodes.length > 0) o.withSignals++;
    if (r.verdict === 3) o.contradictions++;
    if (r.sheetYear !== sheetYear) continue;
    o.onAxis++;
    o.initial += r.initial ?? 0;
    o.current += r.current ?? 0;
    o.executed += r.executed ?? 0;
    if (!r.execState && r.current !== null && r.current > 0 && r.executed !== null) {
      rc += r.current;
      re += r.executed;
    }
  }
  o.execRate = rc > 0 ? re / rc : null;
  return o;
}

/** 府省庁ごとの規模。当初予算は基準シートの行だけで合計し、多い順に並べる。 */
export interface MinistryTotal {
  ministry: string;
  count: number;
  initial: number;
  withSignals: number;
}

export function ministryTotals(rows: Row[], sheetYear: number): MinistryTotal[] {
  const m = new Map<string, MinistryTotal>();
  for (const r of rows) {
    const t = m.get(r.ministry) ?? { ministry: r.ministry, count: 0, initial: 0, withSignals: 0 };
    t.count++;
    if (r.signalCodes.length > 0) t.withSignals++;
    if (r.sheetYear === sheetYear) t.initial += r.initial ?? 0;
    m.set(r.ministry, t);
  }
  return [...m.values()].sort((a, b) => b.initial - a.initial || b.count - a.count);
}
