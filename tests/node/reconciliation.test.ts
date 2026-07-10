import test from "node:test";
import assert from "node:assert/strict";
import { byId, runFixture } from "../helpers/deltaforge.ts";

test("reconciliation classifies expected operational differences", () => {
  const report = runFixture("reconciliation_cycle.json");

  assert.equal(report.reconciliation.microCount, 3);
  assert.equal(report.reconciliation.correctionCount, 0);
  assert.equal(report.reconciliation.blockedCount, 0);
  assert.equal(report.reconciliation.microTotal, 19);
  assert.equal(report.reconciliation.globalPools.length, 2);

  const usdPool = report.reconciliation.globalPools.find((pool) => pool.asset === "USD");
  const eurPool = report.reconciliation.globalPools.find((pool) => pool.asset === "EUR");
  assert.equal(usdPool?.amount, 15);
  assert.equal(usdPool?.entries, 2);
  assert.equal(eurPool?.amount, 4);
  assert.equal(eurPool?.entries, 1);
});

test("reconciliation leaves asset totals balanced after settlement", () => {
  const report = runFixture("reconciliation_cycle.json");
  const usd = byId(report.assets, "USD");
  const eur = byId(report.assets, "EUR");

  assert.equal(usd.finalTotal - usd.executedTotal, usd.drift);
  assert.equal(eur.finalTotal - eur.executedTotal, eur.drift);
  assert.ok(report.reconciliation.balancedByAsset.every((entry) => entry.balanced));
  assert.equal(report.settlement.roundingRemainder, 0);
});
