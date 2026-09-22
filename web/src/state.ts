// URL ハッシュに状態を持つ。"#list?q=x&m=y" の形。file:// でも共有・戻るが効く。

export interface HashState {
  view: string;
  params: URLSearchParams;
}

export function parseHash(hash: string): HashState {
  const h = hash.startsWith("#") ? hash.slice(1) : hash;
  const [view, query = ""] = h.split("?");
  return { view: view || "list", params: new URLSearchParams(query) };
}

export function buildHash(view: string, params: URLSearchParams): string {
  const q = params.toString();
  return `#${view}${q ? "?" + q : ""}`;
}

/** 複数のキーを一度に更新した複製を返す。空文字・null は削除。 */
export function withParams(params: URLSearchParams, changes: Record<string, string | null>): URLSearchParams {
  const p = new URLSearchParams(params);
  for (const [k, v] of Object.entries(changes)) {
    if (v === null || v === "") p.delete(k);
    else p.set(k, v);
  }
  return p;
}
