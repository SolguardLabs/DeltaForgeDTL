import test from "node:test";
import assert from "node:assert/strict";
import { balance, byId, runFixture, runScenario } from "../helpers/deltaforge.ts";

test("final settlement applies configured account floors", () => {
  const report = runFixture("settlement_cycle.json");
  const floor = byId(report.accounts, "member-floor");
  const floorUsd = balance(floor, "USD");

  assert.equal(report.settlement.allocationTotal, 5);
  assert.equal(report.settlement.liquidationTotal, 10);
  assert.equal(report.settlement.liquidations.length, 1);
  assert.equal(report.settlement.liquidations[0].account, "member-floor");
  assert.equal(floorUsd.final, 100);
});

test("built-in scenario output matches the public report shape", () => {
  const report = runScenario("baseline");

  assert.equal(report.name, "baseline");
  assert.equal(report.book, "df-main");
  assert.ok(report.assets.length >= 2);
  assert.ok(report.accounts.length >= 4);
  assert.match(report.snapshot.hash, /^[a-f0-9]{64}$/);
});
