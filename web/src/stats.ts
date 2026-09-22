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
