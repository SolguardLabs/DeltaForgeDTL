import { spawnSync } from "node:child_process";
import { existsSync } from "node:fs";
import { dirname, join, resolve } from "node:path";
import { fileURLToPath } from "node:url";

export type Amount = number;

export type BalanceLine = {
  asset: string;
  expected: Amount;
  executed: Amount;
  final: Amount;
  drift: Amount;
  delta: Amount;
  locked: Amount;
};

export type AccountReport = {
  id: string;
  owner: string;
  kind: string;
  status: string;
  adjustmentGroup: string;
  settlementWeight: number;
  riskTier: number;
  liquidityTier: number;
  balances: BalanceLine[];
  flags?: string[];
};

export type AssetReport = {
  id: string;
  symbol: string;
  expectedTotal: Amount;
  executedTotal: Amount;
  finalTotal: Amount;
  globalNet: Amount;
  correctionNet: Amount;
  liquidationNet: Amount;
  drift: Amount;
  accountCount: number;
  microCount: number;
  correctionCount: number;
};

export type Difference = {
  account: string;
  owner: string;
  asset: string;
  expected: Amount;
  executed: Amount;
  drift: Amount;
  absDrift: Amount;
  kind: string;
  group: string;
  reason: string;
  weight: number;
};

export type GlobalPool = {
  asset: string;
  group: string;
  amount: Amount;
  absAmount: Amount;
  entries: number;
  positiveEntries: number;
  negativeEntries: number;
  maxSingleDrift: Amount;
};

export type Correction = {
  id: string;
  account: string;
  asset: string;
  amount: Amount;
  before: Amount;
  after: Amount;
  reason: string;
  status: string;
  epoch: number;
  trace: string;
};

export type Allocation = {
  id: string;
  account: string;
  asset: string;
  group: string;
  amount: Amount;
  weight: number;
  before: Amount;
  after: Amount;
  pool: string;
  sequence: number;
};

export type Liquidation = {
  id: string;
  account: string;
  asset: string;
  starting: Amount;
  settled: Amount;
  haircut: Amount;
  final: Amount;
  reason: string;
  haircutBps: number;
  sequence: number;
};

export type DeltaForgeReport = {
  name: string;
  description?: string;
  epoch: number;
  book: string;
  source: string;
  assets: AssetReport[];
  accounts: AccountReport[];
  snapshot: {
    epoch: number;
    book: string;
    accountCount: number;
    assetCount: number;
    expectedTotals: Record<string, Amount>;
    executedTotals: Record<string, Amount>;
    finalTotals: Record<string, Amount>;
    lockedTotals: Record<string, Amount>;
    hash: string;
    canonicalLineCount: number;
  };
  reconciliation: {
    differences: Difference[];
    globalPools: GlobalPool[];
    microTotal: Amount;
    correctionTotal: Amount;
    exactCount: number;
    microCount: number;
    correctionCount: number;
    blockedCount: number;
    balancedByAsset: Array<{
      asset: string;
      expectedTotal: Amount;
      executedTotal: Amount;
      finalTotal: Amount;
      observedDrift: Amount;
      settledDrift: Amount;
      balanced: boolean;
    }>;
  };
  corrections: Correction[];
  settlement: {
    allocations: Allocation[];
    liquidations: Liquidation[];
    allocationTotal: Amount;
    liquidationTotal: Amount;
    receiverCount: number;
    poolCount: number;
    roundingRemainder: Amount;
    completed: boolean;
  };
  metrics: Record<string, unknown>;
  events?: Array<Record<string, unknown>>;
};

export const root = resolve(dirname(fileURLToPath(import.meta.url)), "..", "..");
export const binary = join(
  root,
  "out",
  process.platform === "win32" ? "deltaforgedtl.exe" : "deltaforgedtl",
);

export function ensureBuilt(): void {
  if (existsSync(binary)) {
    return;
  }
  const result = spawnSync(process.execPath, ["scripts/build.mjs"], {
    cwd: root,
    encoding: "utf8",
  });
  if (result.status !== 0) {
    throw new Error(result.stderr || result.stdout || "build failed");
  }
}

export function runCli(args: string[]): string {
  ensureBuilt();
  const result = spawnSync(binary, args, {
    cwd: root,
    encoding: "utf8",
  });
  if (result.status !== 0) {
    throw new Error(
      `command failed: ${binary} ${args.join(" ")}\n${result.stderr || result.stdout}`,
    );
  }
  return result.stdout;
}

export function runFixture(name: string, options: string[] = []): DeltaForgeReport {
  return JSON.parse(
    runCli(["run", join("tests", "fixtures", name), ...options]),
  ) as DeltaForgeReport;
}

export function runScenario(name: string, options: string[] = []): DeltaForgeReport {
  return JSON.parse(runCli(["scenario", name, ...options])) as DeltaForgeReport;
}

export function validateFixture(name: string): string {
  return runCli(["validate", join("tests", "fixtures", name)]).trim();
}

export function byId<T extends { id: string }>(items: T[], id: string): T {
  const found = items.find((item) => item.id === id);
  if (!found) {
    throw new Error(`missing id ${id}`);
  }
  return found;
}

export function balance(account: AccountReport, asset: string): BalanceLine {
  const found = account.balances.find((entry) => entry.asset === asset);
  if (!found) {
    throw new Error(`missing balance ${account.id}/${asset}`);
  }
  return found;
}
