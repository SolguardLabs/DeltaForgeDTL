const BPS = 10_000n;
const IDENTIFIER = /^[a-z0-9][a-z0-9:_-]{2,127}$/;
const IDEMPOTENCY_KEY = /^[A-Za-z0-9._:-]{16,128}$/;

export type CapitalInput = {
  asset: string;
  group: string;
  reserve: bigint;
  liquidReserve: bigint;
  grossExpected: bigint;
  grossExecuted: bigint;
  openCorrections: bigint;
  globalPool: bigint;
  correctionCapacity: bigint;
  reserveHaircutBps: bigint;
  driftShockBps: bigint;
  operationalBufferBps: bigint;
};

function requireBigInt(name: string, value: bigint): bigint {
  if (typeof value !== "bigint" || value < 0n) {
    throw new TypeError(`${name} must be a non-negative bigint`);
  }
  return value;
}

function requireIdentifier(name: string, value: string): string {
  if (typeof value !== "string" || !IDENTIFIER.test(value)) {
    throw new TypeError(`${name} is not normalized`);
  }
  return value;
}

export function mulDivFloor(value: bigint, multiplier: bigint, denominator: bigint): bigint {
  requireBigInt("value", value);
  requireBigInt("multiplier", multiplier);
  requireBigInt("denominator", denominator);
  if (denominator === 0n) throw new RangeError("denominator must be positive");
  return (value * multiplier) / denominator;
}

export function mulDivCeil(value: bigint, multiplier: bigint, denominator: bigint): bigint {
  requireBigInt("value", value);
  requireBigInt("multiplier", multiplier);
  requireBigInt("denominator", denominator);
  if (denominator === 0n) throw new RangeError("denominator must be positive");
  if (value === 0n || multiplier === 0n) return 0n;
  return (value * multiplier + denominator - 1n) / denominator;
}

export function evaluateCapital(input: CapitalInput) {
  const asset = requireIdentifier("asset", input.asset);
  const group = requireIdentifier("group", input.group);
  const reserve = requireBigInt("reserve", input.reserve);
  const liquidReserve = requireBigInt("liquidReserve", input.liquidReserve);
  const grossExpected = requireBigInt("grossExpected", input.grossExpected);
  const grossExecuted = requireBigInt("grossExecuted", input.grossExecuted);
  const openCorrections = requireBigInt("openCorrections", input.openCorrections);
  const globalPool = requireBigInt("globalPool", input.globalPool);
  const correctionCapacity = requireBigInt("correctionCapacity", input.correctionCapacity);
  const reserveHaircutBps = requireBigInt("reserveHaircutBps", input.reserveHaircutBps);
  const driftShockBps = requireBigInt("driftShockBps", input.driftShockBps);
  const operationalBufferBps = requireBigInt("operationalBufferBps", input.operationalBufferBps);
  if (liquidReserve > reserve) throw new RangeError("liquid reserve exceeds reserve");
  if (reserveHaircutBps > BPS) throw new RangeError("reserve haircut exceeds 10000 bps");

  const observedDrift =
    grossExpected >= grossExecuted ? grossExpected - grossExecuted : grossExecuted - grossExpected;
  const grossSettlementDemand = observedDrift + globalPool + openCorrections;
  const stressedSettlementDemand = mulDivCeil(grossSettlementDemand, BPS + driftShockBps, BPS);
  const effectiveReserve = mulDivFloor(reserve, BPS - reserveHaircutBps, BPS);
  const operationalBuffer = mulDivCeil(grossExpected, operationalBufferBps, BPS);
  const requiredReserve = stressedSettlementDemand + operationalBuffer;
  const availableReserve = effectiveReserve < liquidReserve ? effectiveReserve : liquidReserve;
  const reserveShortfall =
    requiredReserve > availableReserve ? requiredReserve - availableReserve : 0n;
  const capacityHeadroom =
    availableReserve > requiredReserve ? availableReserve - requiredReserve : 0n;
  const availableCapacity =
    correctionCapacity < capacityHeadroom ? correctionCapacity : capacityHeadroom;

  return Object.freeze({
    asset,
    group,
    observedDrift,
    grossSettlementDemand,
    stressedSettlementDemand,
    effectiveReserve,
    operationalBuffer,
    requiredReserve,
    availableReserve,
    reserveShortfall,
    coverageBps: requiredReserve === 0n ? 0n : mulDivFloor(effectiveReserve, BPS, requiredReserve),
    liquidityBps: reserve === 0n ? 0n : mulDivFloor(liquidReserve, BPS, reserve),
    capacityUtilizationBps:
      correctionCapacity === 0n ? 0n : mulDivFloor(grossSettlementDemand, BPS, correctionCapacity),
    availableCapacity,
    compliant: reserveShortfall === 0n,
  });
}

