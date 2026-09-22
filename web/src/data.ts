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

export interface Meta {
  sheetYear: number;
  actualYear: number;
  years: number[];
  ministries: string[];
  categories: string[];
  reflections: string[];
  signals: SignalMeta[];
  thresholds: Thresholds;
  count: number;
  attribution: string;
  generated: string;
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
}

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
  };
}

export function decode(p: Payload): { meta: Meta; rows: Row[] } {
  return { meta: p.meta, rows: p.rows.map((r) => decodeRow(r, p.meta)) };
}
