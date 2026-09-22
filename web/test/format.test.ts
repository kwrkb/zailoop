import { test } from "node:test";
import assert from "node:assert/strict";
import { yenShort, yenFull, pct, ratePct } from "../src/format.ts";

test("yenShort", () => {
  assert.equal(yenShort(null), "—");
  assert.equal(yenShort(999), "999 円");
  assert.equal(yenShort(35_000), "3.5 万円");
  assert.equal(yenShort(40_217_000), "4,022 万円");
  assert.equal(yenShort(1_200_000_000), "12 億円");
  assert.equal(yenShort(677_292_895_000), "6,773 億円");
  assert.equal(yenShort(1_500_000_000_000), "1.5 兆円");
  assert.equal(yenShort(-569_000), "-56.9 万円");
});

test("yenFull / pct / ratePct", () => {
  assert.equal(yenFull(40217000), "40,217,000 円");
  assert.equal(pct(0.70699), "70.7%");
  assert.equal(pct(null), "—");
  assert.equal(ratePct(177.33), "177.3%");
});
