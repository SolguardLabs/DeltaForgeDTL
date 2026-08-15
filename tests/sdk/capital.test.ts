import test from "node:test";
import assert from "node:assert/strict";
import {
  canonicalJson,
  evaluateCapital,
  mulDivCeil,
  mulDivFloor,
} from "../../sdk/deltaForgeClient.ts";

function input() {
  return {
    asset: "usd",
    group: "primary",
    reserve: 1_000_000n,
    liquidReserve: 800_000n,
    grossExpected: 900_000n,
    grossExecuted: 870_000n,
    openCorrections: 20_000n,
    globalPool: 10_000n,
    correctionCapacity: 500_000n,
    reserveHaircutBps: 500n,
    driftShockBps: 2_000n,
    operationalBufferBps: 800n,
  };
}

test("capital evaluation applies conservative reserve and demand rules", () => {
  const result = evaluateCapital(input());
  assert.equal(result.effectiveReserve, 950_000n);
  assert.equal(result.grossSettlementDemand, 60_000n);
  assert.equal(result.stressedSettlementDemand, 72_000n);
  assert.equal(result.requiredReserve, 144_000n);
  assert.equal(result.availableCapacity, 500_000n);
  assert.equal(result.compliant, true);
});

test("integer helpers round in the declared direction", () => {
  assert.equal(mulDivFloor(10n, 1n, 3n), 3n);
  assert.equal(mulDivCeil(10n, 1n, 3n), 4n);
  assert.throws(() => mulDivCeil(1n, 1n, 0n), /positive/);
});

test("capital evaluation requires liquid coverage", () => {
  const constrained = input();
  constrained.liquidReserve = 100_000n;
  const result = evaluateCapital(constrained);
  assert.equal(result.reserveShortfall, 44_000n);
  assert.equal(result.compliant, false);
});

test("canonical JSON preserves integer precision and key order", () => {
  assert.equal(
    canonicalJson({ z: 9_007_199_254_740_993n, a: "delta" }),
    '{"a":"delta","z":9007199254740993}',
  );
});
