import { test } from "node:test";
import assert from "node:assert/strict";
import { parseHash, buildHash } from "../src/state.ts";

test("parseHash / buildHash roundtrip", () => {
  const s = parseHash("#list?q=%E6%B3%95&m=%E6%B3%95%E5%8B%99%E7%9C%81");
  assert.equal(s.view, "list");
  assert.equal(s.params.get("q"), "法");
  assert.equal(buildHash(s.view, s.params), "#list?q=%E6%B3%95&m=%E6%B3%95%E5%8B%99%E7%9C%81");
  assert.equal(parseHash("").view, "list");
  assert.equal(buildHash("gaps", new URLSearchParams()), "#gaps");
});
