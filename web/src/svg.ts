// inline SVG を文字列で組み立てる純粋関数。値は 0〜1 の比率か生の数値配列。

const esc = (s: string) => s.replace(/&/g, "&amp;").replace(/</g, "&lt;").replace(/"/g, "&quot;");

/** 横バー。ratio は 0〜1（範囲外は丸める）。 */
export function bar(ratio: number | null, cls = ""): string {
  const w = ratio === null ? 0 : Math.max(0, Math.min(1, ratio)) * 100;
  return `<svg class="bar ${esc(cls)}" viewBox="0 0 100 10" preserveAspectRatio="none" aria-hidden="true"><rect x="0" y="1" height="8" width="${w.toFixed(1)}"></rect></svg>`;
}

/** スパークライン（棒）。null は空白。 */
export function sparkline(values: (number | null)[]): string {
  const vals = values.filter((v): v is number => v !== null && v >= 0);
  const max = vals.length ? Math.max(...vals) : 0;
  const n = values.length || 1;
  const w = 100 / n;
  const rects = values
    .map((v, i) => {
      if (v === null || max <= 0) return "";
      const h = (v / max) * 10;
      return `<rect x="${(i * w + 1).toFixed(1)}" y="${(10 - h).toFixed(1)}" width="${(w - 2).toFixed(1)}" height="${h.toFixed(1)}"></rect>`;
    })
    .join("");
  return `<svg class="spark" viewBox="0 0 100 10" preserveAspectRatio="none" aria-hidden="true">${rects}</svg>`;
}

/** ヒストグラム（縦棒）。 */
export function histogramSvg(bins: { label: string; count: number }[]): string {
  const max = Math.max(1, ...bins.map((b) => b.count));
  const n = bins.length || 1;
  const w = 100 / n;
  const bars = bins
    .map((b, i) => {
      const h = (b.count / max) * 40;
      const x = i * w;
      return (
        `<rect x="${(x + 2).toFixed(1)}" y="${(40 - h).toFixed(1)}" width="${(w - 4).toFixed(1)}" height="${h.toFixed(1)}"><title>${esc(b.label)}: ${b.count}</title></rect>` +
        `<text x="${(x + w / 2).toFixed(1)}" y="47" text-anchor="middle">${esc(b.label)}</text>` +
        `<text x="${(x + w / 2).toFixed(1)}" y="${Math.max(6, 40 - h - 2).toFixed(1)}" text-anchor="middle" class="count">${b.count}</text>`
      );
    })
    .join("");
  return `<svg class="hist" viewBox="0 0 100 50">${bars}</svg>`;
}
