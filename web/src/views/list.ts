import type { Meta, Row } from "../data.ts";
import { h } from "../dom.ts";
import { applyFilter, parseSort, sortRows } from "../filter.ts";
import { visibleColumns } from "../columns.ts";
import { PAGE, amountCell, amountHead, badges, detailToggle, loopCell, ministryOptions, nameCell, paged, select, sortOptions, scroll, summaryLineWithStale, th } from "./common.ts";

export function renderList(root: HTMLElement, rows: Row[], meta: Meta, params: URLSearchParams, update: (p: URLSearchParams) => void) {
  const q = params.get("q") ?? "";
  const m = params.get("m") ?? "";
  const c = params.get("c") ?? "";
  const sort = parseSort(params.get("sort"));
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

  const filtered = sortRows(applyFilter(rows, { q, ministry: m, category: c }), sort.key, sort.dir);

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
    detailToggle(showAll, (v) => set("cols", v ? "all" : "")),
  );

  const { tbody, more } = paged(
    filtered,
    (r) =>
      h(
        "tr",
        {},
        nameCell(r, meta),
        ...cols.map((col) => amountCell(col, r)),
        h("td", { class: "refl" }, r.reflection),
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
    h("h2", {}, "一覧", h("small", { class: "muted" }, ` FY${meta.actualYear} を軸にした金額。見出しの点線は用語の説明`)),
    controls,
    summaryLineWithStale(filtered.length, rows.length, filtered, meta),
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
          ...cols.map((col) => amountHead(col, meta)),
          th("⑥ 反映", meta, "反映状況"),
          ...(multi ? [th("前年反映→当初", meta, "ループ検証")] : []),
          th("兆候", meta, "兆候"),
        ),
      ),
      tbody,
    ),
    ...(more ? [more] : []),
  );
}
