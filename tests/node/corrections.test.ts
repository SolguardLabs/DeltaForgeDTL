import test from "node:test";
import assert from "node:assert/strict";
import { balance, byId, runFixture } from "../helpers/deltaforge.ts";

test("material differences produce direct correction records", () => {
  const report = runFixture("correction_cycle.json");

  assert.equal(report.reconciliation.correctionCount, 1);
  assert.equal(report.corrections.length, 1);
  assert.equal(report.corrections[0].account, "member-alpha");
  assert.equal(report.corrections[0].asset, "USD");
  assert.equal(report.corrections[0].amount, 820);
  assert.equal(report.corrections[0].status, "applied");
});

test("corrected account reaches the expected balance", () => {
  const report = runFixture("correction_cycle.json");
  const alpha = byId(report.accounts, "member-alpha");
  const usd = balance(alpha, "USD");

  assert.equal(usd.expected, 1400000);
  assert.equal(usd.executed, 1399180);
  assert.equal(usd.final, 1400000);
  assert.equal(report.settlement.allocationTotal, 10);
});
