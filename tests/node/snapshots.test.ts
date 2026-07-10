import test from "node:test";
import assert from "node:assert/strict";
import { byId, runFixture } from "../helpers/deltaforge.ts";

test("snapshot hash is stable for the same fixture", () => {
  const first = runFixture("snapshot_cycle.json");
  const second = runFixture("snapshot_cycle.json");

  assert.equal(first.snapshot.hash, second.snapshot.hash);
  assert.match(first.snapshot.hash, /^[a-f0-9]{64}$/);
  assert.equal(first.snapshot.accountCount, 3);
  assert.equal(first.snapshot.assetCount, 2);
  assert.ok(first.snapshot.canonicalLineCount >= 10);
});

test("snapshot totals include final settlement state", () => {
  const report = runFixture("snapshot_cycle.json");
  const usd = byId(report.assets, "USD");
  const eur = byId(report.assets, "EUR");

  assert.equal(usd.expectedTotal, 2250000);
  assert.equal(usd.executedTotal, 2249994);
  assert.equal(usd.finalTotal, 2250000);
  assert.equal(eur.expectedTotal, 1600000);
  assert.equal(eur.executedTotal, 1599992);
  assert.equal(eur.finalTotal, 1600000);
});
