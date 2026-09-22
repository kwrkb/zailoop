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
