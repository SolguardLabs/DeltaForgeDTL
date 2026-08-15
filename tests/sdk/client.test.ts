import test from "node:test";
import assert from "node:assert/strict";
import { DeltaForgeClient } from "../../sdk/deltaForgeClient.ts";

function response(payload: unknown, status = 200) {
  return new Response(JSON.stringify(payload), {
    status,
    headers: { "content-type": "application/json; charset=utf-8" },
  });
}

test("client submits canonical snapshots with idempotency", async () => {
  let observed: { url: string; options: RequestInit } | undefined;
  const client = new DeltaForgeClient({
    baseUrl: "https://api.deltaforge.example/settlement",
    fetchImpl: async (url, options) => {
      observed = { url: url.toString(), options };
      return response({ cycleId: "cycle:7001", status: "accepted" }, 202);
    },
  });
  const result = await client.submitSnapshot(
    { book: "df-main", epoch: 7001, total: 9_007_199_254_740_993n },
    { idempotencyKey: "snapshot-20260815-0001" },
  );
  assert.equal(result.status, "accepted");
  assert.equal(observed?.url, "https://api.deltaforge.example/settlement/v1/snapshots");
  assert.equal(
    (observed?.options.headers as Record<string, string>)["Idempotency-Key"],
    "snapshot-20260815-0001",
  );
  assert.match(String(observed?.options.body), /9007199254740993/);
});

test("client rejects insecure endpoints and malformed keys", async () => {
  assert.throws(() => new DeltaForgeClient({ baseUrl: "http://api.example" }), /HTTPS/);
  const client = new DeltaForgeClient({
    baseUrl: "https://api.example",
    fetchImpl: async () => response({}),
  });
  await assert.rejects(client.reconcile("cycle:7001", { idempotencyKey: "short" }), /idempotency/);
});

test("client exposes structured service errors", async () => {
  const client = new DeltaForgeClient({
    baseUrl: "https://api.example",
    fetchImpl: async () =>
      response({ code: "CYCLE_NOT_READY", message: "cycle is not ready" }, 409),
  });
  await assert.rejects(
    client.reconcile("cycle:7001", { idempotencyKey: "reconcile-20260815-0001" }),
    (error: Error & { status?: number; code?: string }) =>
      error.status === 409 && error.code === "CYCLE_NOT_READY",
  );
});
