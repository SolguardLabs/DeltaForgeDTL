import { existsSync, mkdirSync, rmSync } from "node:fs";
import { dirname, join, resolve } from "node:path";
import { fileURLToPath } from "node:url";
import { spawnSync } from "node:child_process";

const root = resolve(dirname(fileURLToPath(import.meta.url)), "..");
const outDir = join(root, "out");
const exeName = process.platform === "win32" ? "deltaforgedtl.exe" : "deltaforgedtl";
const output = join(outDir, exeName);
const args = new Set(process.argv.slice(2));

if (args.has("--clean")) {
  rmSync(outDir, { recursive: true, force: true });
}
mkdirSync(outDir, { recursive: true });

function run(command, commandArgs, options = {}) {
  return spawnSync(command, commandArgs, {
    cwd: root,
    encoding: "utf8",
    shell: false,
    stdio: options.stdio ?? "pipe",
    env: { ...process.env, CGO_ENABLED: process.env.CGO_ENABLED ?? "0" },
  });
}

function commandExists(command) {
  const probe =
    process.platform === "win32"
      ? run("where.exe", [command])
      : spawnSync("sh", ["-c", `command -v "${command.replaceAll('"', '\\"')}"`], {
          cwd: root,
          encoding: "utf8",
          stdio: "pipe",
        });
  return probe.status === 0;
}

if (!commandExists("go")) {
  console.error("Go toolchain not found. Install Go 1.22+ and re-run npm run build.");
  process.exit(1);
}

const build = run("go", ["build", "-trimpath", "-o", output, "./src"], { stdio: "inherit" });
if (build.status !== 0) {
  process.exit(build.status ?? 1);
}

if (args.has("--vet")) {
  const vet = run("go", ["vet", "./src"], { stdio: "inherit" });
  if (vet.status !== 0) {
    process.exit(vet.status ?? 1);
  }
}

if (!existsSync(output)) {
  console.error(`Build finished without producing ${output}`);
  process.exit(1);
}
console.log(output);