export function canonicalJson(value: unknown): string {
  if (typeof value === "bigint") return value.toString();
  if (value === null || typeof value === "boolean" || typeof value === "number") {
    if (typeof value === "number" && !Number.isSafeInteger(value)) {
      throw new TypeError("numbers must be safe integers");
    }
    return JSON.stringify(value);
  }
  if (typeof value === "string") return JSON.stringify(value);
  if (Array.isArray(value)) return `[${value.map(canonicalJson).join(",")}]`;
  if (typeof value === "object") {
    const record = value as Record<string, unknown>;
    return `{${Object.keys(record)
      .sort()
      .filter((key) => record[key] !== undefined)
      .map((key) => `${JSON.stringify(key)}:${canonicalJson(record[key])}`)
      .join(",")}}`;
  }
  throw new TypeError("value cannot be represented canonically");
}

export class DeltaForgeClient {
  #baseUrl: URL;
  #fetch: typeof fetch;
  #timeoutMs: number;

  constructor({
    baseUrl,
    fetchImpl = globalThis.fetch,
    timeoutMs = 8_000,
  }: {
    baseUrl: string;
    fetchImpl?: typeof fetch;
    timeoutMs?: number;
  }) {
    const parsed = new URL(baseUrl);
    if (parsed.protocol !== "https:") throw new TypeError("baseUrl must use HTTPS");
    if (parsed.username || parsed.password || parsed.search || parsed.hash) {
      throw new TypeError("baseUrl must not contain credentials, query, or fragment");
    }
    if (typeof fetchImpl !== "function") throw new TypeError("fetch implementation is required");
    if (!Number.isSafeInteger(timeoutMs) || timeoutMs < 100 || timeoutMs > 60_000) {
      throw new RangeError("timeoutMs is outside the supported range");
    }
    this.#baseUrl = new URL(parsed.pathname.endsWith("/") ? parsed : `${parsed}/`);
    this.#fetch = fetchImpl;
    this.#timeoutMs = timeoutMs;
  }

  async cycle(cycleId: string, { signal }: { signal?: AbortSignal } = {}) {
    const id = requireIdentifier("cycleId", cycleId);
    return this.#request(`v1/cycles/${encodeURIComponent(id)}`, { method: "GET", signal });
  }

  async submitSnapshot(
    snapshot: Record<string, unknown>,
    { idempotencyKey, signal }: { idempotencyKey: string; signal?: AbortSignal },
  ) {
    this.#requireKey(idempotencyKey);
    return this.#request("v1/snapshots", {
      method: "POST",
      body: canonicalJson(snapshot),
      headers: { "Idempotency-Key": idempotencyKey },
      signal,
    });
  }

  async reconcile(
    cycleId: string,
    { idempotencyKey, signal }: { idempotencyKey: string; signal?: AbortSignal },
  ) {
    this.#requireKey(idempotencyKey);
    const id = requireIdentifier("cycleId", cycleId);
    return this.#request(`v1/cycles/${encodeURIComponent(id)}/reconciliation`, {
      method: "POST",
      body: "{}",
      headers: { "Idempotency-Key": idempotencyKey },
      signal,
    });
  }

  #requireKey(value: string) {
    if (!IDEMPOTENCY_KEY.test(value ?? "")) {
      throw new TypeError("a normalized idempotency key is required");
    }
  }

  async #request(path: string, options: RequestInit) {
    const timeout = AbortSignal.timeout(this.#timeoutMs);
    const signal = options.signal ? AbortSignal.any([options.signal, timeout]) : timeout;
    const response = await this.#fetch(new URL(path, this.#baseUrl), {
      ...options,
      signal,
      redirect: "error",
      headers: {
        Accept: "application/json",
        "Content-Type": "application/json",
        ...options.headers,
      },
    });
    const contentType = response.headers.get("content-type") ?? "";
    if (!contentType.toLowerCase().startsWith("application/json")) {
      throw new Error(`unexpected response type (${response.status})`);
    }
    const payload = (await response.json()) as Record<string, unknown>;
    if (!response.ok) {
      const error = new Error(
        String(payload.message ?? `request failed (${response.status})`),
      ) as Error & {
        status?: number;
        code?: string;
      };
      error.status = response.status;
      error.code = String(payload.code ?? "DELTAFORGE_REQUEST_FAILED");
      throw error;
    }
    return payload;
  }
}
