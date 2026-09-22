import { test } from "node:test";
import assert from "node:assert/strict";
import { normalize, applyFilter, sortRows, parseSort, initialDelta } from "../src/filter.ts";
import type { Row } from "../src/data.ts";

const row = (o: Partial<Row>): Row => ({
  id: "1", name: "", ministry: "", category: "", reflection: "", request: null, initial: null, current: null, executed: null,
  execRate: null, execState: "", carriedOut: null, unused: null, unusedDiff: null, unusedState: "", reflected: null,
  nextInitial: null, nextRequest: null, byYear: [], outcomes: 0, rates: [], signals: 0, signalCodes: [], prevReflection: "", prevInitial: null, verdict: 0, renamed: false, sheetYear: 2024, actualYear: 2023, ...o,
});

test("normalize folds width and case", () => {
  assert.equal(normalize("ＡＢＣ 法教育"), "abc法教育");
  assert.equal(normalize("ｶﾞ"), "ガ");
});

test("applyFilter by query, ministry, signals", () => {
  const rows = [
    row({ id: "884", name: "法教育の推進", ministry: "法務省", signalCodes: ["cut"] }),
    row({ id: "11", name: "政府共通プラットフォーム", ministry: "デジタル庁", signalCodes: ["cut", "low_execution"] }),
  ];
  assert.deepEqual(applyFilter(rows, { q: "法教育" }).map((r) => r.id), ["884"]);
  assert.deepEqual(applyFilter(rows, { q: "11" }).map((r) => r.id), ["11"]);
  assert.deepEqual(applyFilter(rows, { ministry: "法務省" }).map((r) => r.id), ["884"]);
  assert.equal(applyFilter(rows, { signals: ["cut", "low_execution"], mode: "or" }).length, 2);
  assert.deepEqual(applyFilter(rows, { signals: ["cut", "low_execution"], mode: "and" }).map((r) => r.id), ["11"]);
});

test("sortRows puts null last and is stable", () => {
  const rows = [row({ id: "1", initial: 5 }), row({ id: "2", initial: null }), row({ id: "3", initial: 10 }), row({ id: "4", initial: 5 })];
  assert.deepEqual(sortRows(rows, "initial", "desc").map((r) => r.id), ["3", "1", "4", "2"]);
  assert.deepEqual(sortRows(rows, "initial", "asc").map((r) => r.id), ["1", "4", "3", "2"]);
  assert.deepEqual(sortRows([row({ id: "1", rates: [50, 150] })], "minRate", "asc")[0]?.rates, [50, 150]);
});

test("parseSort falls back on bad input", () => {
  assert.deepEqual(parseSort("initial:desc"), { key: "initial", dir: "desc" });
  assert.deepEqual(parseSort("bogus:up"), { key: "id", dir: "asc" });
  assert.deepEqual(parseSort(null, "execRate", "asc"), { key: "execRate", dir: "asc" });
});

test("initialDelta and delta sort", () => {
  const a = row({ id: "a", nextInitial: 120, prevInitial: 100 });
  const b = row({ id: "b", nextInitial: 80, prevInitial: 100 });
  const c = row({ id: "c", nextInitial: 80, prevInitial: null });
  assert.equal(initialDelta(a), 20);
  assert.equal(initialDelta(c), null);
  assert.deepEqual(sortRows([c, b, a], "delta", "desc").map((r) => r.id), ["a", "b", "c"]);
});
