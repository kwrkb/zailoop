import { VERDICT_LABEL, type Meta, type Row } from "../data.ts";
import { h } from "../dom.ts";
import { applyFilter, parseSort, sortRows } from "../filter.ts";
import { crosstab } from "../stats.ts";
import { withParams } from "../state.ts";
import { PAGE, badges, loopCell, ministryOptions, nameCell, paged, scroll, select, sortOptions, summaryLine, th, yenCell } from "./common.ts";

const ORDER = ["縮減", "廃止", "終了予定", "執行等改善", "年度内に改善を検討", "現状通り", "(空)"];

export function renderLoops(root: HTMLElement, rows: Row[], meta: Meta, params: URLSearchParams, update: (p: URLSearchParams) => void) {
  const m = params.get("m") ?? "";
  const refl = params.get("r") ?? "";
  const dir = params.get("d") ?? ""; // down / same / up
  const verdict = params.get("v") ?? ""; // 3 = 反映要確認 など
  const sort = parseSort(params.get("sort"), "verdict", "desc");
  const shown = Number(params.get("n") ?? PAGE) || PAGE;
  const set = (k: string, v: string) => setAll({ [k]: v });
  const setAll = (changes: Record<string, string | null>) => update(withParams(params, { ...changes, n: null }));
  const prevYear: number | null = meta.sheetYears.length > 1 ? (meta.sheetYears[meta.sheetYears.length - 2] ?? null) : null;

  if (prevYear === null) {
    root.replaceChildren(
      h("h2", {}, "ループ検証"),
      h("p", { class: "muted" }, "このサイトは 1 年度分のシートだけで生成されています。翌年のシートと突き合わせるには zailoop build --years 2024,2025 のように複数年度を指定してください。"),
    );
    return;
  }

  const scoped = applyFilter(rows, { ministry: m });
  const cells = crosstab(scoped, ORDER);
  const list = sortRows(
    scoped.filter((r) => {
      if (r.nextInitial === null || r.prevInitial === null) return false;
      if (refl && (r.prevReflection || "(空)") !== refl) return false;
      if (dir === "down" && !(r.nextInitial < r.prevInitial)) return false;
      if (dir === "same" && r.nextInitial !== r.prevInitial) return false;
      if (dir === "up" && !(r.nextInitial > r.prevInitial)) return false;
      if (verdict && String(r.verdict) !== verdict) return false;
      return true;
    }),
    sort.key,
    sort.dir,
  );

  const table = h("table", { class: "cross" });
  table.appendChild(h("thead", {}, h("tr", {}, h("th", {}, `${prevYear}年度シートの反映状況`), h("th", { class: "num" }, "減額"), h("th", { class: "num" }, "同額"), h("th", { class: "num" }, "増額"))));
  const tbody = h("tbody");
  for (const c of cells) {
    const cell = (d: "down" | "same" | "up", n: number) =>
      h("td", { class: `num ${refl === c.reflection && dir === d ? "on" : ""}` }, h("button", { type: "button", onclick: () => setAll({ r: c.reflection, d }) }, String(n)));
    const rowEl = h("tr", { class: refl === c.reflection ? "on" : "" }, h("th", {}, h("button", { type: "button", onclick: () => setAll({ r: c.reflection, d: null }) }, c.reflection)), cell("down", c.down), cell("same", c.same), cell("up", c.up));
    tbody.appendChild(rowEl);
  }
  table.appendChild(tbody);

  const controls = h(
    "div",
    { class: "controls" },
    select("m", ministryOptions(rows, meta), m, (v) => set("m", v)),
    select(
      "v",
      [
        { value: "", label: "判定: すべて" },
        { value: "3", label: "反映要確認（縮減・廃止・終了予定なのに増額）" },
        { value: "2", label: "整合" },
        { value: "1", label: "判定対象外" },
      ],
      verdict,
      (v) => set("v", v),
    ),
    select("sort", sortOptions([{ value: "verdict:desc", label: "反映要確認を上に" }, { value: "delta:desc", label: "増額が大きい順" }, { value: "delta:asc", label: "減額が大きい順" }]), `${sort.key}:${sort.dir}`, (v) => set("sort", v)),
    refl || dir || verdict ? h("button", { type: "button", onclick: () => setAll({ r: null, d: null, v: null }) }, "絞り込みを解除") : null,
  );

  const { tbody: listBody, more } = paged(
    list,
    (r) =>
      h(
        "tr",
        {},
        nameCell(r, meta),
        loopCell(r),
        yenCell(r.prevInitial),
        yenCell(r.nextInitial),
        h("td", { class: `verdict ${r.verdict === 3 ? "attn" : r.verdict === 2 ? "good" : "muted"}` }, VERDICT_LABEL[r.verdict] ?? ""),
        h("td", { class: "refl" }, r.reflection),
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
    h("h2", {}, "ループ検証", h("small", { class: "muted" }, ` ${prevYear}年度シートの「概算要求への反映状況」が、${prevYear + 1}年度シートの当初予算にどう現れたか`)),
    h("p", { class: "muted" }, `比較は FY${prevYear} 当初（${prevYear}年度シート）と FY${prevYear + 1} 当初（${prevYear + 1}年度シート）。縮減・廃止・終了予定だったのに翌年度の当初予算が増えた事業を「反映要確認」とします。事業の再編・移管などでも起こり、不正や無駄を示すものではありません。人が詳しく調べる候補を絞り込むための、このサイト独自の目印です。`),
    controls,
    table,
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
          h("th", {}, `${prevYear} 反映 → 当初`),
          h("th", { class: "num" }, `FY${prevYear} 当初`),
          h("th", { class: "num" }, `FY${prevYear + 1} 当初`),
          th("判定", meta, "ループ検証"),
          h("th", {}, `${prevYear + 1} 反映`),
          h("th", {}, "兆候"),
        ),
      ),
      listBody,
    ),
    ...(more ? [more] : []),
  );
}
