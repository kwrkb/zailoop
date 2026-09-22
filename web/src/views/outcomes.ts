import type { Meta, Row } from "../data.ts";
import { h, raw } from "../dom.ts";
import { applyFilter, parseSort, sortRows } from "../filter.ts";
import { ratePct } from "../format.ts";
import { histogram, withRates } from "../stats.ts";
import { histogramSvg } from "../svg.ts";
import { PAGE, ministryOptions, nameCell, paged, scroll, select, summaryLine, yenCell } from "./common.ts";

type Tab = "short" | "over" | "none";

export function renderOutcomes(root: HTMLElement, rows: Row[], meta: Meta, params: URLSearchParams, update: (p: URLSearchParams) => void) {
  const m = params.get("m") ?? "";
  const tab = (params.get("tab") as Tab) || "short";
  const shown = Number(params.get("n") ?? PAGE) || PAGE;
  const set = (k: string, v: string) => {
    const p = new URLSearchParams(params);
    if (v) p.set(k, v);
    else p.delete(k);
    p.delete("n");
    update(p);
  };

  const scoped = applyFilter(rows, { ministry: m });
  const rated = withRates(scoped);
  const bins = histogram(rated);
  const totalRates = bins.reduce((a, b) => a + b.count, 0);
  const t = meta.thresholds;

  const tabs: { key: Tab; label: string; code: string; sort: ReturnType<typeof parseSort> }[] = [
    { key: "short", label: `未達（最小達成率 < ${t.OutcomeShortfall}%）`, code: "outcome_shortfall", sort: parseSort(null, "minRate", "asc") },
    { key: "over", label: `超過（最大達成率 > ${t.OutcomeOvershoot}%）`, code: "outcome_overshoot", sort: parseSort(null, "maxRate", "desc") },
    { key: "none", label: "実績なし（定量的アウトカムがあるのに達成率なし）", code: "no_outcome_actual", sort: parseSort(null, "initial", "desc") },
  ];
  const cur = tabs.find((x) => x.key === tab) ?? tabs[0]!;
  const list = sortRows(applyFilter(scoped, { signals: [cur.code] }), cur.sort.key, cur.sort.dir);

  const tabBar = h("div", { class: "tabs" });
  for (const x of tabs) {
    const n = applyFilter(scoped, { signals: [x.code] }).length;
    tabBar.appendChild(h("button", { type: "button", class: x.key === cur.key ? "on" : "", onclick: () => set("tab", x.key) }, `${x.label} `, h("span", { class: "n" }, String(n))));
  }

  const { tbody, more } = paged(
    list,
    (r) => {
      const mn = r.rates.length ? Math.min(...r.rates) : null;
      const mx = r.rates.length ? Math.max(...r.rates) : null;
      return h(
        "tr",
        {},
        nameCell(r, meta),
        h("td", { class: "num" }, String(r.outcomes)),
        h("td", { class: "num" }, ratePct(mn)),
        h("td", { class: "num" }, ratePct(mx)),
        h("td", { class: "rates muted" }, r.rates.map((v) => ratePct(v)).join(" / ")),
        yenCell(r.initial),
        h("td", { class: "refl" }, r.reflection),
      );
    },
    shown,
    () => {
      const p = new URLSearchParams(params);
      p.set("n", String(shown + PAGE));
      update(p);
    },
  );

  root.replaceChildren(
    h("h2", {}, "成果指標の達成状況", h("small", { class: "muted" }, ` FY${meta.actualYear} の定量的アウトカム達成率`)),
    h("div", { class: "controls" }, select("m", ministryOptions(rows, meta), m, (v) => set("m", v))),
    h("p", { class: "muted" }, `達成率をもつ事業 ${rated.length.toLocaleString("ja-JP")} / ${scoped.length.toLocaleString("ja-JP")}、指標 ${totalRates.toLocaleString("ja-JP")} 件`),
    h("div", { class: "histwrap" }, raw(histogramSvg(bins))),
    tabBar,
    summaryLine(list.length, scoped.length),
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
          h("th", { class: "num" }, "指標数"),
          h("th", { class: "num" }, "最小"),
          h("th", { class: "num" }, "最大"),
          h("th", {}, "達成率"),
          h("th", { class: "num" }, "② 当初"),
          h("th", {}, "⑥ 反映"),
        ),
      ),
      tbody,
    ),
    ...(more ? [more] : []),
  );
}
