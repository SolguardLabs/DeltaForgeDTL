import test from "node:test";
import assert from "node:assert/strict";
import { runCli, runFixture, validateFixture } from "../helpers/deltaforge.ts";

test("fixtures validate through the public CLI", () => {
  assert.equal(validateFixture("snapshot_cycle.json"), "ok");
  assert.equal(validateFixture("reconciliation_cycle.json"), "ok");
  assert.equal(validateFixture("correction_cycle.json"), "ok");
  assert.equal(validateFixture("settlement_cycle.json"), "ok");
});

test("scenario listing is deterministic", () => {
  const names = runCli(["--list"]).trim().split(/\r?\n/);
  assert.deepEqual(names, ["baseline", "correction-window", "multi-asset", "settlement-floor"]);
});

test("event output is opt-in", () => {
  const withoutEvents = runFixture("reconciliation_cycle.json");
  const withEvents = runFixture("reconciliation_cycle.json", ["--events"]);

  assert.equal(withoutEvents.events, undefined);
  assert.ok(withEvents.events);
  assert.equal(withEvents.events[0].type, "init");
  assert.equal(withEvents.events.at(-1)?.type, "allocation");
});
