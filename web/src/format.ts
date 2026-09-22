// 金額・比率の表示。一覧では「1.2 億円」のように短く、title 属性には全桁を入れる。

/** 円を 3 桁区切りの全桁で表す。 */
export function yenFull(v: number | null): string {
  if (v === null) return "—";
  return `${v.toLocaleString("ja-JP")} 円`;
}

/** 円を億・万で短く表す。 */
export function yenShort(v: number | null): string {
  if (v === null) return "—";
  const neg = v < 0;
  const a = Math.abs(v);
  let s: string;
  if (a >= 1e12) s = `${trim(a / 1e12)} 兆円`;
  else if (a >= 1e8) s = `${trim(a / 1e8)} 億円`;
  else if (a >= 1e4) s = `${trim(a / 1e4)} 万円`;
  else s = `${a} 円`;
  return neg ? `-${s}` : s;
}

function trim(x: number): string {
  // 3 桁以上なら整数、それ未満なら小数 1 桁
  return x >= 100 ? Math.round(x).toLocaleString("ja-JP") : (Math.round(x * 10) / 10).toString();
}

/** 0〜1 の比率を百分率で表す。 */
export function pct(v: number | null, digits = 1): string {
  if (v === null) return "—";
  return `${(v * 100).toFixed(digits)}%`;
}

/** 達成率（%）を表す。 */
export function ratePct(v: number | null): string {
  if (v === null) return "—";
  return `${Math.round(v * 10) / 10}%`;
}
