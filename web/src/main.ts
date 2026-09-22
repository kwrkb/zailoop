// index.html に埋め込まれた JSON を読み、URL ハッシュでビューを切り替える。
import { decode, type Payload } from "./data.ts";
import { h } from "./dom.ts";
import { buildHash, parseHash } from "./state.ts";
import { renderGaps } from "./views/gaps.ts";
import { renderList } from "./views/list.ts";
import { renderLoops } from "./views/loops.ts";
import { renderOutcomes } from "./views/outcomes.ts";

function main() {
  const root = document.getElementById("app");
  const dataEl = document.getElementById("zailoop-data");
  if (!root || !dataEl) return;
  let payload: Payload;
  try {
    payload = JSON.parse(dataEl.textContent ?? "") as Payload;
  } catch (e) {
    root.replaceChildren(h("p", { class: "error" }, `データを読めません: ${String(e)}`));
    return;
  }
  const { meta, rows } = decode(payload);

  const render = () => {
    const { view, params } = parseHash(location.hash);
    const update = (p: URLSearchParams) => {
      const next = buildHash(view, p);
      if (next !== location.hash) history.replaceState(null, "", next);
      render();
    };
    for (const a of document.querySelectorAll<HTMLAnchorElement>("#nav a")) {
      a.classList.toggle("on", a.getAttribute("href") === `#${view}`);
    }
    // 検索欄のフォーカスを保つため、入力中は全体を描き直さず値だけ反映する
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

  let focusedName: string | null = null;
  let focusedPos = 0;
  root.addEventListener("input", (e) => {
    const t = e.target as HTMLInputElement;
    if (t.type === "search") {
      focusedName = "search";
      focusedPos = t.selectionStart ?? t.value.length;
    }
  });
  function restoreFocus(el: HTMLElement) {
    if (focusedName !== "search") return;
    const input = el.querySelector<HTMLInputElement>('input[type="search"]');
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
