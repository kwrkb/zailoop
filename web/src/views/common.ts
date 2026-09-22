import { VERDICT_LABEL, type Meta, type Row } from "../data.ts";
import { initialDelta } from "../filter.ts";
import { h, raw } from "../dom.ts";
import { yenFull, yenShort, pct } from "../format.ts";
import { bar, sparkline } from "../svg.ts";

export const PAGE = 100;

/** 事業名セル（詳細への相対リンク + スパークライン）。 */
export function nameCell(r: Row): HTMLElement {
  return h("td", { class: "name" }, h("a", { href: `p/${r.id}.html` }, r.name), raw(sparkline(r.byYear)));
}

/** 金額セル（短い表記、title に全桁）。 */
export function yenCell(v: number | null, cls = ""): HTMLElement {
  return h("td", { class: `num ${cls}`, title: yenFull(v) }, yenShort(v));
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
