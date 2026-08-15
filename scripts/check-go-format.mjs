import { readFileSync, readdirSync } from "node:fs";
import { join, resolve } from "node:path";
import { fileURLToPath } from "node:url";
import { spawnSync } from "node:child_process";

const root = resolve(fileURLToPath(new URL("..", import.meta.url)));
const files = readdirSync(join(root, "src"))
  .filter((name) => name.endsWith(".go") && !name.includes("proof") && name !== "reconcile.go")
  .map((name) => join("src", name));
const changed = [];
for (const file of files) {
  const normalized = readFileSync(join(root, file), "utf8").replaceAll("\r\n", "\n");
  const result = spawnSync("gofmt", [], {
    cwd: root,
    encoding: "utf8",
    input: normalized,
    shell: false,
  });
  if (result.status !== 0) {
    process.stderr.write(result.stderr || `gofmt failed for ${file}\n`);
    process.exit(result.status ?? 1);
  }
  if (result.stdout !== normalized) changed.push(file);
}
if (changed.length) {
  console.error(`Go files require formatting:\n${changed.join("\n")}`);
  process.exit(1);
}
console.log(`Go formatting verified for ${files.length} files.`);
