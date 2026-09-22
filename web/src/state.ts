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
