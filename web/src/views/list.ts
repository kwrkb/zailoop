import type { Meta, Row } from "../data.ts";
import { h } from "../dom.ts";
import { applyFilter, parseSort, sortRows } from "../filter.ts";
import { PAGE, badges, loopCell, ministryOptions, nameCell, paged, rateCell, select, sortOptions, summaryLineWithStale, yenCell } from "./common.ts";

export function renderList(root: HTMLElement, rows: Row[], meta: Meta, params: URLSearchParams, update: (p: URLSearchParams) => void) {
  const q = params.get("q") ?? "";
  const m = params.get("m") ?? "";
  const c = params.get("c") ?? "";
  const sort = parseSort(params.get("sort"));
  const shown = Number(params.get("n") ?? PAGE) || PAGE;
  const multi = meta.sheetYears.length > 1;
  const set = (k: string, v: string) => {
    const p = new URLSearchParams(params);
    if (v) p.set(k, v);
    else p.delete(k);
    p.delete("n");
    update(p);
  };

  const filtered = sortRows(applyFilter(rows, { q, ministry: m, category: c }), sort.key, sort.dir);
  const n = meta.actualYear;

  const controls = h(
    "div",
    { class: "controls" },
    h("input", {
      type: "search",
      placeholder: "事業名 または 予算事業ID",
      value: q,
      oninput: (e) => set("q", (e.target as HTMLInputElement).value),
    }),
    select("m", ministryOptions(rows, meta), m, (v) => set("m", v)),
    select("c", [{ value: "", label: "事業区分: すべて" }, ...meta.categories.map((x) => ({ value: x, label: x }))], c, (v) => set("c", v)),
    select("sort", sortOptions(), `${sort.key}:${sort.dir}`, (v) => set("sort", v)),
  );

  const { tbody, more } = paged(
    filtered,
    (r) =>
      h(
        "tr",
        {},
        h("td", { class: "id" }, r.id),
        nameCell(r, meta),
        h("td", {}, r.ministry),
        h("td", { class: "muted" }, r.category),
        yenCell(r.request),
        yenCell(r.initial),
        yenCell(r.current),
        yenCell(r.executed),
        rateCell(r),
        yenCell(r.unused, r.unusedState ? "muted" : ""),
        h("td", {}, r.reflection),
        ...(multi ? [loopCell(r)] : []),
        badges(r, meta),
      ),
    shown,
    () => {
      const p = new URLSearchParams(params);
      p.set("n", String(shown + PAGE));
      update(p);
    },
  );

  root.replaceChildren(
    h("h2", {}, "一覧", h("small", { class: "muted" }, ` FY${n} を軸にした 6 段階の金額`)),
    controls,
    summaryLineWithStale(filtered.length, rows.length, filtered, meta),
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
          h("th", {}, "区分"),
          h("th", { class: "num" }, `① FY${n} 要求`),
          h("th", { class: "num" }, `② 当初`),
          h("th", { class: "num" }, `② 現額`),
          h("th", { class: "num" }, `③ 執行`),
          h("th", {}, "③ 執行率"),
          h("th", { class: "num" }, "④ 不用相当"),
          h("th", {}, "⑥ 反映"),
          ...(multi ? [h("th", {}, "前年反映→当初")] : []),
          h("th", {}, "兆候"),
        ),
      ),
      tbody,
    ),
    ...(more ? [more] : []),
  );
}
