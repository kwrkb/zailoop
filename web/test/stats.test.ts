import { test } from "node:test";
import assert from "node:assert/strict";
import { histogram, countSignals, crosstab, overview, ministryTotals } from "../src/stats.ts";
import type { Row } from "../src/data.ts";

const row = (o: Partial<Row>): Row => ({
  id: "1", name: "", ministry: "", category: "", reflection: "", request: null, initial: null, supplementary: null, current: null, executed: null,
  execRate: null, execState: "", carriedOut: null, unused: null, unusedDiff: null, unusedState: "", reflected: null,
  nextInitial: null, nextRequest: null, byYear: [], outcomes: 0, rates: [], signals: 0, signalCodes: [], prevReflection: "", prevInitial: null, verdict: 0, renamed: false, sheetYear: 2024, actualYear: 2023, ...o,
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

test("overview sums only rows on the base sheet year", () => {
  const rows = [
    row({ sheetYear: 2025, initial: 100, current: 120, executed: 90, signalCodes: ["cut"], verdict: 3 }),
    row({ sheetYear: 2025, initial: 50, current: 0, executed: 30, execState: "算出対象外" }),
    row({ sheetYear: 2025, initial: null, current: null, executed: null }),
    row({ sheetYear: 2024, initial: 1000, current: 1000, executed: 10, signalCodes: ["low_execution"] }),
  ];
  const o = overview(rows, 2025);
  assert.deepEqual({ ...o, execRate: null }, { projects: 4, onAxis: 3, initial: 150, current: 120, executed: 120, rateExecuted: 90, rateCurrent: 120, execRate: null, withSignals: 2, contradictions: 1 });
  assert.equal(o.execRate, 90 / 120);
  assert.equal(overview([row({ sheetYear: 2025 })], 2025).execRate, null);
});

test("ministryTotals orders by initial and counts signals", () => {
  const rows = [
    row({ ministry: "A", sheetYear: 2025, initial: 10 }),
    row({ ministry: "B", sheetYear: 2025, initial: 30, signalCodes: ["cut"] }),
    row({ ministry: "A", sheetYear: 2025, initial: 5, signalCodes: ["cut"] }),
    row({ ministry: "A", sheetYear: 2024, initial: 100 }),
  ];
  assert.deepEqual(ministryTotals(rows, 2025), [
    { ministry: "B", count: 1, initial: 30, withSignals: 1 },
    { ministry: "A", count: 3, initial: 15, withSignals: 1 },
  ]);
});
