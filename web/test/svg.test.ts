import { test } from "node:test";
import assert from "node:assert/strict";
import { bar, sparkline, histogramSvg } from "../src/svg.ts";

test("bar clamps ratio", () => {
  assert.match(bar(0.5), /width="50\.0"/);
  assert.match(bar(2), /width="100\.0"/);
  assert.match(bar(null), /width="0\.0"/);
});

test("sparkline skips nulls and scales to max", () => {
  const s = sparkline([10, null, 20]);
  assert.equal((s.match(/<rect/g) ?? []).length, 2);
  assert.match(s, /height="10\.0"/);
});

test("histogramSvg escapes labels", () => {
  const s = histogramSvg([{ label: "<50", count: 3 }]);
  assert.match(s, /&lt;50/);
  assert.doesNotMatch(s, /<50/);
});
