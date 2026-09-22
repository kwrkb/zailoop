// 一覧の上に置く概要: 全体の数字と府省庁別の規模。金額の合計は基準シートの行だけ（stats.overview）。
import type { Meta, Row } from "../data.ts";
import { h, raw } from "../dom.ts";
import { pct, yenFull, yenShort } from "../format.ts";
import { ministryTotals, overview } from "../stats.ts";
import { bar } from "../svg.ts";

const TOP = 10;

export function renderOverview(rows: Row[], meta: Meta, params: URLSearchParams, update: (p: URLSearchParams) => void): HTMLElement {
  const o = overview(rows, meta.sheetYear);
  const n = meta.actualYear;
  const multi = meta.sheetYears.length > 1;
  const cur = params.get("m") ?? "";
  const all = params.get("mall") === "1";
  const set = (changes: Record<string, string | null>) => {
    const p = new URLSearchParams(params);
    for (const [k, v] of Object.entries(changes)) {
      if (v) p.set(k, v);
      else p.delete(k);
    }
    p.delete("n");
    update(p);
  };
  const num = (v: number) => v.toLocaleString("ja-JP");

  const stat = (label: string, value: string, note: string, href = "", title = "") =>
    h(href ? "a" : "div", { class: "stat", ...(href ? { href } : {}), ...(title ? { title } : {}) }, h("span", { class: "label" }, label), h("strong", { class: "value" }, value), h("span", { class: "note" }, note));

  const stats = h(
    "div",
    { class: "stats" },
    stat("事業数", num(o.projects), o.onAxis < o.projects ? `金額の合計は FY${n} 軸の ${num(o.onAxis)} 事業` : `${meta.sheetYear}年度シート`),
    stat(`FY${n} 当初予算`, yenShort(o.initial), "事業ごとの値の単純合計（国の予算総額とは一致しない）", "", yenFull(o.initial)),
    stat(`FY${n} 執行率`, pct(o.execRate), `執行 ${yenShort(o.executed)} ／ 現額 ${yenShort(o.current)}`, "", "執行額の合計 ÷ 歳出予算現額の合計（現額が 0 以下の事業を除く）"),
    stat("兆候のある事業", num(o.withSignals), `全体の ${pct(o.projects ? o.withSignals / o.projects : null, 0)} → 断絶を探す`, "#gaps"),
    multi ? stat("ループ検証で「矛盾」", num(o.contradictions), "反映と逆に増額 → ループ検証", "#loops?v=3") : null,
  );

  const totals = ministryTotals(rows, meta.sheetYear);
  const max = Math.max(1, ...totals.map((t) => t.initial));
  // 上位に入らない府省庁で絞り込んでいても、その行は見えるようにする
  const shown = all ? totals : totals.filter((t, i) => i < TOP || t.ministry === cur);
  const list = h("div", { class: "ministries" });
  for (const t of shown) {
    const on = t.ministry === cur;
    list.appendChild(
      h(
        "button",
        { type: "button", class: `ministry ${on ? "on" : ""}`, title: `${t.ministry} で一覧を絞り込む`, onclick: () => set({ m: on ? null : t.ministry }) },
        h("span", { class: "mname" }, t.ministry),
        raw(bar(t.initial / max)),
        h("span", { class: "mval", title: yenFull(t.initial) }, yenShort(t.initial)),
        h("span", { class: "msig" }, `${pct(o.initial ? t.initial / o.initial : null)}・${num(t.count)} 事業・兆候 ${num(t.withSignals)}`),
      ),
    );
  }
  const more =
    totals.length > TOP
      ? h("button", { type: "button", class: "linklike", onclick: () => set({ mall: all ? null : "1" }) }, all ? "上位だけ表示" : `残り ${totals.length - shown.length} 府省庁も表示`)
      : null;

  return h(
    "section",
    { class: "overview" },
    stats,
    h("h3", {}, `府省庁別の FY${n} 当初予算`, h("small", { class: "muted" }, " 押すと一覧をその府省庁で絞り込みます")),
    h("p", { class: "note muted" }, "一般会計と特別会計の事業を単純に足した値です。会計間の繰入れで重複しうるため、国の予算総額とは一致しません。棒の長さは金額に比例します。"),
    list,
    more,
  );
}
