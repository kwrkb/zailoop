import { test } from "node:test";
import assert from "node:assert/strict";
import { visibleColumns, AMOUNT_COLS } from "../src/columns.ts";

test("visibleColumns default hides detail columns", () => {
  assert.deepEqual(visibleColumns(false, "id"), ["initial", "executed", "execRate"]);
  assert.deepEqual(visibleColumns(false, "signals"), ["initial", "executed", "execRate"]);
});

test("visibleColumns shows the sorted column in order", () => {
  assert.deepEqual(visibleColumns(false, "unused"), ["initial", "executed", "execRate", "unused"]);
  assert.deepEqual(visibleColumns(false, "request"), ["request", "initial", "executed", "execRate"]);
  assert.deepEqual(visibleColumns(false, "nextRequest"), ["initial", "executed", "execRate", "nextRequest"]);
  assert.deepEqual(visibleColumns(false, "initial"), ["initial", "executed", "execRate"]);
});

test("visibleColumns showAll", () => {
  assert.deepEqual(visibleColumns(true, "id"), AMOUNT_COLS);
});
