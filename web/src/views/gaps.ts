import type { Meta, Row } from "../data.ts";
import { h } from "../dom.ts";
import { applyFilter, parseSort, sortRows } from "../filter.ts";
import { yenFull } from "../format.ts";
import { countSignals } from "../stats.ts";
import { visibleColumns } from "../columns.ts";
import { PAGE, amountCell, amountHead, badges, detailToggle, loopCell, ministryOptions, nameCell, paged, scroll, select, sortOptions, summaryLineWithStale, th } from "./common.ts";

/** 閾値の説明文。判定は Go 側（zailoop build）で済んでいるので、ここでは表示するだけ。 */
function thresholdText(code: string, meta: Meta): string {
  const t = meta.thresholds;
  switch (code) {
    case "request_gap":
      return `当初/要求 < ${t.RequestGapRatio}`;
    case "low_execution":
      return `執行率 < ${t.LowExecRate}`;
    case "large_unused":
      return `不用相当額 ≥ ${yenFull(t.LargeUnusedYen)} かつ 不用/現額 ≥ ${t.LargeUnusedRatio}`;
    case "outcome_shortfall":
      return `達成率の最小 < ${t.OutcomeShortfall}%`;
    case "outcome_overshoot":
      return `達成率の最大 > ${t.OutcomeOvershoot}%`;
    default:
      return "";
  }
}

export function renderGaps(root: HTMLElement, rows: Row[], meta: Meta, params: URLSearchParams, update: (p: URLSearchParams) => void) {
  const selected = (params.get("sig") ?? "").split(",").filter(Boolean);
  const mode = params.get("mode") === "and" ? "and" : "or";
  const m = params.get("m") ?? "";
  const sort = parseSort(params.get("sort"), "signals", "desc");
  const shown = Number(params.get("n") ?? PAGE) || PAGE;
  const showAll = params.get("cols") === "all";
  const cols = visibleColumns(showAll, sort.key);
  const multi = meta.sheetYears.length > 1;
  const set = (k: string, v: string) => {
    const p = new URLSearchParams(params);
    if (v) p.set(k, v);
    else p.delete(k);
    p.delete("n");
    update(p);
  };

  const scoped = applyFilter(rows, { ministry: m });
  const counts = countSignals(scoped, meta.signals.map((s) => s.code));
  const active = selected.length ? selected : meta.signals.map((s) => s.code);
  const filtered = sortRows(applyFilter(scoped, { signals: active, mode }), sort.key, sort.dir);

  const tiles = h("div", { class: "tiles" });
  for (const s of meta.signals) {
    const on = selected.includes(s.code);
    tiles.appendChild(
      h(
        "label",
        { class: `tile ${on ? "on" : ""}`, title: s.description },
        h("input", {
          type: "checkbox",
          checked: on,
          onchange: () => {
            const next = on ? selected.filter((c) => c !== s.code) : [...selected, s.code];
            set("sig", next.join(","));
          },
        }),
        h("span", { class: "label" }, s.label),
        h("span", { class: "n" }, String(counts.get(s.code) ?? 0)),
        h("span", { class: "th muted" }, thresholdText(s.code, meta)),
      ),
    );
  }

  const controls = h(
    "div",
    { class: "controls" },
    select("m", ministryOptions(rows, meta), m, (v) => set("m", v)),
    select(
      "mode",
      [
        { value: "or", label: "いずれかに該当（OR）" },
        { value: "and", label: "すべてに該当（AND）" },
      ],
      mode,
      (v) => set("mode", v),
    ),
    select("sort", sortOptions([{ value: "signals:desc", label: "兆候の数 多い順" }]), `${sort.key}:${sort.dir}`, (v) => set("sort", v)),
    detailToggle(showAll, (v) => set("cols", v ? "all" : "")),
    selected.length ? h("button", { type: "button", onclick: () => set("sig", "") }, "選択を解除") : null,
  );

  const { tbody, more } = paged(
    filtered,
    (r) =>
      h(
        "tr",
        {},
        nameCell(r, meta),
        badges(r, meta),
        ...cols.map((col) => amountCell(col, r)),
        h("td", { class: "refl" }, r.reflection),
        ...(multi ? [loopCell(r)] : []),
      ),
    shown,
    () => {
      const p = new URLSearchParams(params);
      p.set("n", String(shown + PAGE));
      update(p);
    },
  );

  root.replaceChildren(
    h("h2", {}, "ループの断絶を探す", h("small", { class: "muted" }, " 要求→成立→執行→決算→評価→反映のどこかで整合しない事業")),
    h("p", { class: "muted" }, "このサイト独自の判定で、公式の評価ではありません。判定は生成時に行っていて、閾値は zailoop build の引数で変えられます。"),
    tiles,
    controls,
    summaryLineWithStale(filtered.length, scoped.length, filtered, meta),
    scroll(
      "table",
      { class: "rows" },
      h(
        "thead",
        {},
        h(
          "tr",
          {},
          h("th", {}, "事業名"),
          th("兆候", meta, "兆候"),
          ...cols.map((col) => amountHead(col, meta)),
          th("⑥ 反映", meta, "反映状況"),
          ...(multi ? [th("前年反映→当初", meta, "ループ検証")] : []),
        ),
      ),
      tbody,
    ),
    ...(more ? [more] : []),
  );
}
