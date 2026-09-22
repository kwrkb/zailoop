// 小さな DOM ヘルパ。文字列は textContent として入れる（HTML として解釈しない）。
// SVG など自前で組み立てた HTML 文字列だけ raw() で入れる。

export type Child = Node | string | RawHTML | null | undefined | false | Child[];

export class RawHTML {
  html: string;
  constructor(html: string) {
    this.html = html;
  }
}

export const raw = (html: string): RawHTML => new RawHTML(html);

type Attr = string | boolean | ((e: Event) => void);

export function h(tag: string, attrs: Record<string, Attr> = {}, ...children: Child[]): HTMLElement {
  const el = document.createElement(tag);
  for (const [k, v] of Object.entries(attrs)) {
    if (typeof v === "function") el.addEventListener(k.replace(/^on/, ""), v);
    else if (v === true) el.setAttribute(k, "");
    else if (v !== false) el.setAttribute(k, v);
  }
  append(el, children);
  return el;
}

function append(el: HTMLElement, children: Child[]) {
  for (const c of children) {
    if (c === null || c === undefined || c === false) continue;
    if (Array.isArray(c)) append(el, c);
    else if (c instanceof RawHTML) el.insertAdjacentHTML("beforeend", c.html);
    else if (typeof c === "string") el.appendChild(document.createTextNode(c));
    else el.appendChild(c);
  }
}
