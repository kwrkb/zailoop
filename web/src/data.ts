// index.html に埋め込まれた JSON の型と、添字を文字列に戻すデコード。
// キーの意味は internal/site/index.go の Row/Meta と一致させる。

export interface SignalMeta {
  bit: number;
  code: string;
  label: string;
  description: string;
  count: number;
}

export interface Thresholds {
  RequestGapRatio: number;
  LowExecRate: number;
  LargeUnusedYen: number;
  LargeUnusedRatio: number;
  OutcomeShortfall: number;
  OutcomeOvershoot: number;
}

export interface Term {
  id: string;
  name: string;
  desc: string;
}

export interface Meta {
  sheetYear: number;
  actualYear: number;
  years: number[];
  ministries: string[];
  categories: string[];
  reflections: string[];
  signals: SignalMeta[];
  thresholds: Thresholds;
  sheetYears: number[];
  count: number;
  attribution: string;
  generated: string;
  /** 用語の説明（terms.html と同じ） */
  terms: Term[];
}

export interface RawRow {
  id: string;
  n: string;
  m: number;
  c: number;
  rf: number;
  rq?: number;
  in?: number;
  cu?: number;
  ex?: number;
  er?: number;
  xs?: string;
  co?: number;
  un?: number;
  ud?: number;
  us?: string;
  ra?: number;
  ni?: number;
  nr?: number;
  yr: (number | null)[];
  oc?: number;
  or?: number[];
  sg?: number;
  pr?: number;
  pi?: number;
  lv?: number;
  rn?: boolean;
  sy?: number;
}

export interface Payload {
  meta: Meta;
  rows: RawRow[];
}

/** デコード済みの 1 事業。数値は円、無効は null。 */
export interface Row {
  id: string;
  name: string;
  ministry: string;
  category: string;
  reflection: string;
  request: number | null;
  initial: number | null;
  current: number | null;
  executed: number | null;
  execRate: number | null;
  execState: string;
  carriedOut: number | null;
  unused: number | null;
  unusedDiff: number | null;
  unusedState: string;
  reflected: number | null;
  nextInitial: number | null;
  nextRequest: number | null;
  byYear: (number | null)[];
  outcomes: number;
  rates: number[];
  signals: number;
  signalCodes: string[];
  /** 前年シートの反映状況（複数年度のときだけ） */
  prevReflection: string;
  prevInitial: number | null;
  /** 0 不明 / 1 対象外 / 2 整合 / 3 矛盾 */
  verdict: number;
  renamed: boolean;
  /** この行の最新シートの事業年度。meta.sheetYear より古ければ実績年度もずれる */
  sheetYear: number;
  /** この行の実績年度（sheetYear − 1） */
  actualYear: number;
}

export const VERDICT_LABEL: Record<number, string> = { 0: "不明", 1: "判定対象外", 2: "整合", 3: "矛盾" };

const n = (v: number | undefined): number | null => (v === undefined ? null : v);

export function decodeRow(r: RawRow, meta: Meta): Row {
  const sg = r.sg ?? 0;
  return {
    id: r.id,
    name: r.n,
    ministry: meta.ministries[r.m] ?? "",
    category: meta.categories[r.c] ?? "",
    reflection: r.rf >= 0 ? (meta.reflections[r.rf] ?? "") : "",
    request: n(r.rq),
    initial: n(r.in),
    current: n(r.cu),
    executed: n(r.ex),
    execRate: n(r.er),
    execState: r.xs ?? "",
    carriedOut: n(r.co),
    unused: n(r.un),
    unusedDiff: n(r.ud),
    unusedState: r.us ?? "",
    reflected: n(r.ra),
    nextInitial: n(r.ni),
    nextRequest: n(r.nr),
    byYear: r.yr ?? [],
    outcomes: r.oc ?? 0,
    rates: r.or ?? [],
    signals: sg,
    signalCodes: meta.signals.filter((s) => (sg & s.bit) !== 0).map((s) => s.code),
    prevReflection: r.pr && r.pr > 0 ? (meta.reflections[r.pr - 1] ?? "") : "",
    prevInitial: n(r.pi),
    verdict: r.lv ?? 0,
    renamed: r.rn ?? false,
    sheetYear: r.sy ?? meta.sheetYear,
    actualYear: (r.sy ?? meta.sheetYear) - 1,
  };
}

export function decode(p: Payload): { meta: Meta; rows: Row[] } {
  return { meta: p.meta, rows: p.rows.map((r) => decodeRow(r, p.meta)) };
}
