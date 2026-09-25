"use strict";
(() => {
  // src/data.ts
  var VERDICT_LABEL = { 0: "\u4E0D\u660E", 1: "\u5224\u5B9A\u5BFE\u8C61\u5916", 2: "\u6574\u5408", 3: "\u53CD\u6620\u8981\u78BA\u8A8D" };
  var n = (v) => v === void 0 ? null : v;
  function decodeRow(r, meta) {
    const sg = r.sg ?? 0;
    return {
      id: r.id,
      name: r.n,
      ministry: meta.ministries[r.m] ?? "",
      category: meta.categories[r.c] ?? "",
      reflection: r.rf >= 0 ? meta.reflections[r.rf] ?? "" : "",
      request: n(r.rq),
      initial: n(r.in),
      supplementary: n(r.sp),
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
      prevReflection: r.pr && r.pr > 0 ? meta.reflections[r.pr - 1] ?? "" : "",
      prevInitial: n(r.pi),
      verdict: r.lv ?? 0,
      renamed: r.rn ?? false,
      sheetYear: r.sy ?? meta.sheetYear,
      actualYear: (r.sy ?? meta.sheetYear) - 1
    };
  }
  function decode(p) {
    return { meta: p.meta, rows: p.rows.map((r) => decodeRow(r, p.meta)) };
  }

  // src/dom.ts
  var RawHTML = class {
    html;
    constructor(html) {
      this.html = html;
    }
  };
  var raw = (html) => new RawHTML(html);
  function h(tag, attrs = {}, ...children) {
    const el = document.createElement(tag);
    for (const [k, v] of Object.entries(attrs)) {
      if (typeof v === "function") el.addEventListener(k.replace(/^on/, ""), v);
      else if (v === true) el.setAttribute(k, "");
      else if (v !== false) el.setAttribute(k, v);
    }
    append(el, children);
    return el;
  }
  function append(el, children) {
    for (const c of children) {
      if (c === null || c === void 0 || c === false) continue;
      if (Array.isArray(c)) append(el, c);
      else if (c instanceof RawHTML) el.insertAdjacentHTML("beforeend", c.html);
      else if (typeof c === "string") el.appendChild(document.createTextNode(c));
      else el.appendChild(c);
    }
  }

  // src/state.ts
  function parseHash(hash) {
    const h2 = hash.startsWith("#") ? hash.slice(1) : hash;
    const [view, query = ""] = h2.split("?");
    return { view: view || "list", params: new URLSearchParams(query) };
  }
  function buildHash(view, params) {
    const q = params.toString();
    return `#${view}${q ? "?" + q : ""}`;
  }
  function withParams(params, changes) {
    const p = new URLSearchParams(params);
    for (const [k, v] of Object.entries(changes)) {
      if (v === null || v === "") p.delete(k);
      else p.set(k, v);
    }
    return p;
  }

  // src/filter.ts
  function normalize(s) {
    return s.normalize("NFKC").toLowerCase().replace(/\s+/g, "");
  }
  function applyFilter(rows, f) {
    const q = f.q ? normalize(f.q) : "";
    const sigs = f.signals ?? [];
    return rows.filter((r) => {
      if (f.ministry && r.ministry !== f.ministry) return false;
      if (f.category && r.category !== f.category) return false;
      if (q && r.id !== q && !normalize(r.name).includes(q)) return false;
      if (sigs.length > 0) {
        const has = sigs.map((c) => r.signalCodes.includes(c));
        if (f.mode === "and" ? !has.every(Boolean) : !has.some(Boolean)) return false;
      }
      return true;
    });
  }
  var SORT_KEYS = ["id", "request", "initial", "current", "executed", "execRate", "unused", "nextRequest", "signals", "minRate", "maxRate", "verdict", "delta"];
  function initialDelta(r) {
    if (r.nextInitial === null || r.prevInitial === null) return null;
    return r.nextInitial - r.prevInitial;
  }
  function sortValue(r, key) {
    switch (key) {
      case "id":
        return Number(r.id);
      case "request":
        return r.request;
      case "initial":
        return r.initial;
      case "current":
        return r.current;
      case "executed":
        return r.executed;
      case "execRate":
        return r.execRate;
      case "unused":
        return r.unused;
      case "nextRequest":
        return r.nextRequest;
      case "signals":
        return r.signalCodes.length;
      case "minRate":
        return r.rates.length ? Math.min(...r.rates) : null;
      case "maxRate":
        return r.rates.length ? Math.max(...r.rates) : null;
      case "verdict":
        return r.verdict;
      case "delta":
        return initialDelta(r);
    }
  }
  function sortRows(rows, key, dir) {
    const sign = dir === "asc" ? 1 : -1;
    return rows.map((r, i) => ({ r, i, v: sortValue(r, key) })).sort((a, b) => {
      if (a.v === null && b.v === null) return a.i - b.i;
      if (a.v === null) return 1;
      if (b.v === null) return -1;
      return a.v === b.v ? a.i - b.i : (a.v - b.v) * sign;
    }).map((x) => x.r);
  }
  function parseSort(s, def = "id", defDir = "asc") {
    if (!s) return { key: def, dir: defDir };
    const [k, d] = s.split(":");
    const key = SORT_KEYS.includes(k) ? k : def;
    const dir = d === "asc" || d === "desc" ? d : defDir;
    return { key, dir };
  }

  // src/format.ts
  function yenFull(v) {
    if (v === null) return "\u2014";
    return `${v.toLocaleString("ja-JP")} \u5186`;
  }
  function yenShort(v) {
    if (v === null) return "\u2014";
    const neg = v < 0;
    const a = Math.abs(v);
    let s;
    if (a >= 1e12) s = `${trim(a / 1e12)} \u5146\u5186`;
    else if (a >= 1e8) s = `${trim(a / 1e8)} \u5104\u5186`;
    else if (a >= 1e4) s = `${trim(a / 1e4)} \u4E07\u5186`;
    else s = `${a} \u5186`;
    return neg ? `-${s}` : s;
  }
  function trim(x) {
    return x >= 100 ? Math.round(x).toLocaleString("ja-JP") : (Math.round(x * 10) / 10).toString();
  }
  function pct(v, digits = 1) {
    if (v === null) return "\u2014";
    return `${(v * 100).toFixed(digits)}%`;
  }
  function ratePct(v) {
    if (v === null) return "\u2014";
    return `${Math.round(v * 10) / 10}%`;
  }

  // src/stats.ts
  function rateBins() {
    return [
      { label: "<50", count: 0, test: (v) => v < 50 },
      { label: "50\u201380", count: 0, test: (v) => v >= 50 && v < 80 },
      { label: "80\u2013100", count: 0, test: (v) => v >= 80 && v < 100 },
      { label: "100", count: 0, test: (v) => v === 100 },
      { label: "100\u2013120", count: 0, test: (v) => v > 100 && v <= 120 },
      { label: "120\u2013200", count: 0, test: (v) => v > 120 && v <= 200 },
      { label: ">200", count: 0, test: (v) => v > 200 }
    ];
  }
  function histogram(rows) {
    const bins = rateBins();
    for (const r of rows) {
      for (const v of r.rates) {
        const b = bins.find((x) => x.test(v));
        if (b) b.count++;
      }
    }
    return bins;
  }
  function countSignals(rows, codes) {
    const m = new Map(codes.map((c) => [c, 0]));
    for (const r of rows) for (const c of r.signalCodes) m.set(c, (m.get(c) ?? 0) + 1);
    return m;
  }
  function withRates(rows) {
    return rows.filter((r) => r.rates.length > 0);
  }
  function crosstab(rows, order) {
    const m = /* @__PURE__ */ new Map();
    for (const r of rows) {
      if (r.nextInitial === null || r.prevInitial === null) continue;
      const k = r.prevReflection || "(\u7A7A)";
      const c = m.get(k) ?? { reflection: k, down: 0, same: 0, up: 0 };
      if (r.nextInitial < r.prevInitial) c.down++;
      else if (r.nextInitial > r.prevInitial) c.up++;
      else c.same++;
      m.set(k, c);
    }
    const keys = [...order.filter((k) => m.has(k)), ...[...m.keys()].filter((k) => !order.includes(k))];
    return keys.map((k) => m.get(k));
  }
  function overview(rows, sheetYear) {
    const o = { projects: rows.length, onAxis: 0, initial: 0, current: 0, executed: 0, rateExecuted: 0, rateCurrent: 0, execRate: null, withSignals: 0, contradictions: 0 };
    for (const r of rows) {
      if (r.signalCodes.length > 0) o.withSignals++;
      if (r.verdict === 3) o.contradictions++;
      if (r.sheetYear !== sheetYear) continue;
      o.onAxis++;
      o.initial += r.initial ?? 0;
      o.current += r.current ?? 0;
      o.executed += r.executed ?? 0;
      if (!r.execState && r.current !== null && r.current > 0 && r.executed !== null) {
        o.rateCurrent += r.current;
        o.rateExecuted += r.executed;
      }
    }
    o.execRate = o.rateCurrent > 0 ? o.rateExecuted / o.rateCurrent : null;
    return o;
  }
  function ministryTotals(rows, sheetYear) {
    const m = /* @__PURE__ */ new Map();
    for (const r of rows) {
      const t = m.get(r.ministry) ?? { ministry: r.ministry, count: 0, initial: 0, withSignals: 0 };
      t.count++;
      if (r.signalCodes.length > 0) t.withSignals++;
      if (r.sheetYear === sheetYear) t.initial += r.initial ?? 0;
      m.set(r.ministry, t);
    }
    return [...m.values()].sort((a, b) => b.initial - a.initial || b.count - a.count);
  }

  // src/columns.ts
  var AMOUNT_COLS = ["request", "initial", "current", "executed", "execRate", "unused", "nextRequest"];
  var DEFAULT_COLS = ["initial", "executed", "execRate"];
  function visibleColumns(showAll, sortKey) {
    if (showAll) return AMOUNT_COLS;
    return AMOUNT_COLS.filter((c) => DEFAULT_COLS.includes(c) || c === sortKey);
  }

  // src/svg.ts
  var esc = (s) => s.replace(/&/g, "&amp;").replace(/</g, "&lt;").replace(/"/g, "&quot;");
  function bar(ratio, cls = "") {
    const w = ratio === null ? 0 : Math.max(0, Math.min(1, ratio)) * 100;
    return `<svg class="bar ${esc(cls)}" viewBox="0 0 100 10" preserveAspectRatio="none" aria-hidden="true"><rect x="0" y="1" height="8" width="${w.toFixed(1)}"></rect></svg>`;
  }
  function sparkline(values) {
    const vals = values.filter((v) => v !== null && v >= 0);
    const max = vals.length ? Math.max(...vals) : 0;
    const n2 = values.length || 1;
    const w = 100 / n2;
    const rects = values.map((v, i) => {
      if (v === null || max <= 0) return "";
      const h2 = v / max * 10;
      return `<rect x="${(i * w + 1).toFixed(1)}" y="${(10 - h2).toFixed(1)}" width="${(w - 2).toFixed(1)}" height="${h2.toFixed(1)}"></rect>`;
    }).join("");
    return `<svg class="spark" viewBox="0 0 100 10" preserveAspectRatio="none" aria-hidden="true">${rects}</svg>`;
  }
  function histogramSvg(bins) {
    const max = Math.max(1, ...bins.map((b) => b.count));
    const n2 = bins.length || 1;
    const w = 100 / n2;
    const bars = bins.map((b, i) => {
      const h2 = b.count / max * 40;
      const x = i * w;
      return `<rect x="${(x + 2).toFixed(1)}" y="${(40 - h2).toFixed(1)}" width="${(w - 4).toFixed(1)}" height="${h2.toFixed(1)}"><title>${esc(b.label)}: ${b.count}</title></rect><text x="${(x + w / 2).toFixed(1)}" y="47" text-anchor="middle">${esc(b.label)}</text><text x="${(x + w / 2).toFixed(1)}" y="${Math.max(6, 40 - h2 - 2).toFixed(1)}" text-anchor="middle" class="count">${b.count}</text>`;
    }).join("");
    return `<svg class="hist" viewBox="0 0 100 50">${bars}</svg>`;
  }

  // src/views/common.ts
  var PAGE = 100;
  function isStale(r, meta) {
    return r.sheetYear < meta.sheetYear;
  }
  function nameCell(r, meta) {
    const top = h("div", { class: "nm" }, h("a", { href: `p/${r.id}.html` }, r.name));
    if (meta && isStale(r, meta)) {
      top.appendChild(h("span", { class: "stale", title: `${r.sheetYear}\u5E74\u5EA6\u30B7\u30FC\u30C8\u307E\u3067\u3002\u91D1\u984D\u306F FY${r.actualYear} \u8EF8` }, `${r.sheetYear}\u5E74\u5EA6\u307E\u3067`));
    }
    const sub = h("div", { class: "nm-sub" }, h("span", { class: "id" }, r.id), h("span", {}, r.ministry), r.category ? h("span", {}, r.category) : null, raw(sparkline(r.byYear)));
    return h("td", { class: "name" }, top, sub);
  }
  function scroll(tag, attrs, ...children) {
    return h("div", {}, h("p", { class: "scroll-hint" }, "\u8868\u306F\u6A2A\u306B\u30B9\u30AF\u30ED\u30FC\u30EB\u3067\u304D\u307E\u3059 \u2192"), h("div", { class: "tablewrap" }, h(tag, attrs, ...children)));
  }
  function th(label, meta, term = "", cls = "") {
    const t = term ? meta.terms.find((x) => x.name === term) : void 0;
    return h("th", cls ? { class: cls } : {}, t ? h("abbr", { title: t.desc }, label) : label);
  }
  function amountHead(c, meta) {
    const n2 = meta.actualYear;
    switch (c) {
      case "request":
        return th(`\u2460 FY${n2} \u8981\u6C42`, meta, "\u6982\u7B97\u8981\u6C42", "num");
      case "initial":
        return th("\u2461 \u5F53\u521D", meta, "\u5F53\u521D\u4E88\u7B97", "num");
      case "current":
        return th("\u2461 \u73FE\u984D", meta, "\u6B73\u51FA\u4E88\u7B97\u73FE\u984D", "num");
      case "executed":
        return th("\u2462 \u57F7\u884C", meta, "\u57F7\u884C\u984D", "num");
      case "execRate":
        return th("\u2462 \u57F7\u884C\u7387", meta, "\u57F7\u884C\u7387");
      case "unused":
        return th("\u2463 \u4E0D\u7528\u76F8\u5F53", meta, "\u4E0D\u7528\u76F8\u5F53\u984D", "num");
      case "nextRequest":
        return th(`\u2465 FY${n2 + 2} \u8981\u6C42`, meta, "\u6982\u7B97\u8981\u6C42", "num");
    }
  }
  function amountCell(c, r) {
    switch (c) {
      case "request":
        return yenCell(r.request);
      case "initial":
        return initialCell(r);
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
  function detailToggle(showAll, onchange) {
    return h(
      "label",
      { class: "toggle" },
      h("input", { type: "checkbox", checked: showAll, onchange: (e) => onchange(e.target.checked) }),
      " \u8A73\u7D30\u5217\uFF08\u8981\u6C42\u30FB\u73FE\u984D\u30FB\u4E0D\u7528\u76F8\u5F53\u30FB\u6B21\u5E74\u5EA6\u8981\u6C42\uFF09"
    );
  }
  function initialCell(r) {
    const td = yenCell(r.initial);
    if (r.initial === 0 && r.supplementary !== null) {
      td.appendChild(h("span", { class: "sub block", title: yenFull(r.supplementary) }, `\u88DC\u6B63 ${yenShort(r.supplementary)}`));
    }
    return td;
  }
  function yenCell(v, cls = "") {
    return h("td", { class: `num ${cls}`, title: yenFull(v) }, yenShort(v));
  }
  function summaryLineWithStale(n2, total, rows, meta) {
    const stale = rows.filter((r) => isStale(r, meta)).length;
    const el = summaryLine(n2, total);
    if (stale > 0) el.appendChild(h("span", { class: "muted" }, `\uFF08\u3046\u3061 ${stale.toLocaleString("ja-JP")} \u4EF6\u306F ${meta.sheetYear} \u5E74\u5EA6\u30B7\u30FC\u30C8\u304C\u306A\u304F\u3001\u5B9F\u7E3E\u5E74\u5EA6\u304C 1 \u5E74\u53E4\u3044\uFF09`));
    return el;
  }
  function rateCell(r) {
    if (r.execState) return h("td", { class: "rate muted" }, r.execState);
    return h("td", { class: "rate" }, raw(bar(r.execRate)), h("span", {}, pct(r.execRate)));
  }
  function badges(r, meta) {
    const td = h("td", { class: "badges" });
    for (const s of meta.signals) {
      if (r.signalCodes.includes(s.code)) td.appendChild(h("span", { class: "badge", title: s.description }, s.label));
    }
    return td;
  }
  function paged(rows, renderRow, shown, onMore) {
    const tbody = h("tbody");
    for (const r of rows.slice(0, shown)) tbody.appendChild(renderRow(r));
    const more = rows.length > shown ? h("p", { class: "more" }, h("button", { type: "button", onclick: onMore }, `\u3055\u3089\u306B\u8868\u793A\uFF08\u6B8B\u308A ${rows.length - shown} \u4EF6\uFF09`)) : null;
    return { tbody, more };
  }
  function select(name, options, value, onchange) {
    const sel = h("select", { name, onchange: (e) => onchange(e.target.value) });
    for (const o of options) {
      const opt = h("option", { value: o.value }, o.label);
      if (o.value === value) opt.selected = true;
      sel.appendChild(opt);
    }
    return sel;
  }
  function ministryOptions(rows, meta) {
    const counts = /* @__PURE__ */ new Map();
    for (const r of rows) counts.set(r.ministry, (counts.get(r.ministry) ?? 0) + 1);
    return [{ value: "", label: "\u5E9C\u7701\u5E81: \u3059\u3079\u3066" }, ...meta.ministries.map((m) => ({ value: m, label: `${m} (${counts.get(m) ?? 0})` }))];
  }
  function sortOptions(extra = []) {
    return [
      { value: "id:asc", label: "ID \u9806" },
      { value: "initial:desc", label: "\u5F53\u521D\u4E88\u7B97 \u591A\u3044\u9806" },
      { value: "current:desc", label: "\u73FE\u984D \u591A\u3044\u9806" },
      { value: "executed:desc", label: "\u57F7\u884C\u984D \u591A\u3044\u9806" },
      { value: "execRate:asc", label: "\u57F7\u884C\u7387 \u4F4E\u3044\u9806" },
      { value: "execRate:desc", label: "\u57F7\u884C\u7387 \u9AD8\u3044\u9806" },
      { value: "unused:desc", label: "\u4E0D\u7528\u76F8\u5F53\u984D \u591A\u3044\u9806" },
      { value: "request:desc", label: "\u6982\u7B97\u8981\u6C42 \u591A\u3044\u9806" },
      { value: "nextRequest:desc", label: "\u6B21\u5E74\u5EA6\u8981\u6C42 \u591A\u3044\u9806" },
      ...extra
    ];
  }
  function summaryLine(n2, total) {
    return h("p", { class: "count muted" }, `${n2.toLocaleString("ja-JP")} / ${total.toLocaleString("ja-JP")} \u4E8B\u696D`);
  }
  function loopCell(r) {
    const d = initialDelta(r);
    if (!r.prevReflection && d === null) return h("td", { class: "muted" }, "\u2014");
    const cls = r.verdict === 3 ? "attn" : r.verdict === 2 ? "good" : "";
    const arrow = d === null ? "" : d < 0 ? "\u2193" : d > 0 ? "\u2191" : "\u2192";
    return h(
      "td",
      { class: `loop ${cls}`, title: d === null ? "" : `\u7FCC\u5E74\u5F53\u521D\u306E\u5897\u6E1B ${yenFull(d)}\uFF08${VERDICT_LABEL[r.verdict] ?? ""}\uFF09` },
      r.prevReflection || "(\u7A7A)",
      " ",
      h("span", { class: "arrow" }, arrow, d === null ? "" : ` ${yenShort(Math.abs(d))}`)
    );
  }

  // src/views/gaps.ts
  function thresholdText(code, meta) {
    const t = meta.thresholds;
    switch (code) {
      case "request_gap":
        return `\u5F53\u521D/\u8981\u6C42 < ${t.RequestGapRatio}`;
      case "low_execution":
        return `\u57F7\u884C\u7387 < ${t.LowExecRate}`;
      case "large_unused":
        return `\u4E0D\u7528\u76F8\u5F53\u984D \u2265 ${yenFull(t.LargeUnusedYen)} \u304B\u3064 \u4E0D\u7528/\u73FE\u984D \u2265 ${t.LargeUnusedRatio}`;
      case "outcome_shortfall":
        return `\u9054\u6210\u7387\u306E\u6700\u5C0F < ${t.OutcomeShortfall}%`;
      case "outcome_overshoot":
        return `\u9054\u6210\u7387\u306E\u6700\u5927 > ${t.OutcomeOvershoot}%`;
      case "amount_revised":
        return `\u6539\u8A02\u306E\u5909\u5316\u7387 \u2265 ${t.RevisionRatio}`;
      default:
        return "";
    }
  }
  function renderGaps(root, rows, meta, params, update) {
    const selected = (params.get("sig") ?? "").split(",").filter(Boolean);
    const mode = params.get("mode") === "and" ? "and" : "or";
    const m = params.get("m") ?? "";
    const sort = parseSort(params.get("sort"), "signals", "desc");
    const shown = Number(params.get("n") ?? PAGE) || PAGE;
    const showAll = params.get("cols") === "all";
    const cols = visibleColumns(showAll, sort.key);
    const multi = meta.sheetYears.length > 1;
    const set = (k, v) => {
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
            }
          }),
          h("span", { class: "label" }, s.label),
          h("span", { class: "n" }, String(counts.get(s.code) ?? 0)),
          h("span", { class: "th muted" }, thresholdText(s.code, meta))
        )
      );
    }
    const controls = h(
      "div",
      { class: "controls" },
      select("m", ministryOptions(rows, meta), m, (v) => set("m", v)),
      select(
        "mode",
        [
          { value: "or", label: "\u3044\u305A\u308C\u304B\u306B\u8A72\u5F53\uFF08OR\uFF09" },
          { value: "and", label: "\u3059\u3079\u3066\u306B\u8A72\u5F53\uFF08AND\uFF09" }
        ],
        mode,
        (v) => set("mode", v)
      ),
      select("sort", sortOptions([{ value: "signals:desc", label: "\u5146\u5019\u306E\u6570 \u591A\u3044\u9806" }]), `${sort.key}:${sort.dir}`, (v) => set("sort", v)),
      detailToggle(showAll, (v) => set("cols", v ? "all" : "")),
      selected.length ? h("button", { type: "button", onclick: () => set("sig", "") }, "\u9078\u629E\u3092\u89E3\u9664") : null
    );
    const { tbody, more } = paged(
      filtered,
      (r) => h(
        "tr",
        {},
        nameCell(r, meta),
        badges(r, meta),
        ...cols.map((col) => amountCell(col, r)),
        h("td", { class: "refl" }, r.reflection),
        ...multi ? [loopCell(r)] : []
      ),
      shown,
      () => {
        const p = new URLSearchParams(params);
        p.set("n", String(shown + PAGE));
        update(p);
      }
    );
    root.replaceChildren(
      h("h2", {}, "\u30EB\u30FC\u30D7\u306E\u65AD\u7D76\u3092\u63A2\u3059", h("small", { class: "muted" }, " \u8981\u6C42\u2192\u6210\u7ACB\u2192\u57F7\u884C\u2192\u6C7A\u7B97\u2192\u8A55\u4FA1\u2192\u53CD\u6620\u306E\u3069\u3053\u304B\u3067\u6574\u5408\u3057\u306A\u3044\u4E8B\u696D")),
      h("p", { class: "muted" }, "\u3053\u306E\u30B5\u30A4\u30C8\u72EC\u81EA\u306E\u5224\u5B9A\u3067\u3001\u516C\u5F0F\u306E\u8A55\u4FA1\u3067\u306F\u3042\u308A\u307E\u305B\u3093\u3002\u5224\u5B9A\u306F\u751F\u6210\u6642\u306B\u884C\u3063\u3066\u3044\u3066\u3001\u95BE\u5024\u306F zailoop build \u306E\u5F15\u6570\u3067\u5909\u3048\u3089\u308C\u307E\u3059\u3002"),
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
            h("th", {}, "\u4E8B\u696D\u540D"),
            th("\u5146\u5019", meta, "\u5146\u5019"),
            ...cols.map((col) => amountHead(col, meta)),
            th("\u2465 \u53CD\u6620", meta, "\u53CD\u6620\u72B6\u6CC1"),
            ...multi ? [th("\u524D\u5E74\u53CD\u6620\u2192\u5F53\u521D", meta, "\u30EB\u30FC\u30D7\u691C\u8A3C")] : []
          )
        ),
        tbody
      ),
      ...more ? [more] : []
    );
  }

  // src/views/overview.ts
  var TOP = 10;
  function renderOverview(rows, meta, params, update) {
    const o = overview(rows, meta.sheetYear);
    const n2 = meta.actualYear;
    const multi = meta.sheetYears.length > 1;
    const cur = params.get("m") ?? "";
    const all = params.get("mall") === "1";
    const set = (changes) => {
      const p = new URLSearchParams(params);
      for (const [k, v] of Object.entries(changes)) {
        if (v) p.set(k, v);
        else p.delete(k);
      }
      p.delete("n");
      update(p);
    };
    const num = (v) => v.toLocaleString("ja-JP");
    const stat = (label, value, note, href = "", title = "") => h(href ? "a" : "div", { class: "stat", ...href ? { href } : {}, ...title ? { title } : {} }, h("span", { class: "label" }, label), h("strong", { class: "value" }, value), h("span", { class: "note" }, note));
    const stats = h(
      "div",
      { class: "stats" },
      stat("\u4E8B\u696D\u6570", num(o.projects), o.onAxis < o.projects ? `\u91D1\u984D\u306E\u5408\u8A08\u306F FY${n2} \u8EF8\u306E ${num(o.onAxis)} \u4E8B\u696D` : `${meta.sheetYear}\u5E74\u5EA6\u30B7\u30FC\u30C8`),
      stat(`FY${n2} \u5F53\u521D\u4E88\u7B97`, yenShort(o.initial), "\u4E8B\u696D\u3054\u3068\u306E\u5024\u306E\u5358\u7D14\u5408\u8A08\uFF08\u56FD\u306E\u4E88\u7B97\u7DCF\u984D\u3068\u306F\u4E00\u81F4\u3057\u306A\u3044\uFF09", "", yenFull(o.initial)),
      stat(`FY${n2} \u57F7\u884C\u7387`, pct(o.execRate), `\u57F7\u884C ${yenShort(o.rateExecuted)} \uFF0F \u73FE\u984D ${yenShort(o.rateCurrent)}\uFF08\u73FE\u984D 0 \u4EE5\u4E0B\u3092\u9664\u304F\uFF09`, "", "\u57F7\u884C\u984D\u306E\u5408\u8A08 \xF7 \u6B73\u51FA\u4E88\u7B97\u73FE\u984D\u306E\u5408\u8A08\u3002\u57F7\u884C\u304C\u78BA\u5B9A\u3057\u3001\u6B73\u51FA\u4E88\u7B97\u73FE\u984D\u304C\u6B63\u306E\u4E8B\u696D\u3060\u3051\u3067\u8A08\u7B97"),
      stat("\u5146\u5019\u306E\u3042\u308B\u4E8B\u696D", num(o.withSignals), `\u5168\u4F53\u306E ${pct(o.projects ? o.withSignals / o.projects : null, 0)}\u3002\u3053\u306E\u30B5\u30A4\u30C8\u72EC\u81EA\u306E\u5224\u5B9A \u2192 \u65AD\u7D76\u3092\u63A2\u3059`, "#gaps"),
      multi ? stat("\u30EB\u30FC\u30D7\u691C\u8A3C\u3067\u300C\u53CD\u6620\u8981\u78BA\u8A8D\u300D", num(o.contradictions), "\u7E2E\u6E1B\u7B49\u306E\u65B9\u91DD\u306A\u306E\u306B\u7FCC\u5E74\u5EA6\u5897\u984D\u3002\u3053\u306E\u30B5\u30A4\u30C8\u72EC\u81EA\u306E\u76EE\u5370 \u2192 \u30EB\u30FC\u30D7\u691C\u8A3C", "#loops?v=3") : null
    );
    const totals = ministryTotals(rows, meta.sheetYear);
    const max = Math.max(1, ...totals.map((t) => t.initial));
    const shown = all ? totals : totals.filter((t, i) => i < TOP || t.ministry === cur);
    const list = h("div", { class: "ministries" });
    for (const t of shown) {
      const on = t.ministry === cur;
      list.appendChild(
        h(
          "button",
          { type: "button", class: `ministry ${on ? "on" : ""}`, title: `${t.ministry} \u3067\u4E00\u89A7\u3092\u7D5E\u308A\u8FBC\u3080`, onclick: () => set({ m: on ? null : t.ministry }) },
          h("span", { class: "mname" }, t.ministry),
          raw(bar(t.initial / max)),
          h("span", { class: "mval", title: yenFull(t.initial) }, yenShort(t.initial)),
          h("span", { class: "msig" }, `${pct(o.initial ? t.initial / o.initial : null)}\u30FB${num(t.count)} \u4E8B\u696D\u30FB\u5146\u5019 ${num(t.withSignals)}`)
        )
      );
    }
    const more = totals.length > TOP ? h("button", { type: "button", class: "linklike", onclick: () => set({ mall: all ? null : "1" }) }, all ? "\u4E0A\u4F4D\u3060\u3051\u8868\u793A" : `\u6B8B\u308A ${totals.length - shown.length} \u5E9C\u7701\u5E81\u3082\u8868\u793A`) : null;
    return h(
      "section",
      { class: "overview" },
      stats,
      h("h3", {}, `\u5E9C\u7701\u5E81\u5225\u306E FY${n2} \u5F53\u521D\u4E88\u7B97`, h("small", { class: "muted" }, " \u62BC\u3059\u3068\u4E00\u89A7\u3092\u305D\u306E\u5E9C\u7701\u5E81\u3067\u7D5E\u308A\u8FBC\u307F\u307E\u3059")),
      h("p", { class: "note muted" }, "\u4E00\u822C\u4F1A\u8A08\u3068\u7279\u5225\u4F1A\u8A08\u306E\u4E8B\u696D\u3092\u5358\u7D14\u306B\u8DB3\u3057\u305F\u5024\u3067\u3059\u3002\u4F1A\u8A08\u9593\u306E\u7E70\u5165\u308C\u3067\u91CD\u8907\u3057\u3046\u308B\u305F\u3081\u3001\u56FD\u306E\u4E88\u7B97\u7DCF\u984D\u3068\u306F\u4E00\u81F4\u3057\u307E\u305B\u3093\u3002\u68D2\u306E\u9577\u3055\u306F\u91D1\u984D\u306B\u6BD4\u4F8B\u3057\u307E\u3059\u3002\u300C\u5146\u5019\u300D\u306E\u4EF6\u6570\u306F\u3053\u306E\u30B5\u30A4\u30C8\u72EC\u81EA\u306E\u5224\u5B9A\u3067\u3001\u516C\u5F0F\u306E\u8A55\u4FA1\u3067\u306F\u3042\u308A\u307E\u305B\u3093\u3002"),
      list,
      more
    );
  }

  // src/views/list.ts
  function renderList(root, rows, meta, params, update) {
    const q = params.get("q") ?? "";
    const m = params.get("m") ?? "";
    const c = params.get("c") ?? "";
    const sort = parseSort(params.get("sort"));
    const shown = Number(params.get("n") ?? PAGE) || PAGE;
    const showAll = params.get("cols") === "all";
    const cols = visibleColumns(showAll, sort.key);
    const multi = meta.sheetYears.length > 1;
    const set = (k, v) => {
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
        placeholder: "\u4E8B\u696D\u540D \u307E\u305F\u306F \u4E88\u7B97\u4E8B\u696DID",
        value: q,
        oninput: (e) => set("q", e.target.value)
      }),
      select("m", ministryOptions(rows, meta), m, (v) => set("m", v)),
      select("c", [{ value: "", label: "\u4E8B\u696D\u533A\u5206: \u3059\u3079\u3066" }, ...meta.categories.map((x) => ({ value: x, label: x }))], c, (v) => set("c", v)),
      select("sort", sortOptions(), `${sort.key}:${sort.dir}`, (v) => set("sort", v)),
      detailToggle(showAll, (v) => set("cols", v ? "all" : ""))
    );
    const { tbody, more } = paged(
      filtered,
      (r) => h(
        "tr",
        {},
        nameCell(r, meta),
        ...cols.map((col) => amountCell(col, r)),
        h("td", { class: "refl" }, r.reflection),
        ...multi ? [loopCell(r)] : [],
        badges(r, meta)
      ),
      shown,
      () => {
        const p = new URLSearchParams(params);
        p.set("n", String(shown + PAGE));
        update(p);
      }
    );
    root.replaceChildren(
      renderOverview(rows, meta, params, update),
      h("h2", {}, "\u4E00\u89A7", h("small", { class: "muted" }, ` FY${meta.actualYear} \u3092\u8EF8\u306B\u3057\u305F\u91D1\u984D\u3002\u898B\u51FA\u3057\u306E\u70B9\u7DDA\u306F\u7528\u8A9E\u306E\u8AAC\u660E`)),
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
            h("th", {}, "\u4E8B\u696D\u540D"),
            ...cols.map((col) => amountHead(col, meta)),
            th("\u2465 \u53CD\u6620", meta, "\u53CD\u6620\u72B6\u6CC1"),
            ...multi ? [th("\u524D\u5E74\u53CD\u6620\u2192\u5F53\u521D", meta, "\u30EB\u30FC\u30D7\u691C\u8A3C")] : [],
            th("\u5146\u5019", meta, "\u5146\u5019")
          )
        ),
        tbody
      ),
      ...more ? [more] : []
    );
  }

  // src/views/loops.ts
  var ORDER = ["\u7E2E\u6E1B", "\u5EC3\u6B62", "\u7D42\u4E86\u4E88\u5B9A", "\u57F7\u884C\u7B49\u6539\u5584", "\u5E74\u5EA6\u5185\u306B\u6539\u5584\u3092\u691C\u8A0E", "\u73FE\u72B6\u901A\u308A", "(\u7A7A)"];
  function renderLoops(root, rows, meta, params, update) {
    const m = params.get("m") ?? "";
    const refl = params.get("r") ?? "";
    const dir = params.get("d") ?? "";
    const verdict = params.get("v") ?? "";
    const sort = parseSort(params.get("sort"), "verdict", "desc");
    const shown = Number(params.get("n") ?? PAGE) || PAGE;
    const set = (k, v) => setAll({ [k]: v });
    const setAll = (changes) => update(withParams(params, { ...changes, n: null }));
    const prevYear = meta.sheetYears.length > 1 ? meta.sheetYears[meta.sheetYears.length - 2] ?? null : null;
    if (prevYear === null) {
      root.replaceChildren(
        h("h2", {}, "\u30EB\u30FC\u30D7\u691C\u8A3C"),
        h("p", { class: "muted" }, "\u3053\u306E\u30B5\u30A4\u30C8\u306F 1 \u5E74\u5EA6\u5206\u306E\u30B7\u30FC\u30C8\u3060\u3051\u3067\u751F\u6210\u3055\u308C\u3066\u3044\u307E\u3059\u3002\u7FCC\u5E74\u306E\u30B7\u30FC\u30C8\u3068\u7A81\u304D\u5408\u308F\u305B\u308B\u306B\u306F zailoop build --years 2024,2025 \u306E\u3088\u3046\u306B\u8907\u6570\u5E74\u5EA6\u3092\u6307\u5B9A\u3057\u3066\u304F\u3060\u3055\u3044\u3002")
      );
      return;
    }
    const scoped = applyFilter(rows, { ministry: m });
    const cells = crosstab(scoped, ORDER);
    const list = sortRows(
      scoped.filter((r) => {
        if (r.nextInitial === null || r.prevInitial === null) return false;
        if (refl && (r.prevReflection || "(\u7A7A)") !== refl) return false;
        if (dir === "down" && !(r.nextInitial < r.prevInitial)) return false;
        if (dir === "same" && r.nextInitial !== r.prevInitial) return false;
        if (dir === "up" && !(r.nextInitial > r.prevInitial)) return false;
        if (verdict && String(r.verdict) !== verdict) return false;
        return true;
      }),
      sort.key,
      sort.dir
    );
    const table = h("table", { class: "cross" });
    table.appendChild(h("thead", {}, h("tr", {}, h("th", {}, `${prevYear}\u5E74\u5EA6\u30B7\u30FC\u30C8\u306E\u53CD\u6620\u72B6\u6CC1`), h("th", { class: "num" }, "\u6E1B\u984D"), h("th", { class: "num" }, "\u540C\u984D"), h("th", { class: "num" }, "\u5897\u984D"))));
    const tbody = h("tbody");
    for (const c of cells) {
      const cell = (d, n2) => h("td", { class: `num ${refl === c.reflection && dir === d ? "on" : ""}` }, h("button", { type: "button", onclick: () => setAll({ r: c.reflection, d }) }, String(n2)));
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
          { value: "", label: "\u5224\u5B9A: \u3059\u3079\u3066" },
          { value: "3", label: "\u53CD\u6620\u8981\u78BA\u8A8D\uFF08\u7E2E\u6E1B\u30FB\u5EC3\u6B62\u30FB\u7D42\u4E86\u4E88\u5B9A\u306A\u306E\u306B\u5897\u984D\uFF09" },
          { value: "2", label: "\u6574\u5408" },
          { value: "1", label: "\u5224\u5B9A\u5BFE\u8C61\u5916" }
        ],
        verdict,
        (v) => set("v", v)
      ),
      select("sort", sortOptions([{ value: "verdict:desc", label: "\u53CD\u6620\u8981\u78BA\u8A8D\u3092\u4E0A\u306B" }, { value: "delta:desc", label: "\u5897\u984D\u304C\u5927\u304D\u3044\u9806" }, { value: "delta:asc", label: "\u6E1B\u984D\u304C\u5927\u304D\u3044\u9806" }]), `${sort.key}:${sort.dir}`, (v) => set("sort", v)),
      refl || dir || verdict ? h("button", { type: "button", onclick: () => setAll({ r: null, d: null, v: null }) }, "\u7D5E\u308A\u8FBC\u307F\u3092\u89E3\u9664") : null
    );
    const { tbody: listBody, more } = paged(
      list,
      (r) => h(
        "tr",
        {},
        nameCell(r, meta),
        loopCell(r),
        yenCell(r.prevInitial),
        yenCell(r.nextInitial),
        h("td", { class: `verdict ${r.verdict === 3 ? "attn" : r.verdict === 2 ? "good" : "muted"}` }, VERDICT_LABEL[r.verdict] ?? ""),
        h("td", { class: "refl" }, r.reflection),
        badges(r, meta)
      ),
      shown,
      () => {
        const p = new URLSearchParams(params);
        p.set("n", String(shown + PAGE));
        update(p);
      }
    );
    root.replaceChildren(
      h("h2", {}, "\u30EB\u30FC\u30D7\u691C\u8A3C", h("small", { class: "muted" }, ` ${prevYear}\u5E74\u5EA6\u30B7\u30FC\u30C8\u306E\u300C\u6982\u7B97\u8981\u6C42\u3078\u306E\u53CD\u6620\u72B6\u6CC1\u300D\u304C\u3001${prevYear + 1}\u5E74\u5EA6\u30B7\u30FC\u30C8\u306E\u5F53\u521D\u4E88\u7B97\u306B\u3069\u3046\u73FE\u308C\u305F\u304B`)),
      h("p", { class: "muted" }, `\u6BD4\u8F03\u306F FY${prevYear} \u5F53\u521D\uFF08${prevYear}\u5E74\u5EA6\u30B7\u30FC\u30C8\uFF09\u3068 FY${prevYear + 1} \u5F53\u521D\uFF08${prevYear + 1}\u5E74\u5EA6\u30B7\u30FC\u30C8\uFF09\u3002\u7E2E\u6E1B\u30FB\u5EC3\u6B62\u30FB\u7D42\u4E86\u4E88\u5B9A\u3060\u3063\u305F\u306E\u306B\u7FCC\u5E74\u5EA6\u306E\u5F53\u521D\u4E88\u7B97\u304C\u5897\u3048\u305F\u4E8B\u696D\u3092\u300C\u53CD\u6620\u8981\u78BA\u8A8D\u300D\u3068\u3057\u307E\u3059\u3002\u4E8B\u696D\u306E\u518D\u7DE8\u30FB\u79FB\u7BA1\u306A\u3069\u3067\u3082\u8D77\u3053\u308A\u3001\u4E0D\u6B63\u3084\u7121\u99C4\u3092\u793A\u3059\u3082\u306E\u3067\u306F\u3042\u308A\u307E\u305B\u3093\u3002\u4EBA\u304C\u8A73\u3057\u304F\u8ABF\u3079\u308B\u5019\u88DC\u3092\u7D5E\u308A\u8FBC\u3080\u305F\u3081\u306E\u3001\u3053\u306E\u30B5\u30A4\u30C8\u72EC\u81EA\u306E\u76EE\u5370\u3067\u3059\u3002`),
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
            h("th", {}, "\u4E8B\u696D\u540D"),
            h("th", {}, `${prevYear} \u53CD\u6620 \u2192 \u5F53\u521D`),
            h("th", { class: "num" }, `FY${prevYear} \u5F53\u521D`),
            h("th", { class: "num" }, `FY${prevYear + 1} \u5F53\u521D`),
            th("\u5224\u5B9A", meta, "\u30EB\u30FC\u30D7\u691C\u8A3C"),
            h("th", {}, `${prevYear + 1} \u53CD\u6620`),
            h("th", {}, "\u5146\u5019")
          )
        ),
        listBody
      ),
      ...more ? [more] : []
    );
  }

  // src/views/outcomes.ts
  function renderOutcomes(root, rows, meta, params, update) {
    const m = params.get("m") ?? "";
    const tab = params.get("tab") || "short";
    const shown = Number(params.get("n") ?? PAGE) || PAGE;
    const set = (k, v) => {
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
    const tabs = [
      { key: "short", label: `\u672A\u9054\uFF08\u6700\u5C0F\u9054\u6210\u7387 < ${t.OutcomeShortfall}%\uFF09`, code: "outcome_shortfall", sort: parseSort(null, "minRate", "asc") },
      { key: "over", label: `\u8D85\u904E\uFF08\u6700\u5927\u9054\u6210\u7387 > ${t.OutcomeOvershoot}%\uFF09`, code: "outcome_overshoot", sort: parseSort(null, "maxRate", "desc") },
      { key: "none", label: "\u5B9F\u7E3E\u306A\u3057\uFF08\u5B9A\u91CF\u7684\u30A2\u30A6\u30C8\u30AB\u30E0\u304C\u3042\u308B\u306E\u306B\u9054\u6210\u7387\u306A\u3057\uFF09", code: "no_outcome_actual", sort: parseSort(null, "initial", "desc") }
    ];
    const cur = tabs.find((x) => x.key === tab) ?? tabs[0];
    const list = sortRows(applyFilter(scoped, { signals: [cur.code] }), cur.sort.key, cur.sort.dir);
    const tabBar = h("div", { class: "tabs" });
    for (const x of tabs) {
      const n2 = applyFilter(scoped, { signals: [x.code] }).length;
      tabBar.appendChild(h("button", { type: "button", class: x.key === cur.key ? "on" : "", onclick: () => set("tab", x.key) }, `${x.label} `, h("span", { class: "n" }, String(n2))));
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
          initialCell(r),
          h("td", { class: "refl" }, r.reflection)
        );
      },
      shown,
      () => {
        const p = new URLSearchParams(params);
        p.set("n", String(shown + PAGE));
        update(p);
      }
    );
    root.replaceChildren(
      h("h2", {}, "\u6210\u679C\u6307\u6A19\u306E\u9054\u6210\u72B6\u6CC1", h("small", { class: "muted" }, ` FY${meta.actualYear} \u306E\u5B9A\u91CF\u7684\u30A2\u30A6\u30C8\u30AB\u30E0\u9054\u6210\u7387`)),
      h("div", { class: "controls" }, select("m", ministryOptions(rows, meta), m, (v) => set("m", v))),
      h("p", { class: "muted" }, `\u9054\u6210\u7387\u3092\u3082\u3064\u4E8B\u696D ${rated.length.toLocaleString("ja-JP")} / ${scoped.length.toLocaleString("ja-JP")}\u3001\u6307\u6A19 ${totalRates.toLocaleString("ja-JP")} \u4EF6`),
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
            h("th", {}, "\u4E8B\u696D\u540D"),
            h("th", { class: "num" }, "\u6307\u6A19\u6570"),
            h("th", { class: "num" }, "\u6700\u5C0F"),
            h("th", { class: "num" }, "\u6700\u5927"),
            h("th", {}, "\u9054\u6210\u7387"),
            h("th", { class: "num" }, "\u2461 \u5F53\u521D"),
            h("th", {}, "\u2465 \u53CD\u6620")
          )
        ),
        tbody
      ),
      ...more ? [more] : []
    );
  }

  // src/main.ts
  function main() {
    const root = document.getElementById("app");
    const dataEl = document.getElementById("zailoop-data");
    if (!root || !dataEl) return;
    let payload;
    try {
      payload = JSON.parse(dataEl.textContent ?? "");
    } catch (e) {
      root.replaceChildren(h("p", { class: "error" }, `\u30C7\u30FC\u30BF\u3092\u8AAD\u3081\u307E\u305B\u3093: ${String(e)}`));
      return;
    }
    const { meta, rows } = decode(payload);
    const render = () => {
      const { view, params } = parseHash(location.hash);
      const update = (p) => {
        const next = buildHash(view, p);
        if (next !== location.hash) history.replaceState(null, "", next);
        render();
      };
      for (const a of document.querySelectorAll("#nav a")) {
        a.classList.toggle("on", a.getAttribute("href") === `#${view}`);
      }
      switch (view) {
        case "gaps":
          renderGaps(root, rows, meta, params, update);
          break;
        case "outcomes":
          renderOutcomes(root, rows, meta, params, update);
          break;
        case "loops":
          renderLoops(root, rows, meta, params, update);
          break;
        default:
          renderList(root, rows, meta, params, update);
      }
      restoreFocus(root);
    };
    let focusedName = null;
    let focusedPos = 0;
    root.addEventListener("input", (e) => {
      const t = e.target;
      if (t.type === "search") {
        focusedName = "search";
        focusedPos = t.selectionStart ?? t.value.length;
      }
    });
    function restoreFocus(el) {
      if (focusedName !== "search") return;
      const input = el.querySelector('input[type="search"]');
      if (input) {
        input.focus();
        input.setSelectionRange(focusedPos, focusedPos);
      }
      focusedName = null;
    }
    window.addEventListener("hashchange", render);
    render();
  }
  main();
})();
