import { VERDICT_LABEL, type Meta, type Row } from "../data.ts";
import { initialDelta } from "../filter.ts";
import { h, raw, type Child } from "../dom.ts";
import { yenFull, yenShort, pct } from "../format.ts";
import { bar, sparkline } from "../svg.ts";
import type { AmountCol } from "../columns.ts";

export const PAGE = 100;

/** この行が基準より古いシートのものか（実績年度がずれる）。 */
export function isStale(r: Row, meta: Meta): boolean {
  return r.sheetYear < meta.sheetYear;
}

/** 事業名セル（2 段）。1 段目は詳細への相対リンク、2 段目は ID・府省庁・区分とスパークライン。
 * 古いシートの行には年度を明示する。ID・府省庁・区分の列は持たず、ここにまとめる。 */
export function nameCell(r: Row, meta?: Meta): HTMLElement {
  const top = h("div", { class: "nm" }, h("a", { href: `p/${r.id}.html` }, r.name));
  if (meta && isStale(r, meta)) {
    top.appendChild(h("span", { class: "stale", title: `${r.sheetYear}年度シートまで。金額は FY${r.actualYear} 軸` }, `${r.sheetYear}年度まで`));
  }
  const sub = h("div", { class: "nm-sub" }, h("span", { class: "id" }, r.id), h("span", {}, r.ministry), r.category ? h("span", {}, r.category) : null, raw(sparkline(r.byYear)));
  return h("td", { class: "name" }, top, sub);
}

/** 横に広い表を包む。狭い画面では表だけ横スクロールし、ページ全体は横に動かない。 */
export function scroll(tag: string, attrs: Record<string, string>, ...children: Child[]): HTMLElement {
  return h("div", { class: "tablewrap" }, h(tag, attrs, ...children));
}

/** 見出しセル。用語があれば説明を title に入れ、点線の下線で示す（詳しくは terms.html）。 */
export function th(label: string, meta: Meta, term = "", cls = ""): HTMLElement {
  const t = term ? meta.terms.find((x) => x.name === term) : undefined;
  return h("th", cls ? { class: cls } : {}, t ? h("abbr", { title: t.desc }, label) : label);
}

/** 金額列の見出し。 */
export function amountHead(c: AmountCol, meta: Meta): HTMLElement {
  const n = meta.actualYear;
  switch (c) {
    case "request":
      return th(`① FY${n} 要求`, meta, "概算要求", "num");
    case "initial":
      return th("② 当初", meta, "当初予算", "num");
    case "current":
      return th("② 現額", meta, "歳出予算現額", "num");
    case "executed":
      return th("③ 執行", meta, "執行額", "num");
    case "execRate":
      return th("③ 執行率", meta, "執行率");
    case "unused":
      return th("④ 不用相当", meta, "不用相当額", "num");
    case "nextRequest":
      return th(`⑥ FY${n + 2} 要求`, meta, "概算要求", "num");
  }
}

/** 金額列のセル。 */
export function amountCell(c: AmountCol, r: Row): HTMLElement {
  switch (c) {
    case "request":
      return yenCell(r.request);
    case "initial":
      return yenCell(r.initial);
    case "current":
      return yenCell(r.current);
    case "executed":
      return yenCell(r.executed);
    case "execRate":
      return rateCell(r);
    case "unused":
      return yenCell(r.unused, r.unusedState ? "muted" : "");
    case "nextRequest":
      return yenCell(r.nextRequest);
  }
}

/** 詳細列の切り替え（URL の cols=all）。 */
export function detailToggle(showAll: boolean, onchange: (v: boolean) => void): HTMLElement {
  return h(
    "label",
    { class: "toggle" },
    h("input", { type: "checkbox", checked: showAll, onchange: (e) => onchange((e.target as HTMLInputElement).checked) }),
    " 詳細列（要求・現額・不用相当・次年度要求）",
  );
}

/** 金額セル（短い表記、title に全桁）。 */
export function yenCell(v: number | null, cls = ""): HTMLElement {
  return h("td", { class: `num ${cls}`, title: yenFull(v) }, yenShort(v));
}

