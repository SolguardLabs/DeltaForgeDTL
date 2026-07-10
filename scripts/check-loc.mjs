import { readdirSync, readFileSync } from "node:fs";
import { join, resolve } from "node:path";
import { fileURLToPath } from "node:url";

const root = resolve(fileURLToPath(new URL("..", import.meta.url)));
let lines = 0;

for (const file of readdirSync(join(root, "src"))) {
  if (!file.endsWith(".go")) continue;
  const text = readFileSync(join(root, "src", file), "utf8");
  lines += text.split(/\r?\n/).filter((line) => line.trim().length > 0).length;
}

console.log(lines);
if (lines < 3000 || lines > 4000) {
  console.error(`src/ LOC must be between 3000 and 4000, got ${lines}`);
  process.exit(1);
}
