import { test } from "node:test";
import assert from "node:assert/strict";
import { parseHash, buildHash, withParams } from "../src/state.ts";

test("parseHash / buildHash roundtrip", () => {
  const s = parseHash("#list?q=%E6%B3%95&m=%E6%B3%95%E5%8B%99%E7%9C%81");
  assert.equal(s.view, "list");
  assert.equal(s.params.get("q"), "法");
  assert.equal(buildHash(s.view, s.params), "#list?q=%E6%B3%95&m=%E6%B3%95%E5%8B%99%E7%9C%81");
  assert.equal(parseHash("").view, "list");
  assert.equal(buildHash("gaps", new URLSearchParams()), "#gaps");
});

test("withParams updates several keys at once and deletes empties", () => {
  const p = withParams(new URLSearchParams("r=a&d=up&n=200"), { r: "縮減", d: "up", n: null });
  assert.equal(p.get("r"), "縮減");
  assert.equal(p.get("d"), "up");
  assert.equal(p.get("n"), null);
  assert.equal(withParams(p, { d: "" }).get("d"), null);
});
