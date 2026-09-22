import type { Meta, Row } from "../data.ts";
import { h } from "../dom.ts";
import { applyFilter, parseSort, sortRows } from "../filter.ts";
import { yenFull } from "../format.ts";
import { countSignals } from "../stats.ts";
import { PAGE, badges, ministryOptions, nameCell, paged, rateCell, select, sortOptions, summaryLine, yenCell } from "./common.ts";

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
    selected.length ? h("button", { type: "button", onclick: () => set("sig", "") }, "選択を解除") : null,
  );

  const { tbody, more } = paged(
    filtered,
    (r) =>
      h(
        "tr",
        {},
        h("td", { class: "id" }, r.id),
        nameCell(r),
        h("td", {}, r.ministry),
        badges(r, meta),
        yenCell(r.request),
        yenCell(r.initial),
        yenCell(r.current),
        yenCell(r.executed),
        rateCell(r),
        yenCell(r.unused, r.unusedState ? "muted" : ""),
        h("td", {}, r.reflection),
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
    h("p", { class: "muted" }, "判定は生成時に行っています。閾値を変えるには zailoop build の引数を使ってください。"),
    tiles,
    controls,
    summaryLine(filtered.length, scoped.length),
    h(
      "table",
      { class: "rows" },
      h(
        "thead",
        {},
        h(
          "tr",
          {},
          h("th", {}, "ID"),
          h("th", {}, "事業名"),
          h("th", {}, "府省庁"),
          h("th", {}, "兆候"),
          h("th", { class: "num" }, "① 要求"),
          h("th", { class: "num" }, "② 当初"),
          h("th", { class: "num" }, "② 現額"),
          h("th", { class: "num" }, "③ 執行"),
          h("th", {}, "③ 執行率"),
          h("th", { class: "num" }, "④ 不用相当"),
          h("th", {}, "⑥ 反映"),
        ),
      ),
      tbody,
    ),
    ...(more ? [more] : []),
  );
}