/** 基準年度と実績年度が違う行の件数を添えた件数表示。 */
export function summaryLineWithStale(n: number, total: number, rows: Row[], meta: Meta): HTMLElement {
  const stale = rows.filter((r) => isStale(r, meta)).length;
  const el = summaryLine(n, total);
  if (stale > 0) el.appendChild(h("span", { class: "muted" }, `（うち ${stale.toLocaleString("ja-JP")} 件は ${meta.sheetYear} 年度シートがなく、実績年度が 1 年古い）`));
  return el;
}

/** 執行率セル（バー + 数値）。 */
export function rateCell(r: Row): HTMLElement {
  if (r.execState) return h("td", { class: "rate muted" }, r.execState);
  return h("td", { class: "rate" }, raw(bar(r.execRate)), h("span", {}, pct(r.execRate)));
}

/** シグナルのバッジ列。 */
export function badges(r: Row, meta: Meta): HTMLElement {
  const td = h("td", { class: "badges" });
  for (const s of meta.signals) {
    if (r.signalCodes.includes(s.code)) td.appendChild(h("span", { class: "badge", title: s.description }, s.label));
  }
  return td;
}

/** 「さらに表示」付きの表本体を描く。 */
export function paged(
  rows: Row[],
  renderRow: (r: Row) => HTMLElement,
  shown: number,
  onMore: () => void,
): { tbody: HTMLElement; more: HTMLElement | null } {
  const tbody = h("tbody");
  for (const r of rows.slice(0, shown)) tbody.appendChild(renderRow(r));
  const more =
    rows.length > shown
      ? h("p", { class: "more" }, h("button", { type: "button", onclick: onMore }, `さらに表示（残り ${rows.length - shown} 件）`))
      : null;
  return { tbody, more };
}

/** select 要素。 */
export function select(name: string, options: { value: string; label: string }[], value: string, onchange: (v: string) => void): HTMLElement {
  const sel = h("select", { name, onchange: (e) => onchange((e.target as HTMLSelectElement).value) });
  for (const o of options) {
    const opt = h("option", { value: o.value }, o.label) as HTMLOptionElement;
    if (o.value === value) opt.selected = true;
    sel.appendChild(opt);
  }
  return sel;
}

export function ministryOptions(rows: Row[], meta: Meta): { value: string; label: string }[] {
  const counts = new Map<string, number>();
  for (const r of rows) counts.set(r.ministry, (counts.get(r.ministry) ?? 0) + 1);
  return [{ value: "", label: "府省庁: すべて" }, ...meta.ministries.map((m) => ({ value: m, label: `${m} (${counts.get(m) ?? 0})` }))];
}

export function sortOptions(extra: { value: string; label: string }[] = []): { value: string; label: string }[] {
  return [
    { value: "id:asc", label: "ID 順" },
    { value: "initial:desc", label: "当初予算 多い順" },
    { value: "current:desc", label: "現額 多い順" },
    { value: "executed:desc", label: "執行額 多い順" },
    { value: "execRate:asc", label: "執行率 低い順" },
    { value: "execRate:desc", label: "執行率 高い順" },
    { value: "unused:desc", label: "不用相当額 多い順" },
    { value: "request:desc", label: "概算要求 多い順" },
    { value: "nextRequest:desc", label: "次年度要求 多い順" },
    ...extra,
  ];
}

export function summaryLine(n: number, total: number): HTMLElement {
  return h("p", { class: "count muted" }, `${n.toLocaleString("ja-JP")} / ${total.toLocaleString("ja-JP")} 事業`);
}

/** 前年シートの反映状況 → 翌年当初（FY S+1）の増減セル。複数年度のときだけ意味を持つ。 */
export function loopCell(r: Row): HTMLElement {
  const d = initialDelta(r);
  if (!r.prevReflection && d === null) return h("td", { class: "muted" }, "—");
  const cls = r.verdict === 3 ? "bad" : r.verdict === 2 ? "good" : "";
  const arrow = d === null ? "" : d < 0 ? "↓" : d > 0 ? "↑" : "→";
  return h(
    "td",
    { class: `loop ${cls}`, title: d === null ? "" : `翌年当初の増減 ${yenFull(d)}（${VERDICT_LABEL[r.verdict] ?? ""}）` },
    r.prevReflection || "(空)",
    " ",
    h("span", { class: "arrow" }, arrow, d === null ? "" : ` ${yenShort(Math.abs(d))}`),
  );
}
