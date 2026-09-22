import { test } from "node:test";
import assert from "node:assert/strict";
import { histogram, countSignals, crosstab } from "../src/stats.ts";
import type { Row } from "../src/data.ts";

const row = (o: Partial<Row>): Row => ({
  id: "1", name: "", ministry: "", category: "", reflection: "", request: null, initial: null, current: null, executed: null,
  execRate: null, execState: "", carriedOut: null, unused: null, unusedDiff: null, unusedState: "", reflected: null,
  nextInitial: null, nextRequest: null, byYear: [], outcomes: 0, rates: [], signals: 0, signalCodes: [], prevReflection: "", prevInitial: null, verdict: 0, renamed: false, ...o,
});

test("histogram bins are exhaustive and exclusive", () => {
  const bins = histogram([row({ rates: [0, 49.9, 50, 79.9, 80, 99.9, 100, 100.1, 120, 120.1, 200, 200.1, 1000] })]);
  assert.deepEqual(bins.map((b) => b.count), [2, 2, 2, 1, 2, 2, 2]);
  assert.equal(bins.reduce((a, b) => a + b.count, 0), 13);
});

test("countSignals", () => {
  const m = countSignals([row({ signalCodes: ["cut"] }), row({ signalCodes: ["cut", "low_execution"] })], ["cut", "low_execution", "x"]);
  assert.equal(m.get("cut"), 2);
  assert.equal(m.get("low_execution"), 1);
  assert.equal(m.get("x"), 0);
});

test("crosstab groups by previous reflection and delta direction", () => {
  const mk = (pr: string, pi: number | null, ni: number | null) => ({ ...row({}), prevReflection: pr, prevInitial: pi, nextInitial: ni });
  const cells = crosstab([mk("縮減", 100, 90), mk("縮減", 100, 100), mk("現状通り", 50, 60), mk("", 10, 5), mk("縮減", null, 5)], ["縮減", "廃止", "現状通り", "(空)"]);
  assert.deepEqual(cells.map((c) => [c.reflection, c.down, c.same, c.up]), [["縮減", 1, 1, 0], ["現状通り", 0, 0, 1], ["(空)", 1, 0, 0]]);
});
