import { createHash } from "node:crypto";
import { existsSync, readFileSync, readdirSync } from "node:fs";
import { extname, join, relative } from "node:path";
import { fileURLToPath } from "node:url";

const root = new URL("../", import.meta.url);
const rootPath = fileURLToPath(root);
const requiredDocs = [
  "arquitectura.md",
  "ciclo-conciliacion.md",
  "gobernanza.md",
  "integracion.md",
  "modelo-economico.md",
  "observabilidad.md",
  "operaciones.md",
];
const docs = readdirSync(new URL("docs/", root))
  .filter((name) => name.endsWith(".md"))
  .sort();
if (JSON.stringify(docs) !== JSON.stringify(requiredDocs)) {
  throw new Error(`docs inventory differs: ${docs.join(", ")}`);
}

const banner = readFileSync(new URL("assets/banner.png", root));
const bannerHash = createHash("sha256").update(banner).digest("hex");
if (bannerHash !== "b3a0cf66e238726add1bdf4da550439787ca42ed5048ee7a6b3c3ad832675385") {
  throw new Error("banner identity differs from the approved DeltaForgeDTL asset");
}

const readableExtensions = new Set([".md", ".go", ".ts", ".mjs", ".json", ".yml", ".yaml", ".sh"]);
const ignoredDirectories = new Set([".git", "node_modules", "out"]);
const readableFiles = [];
function walk(directory) {
  for (const entry of readdirSync(directory, { withFileTypes: true })) {
    if (entry.isDirectory() && ignoredDirectories.has(entry.name)) continue;
    const path = join(directory, entry.name);
    if (entry.isDirectory()) walk(path);
    else if (readableExtensions.has(extname(entry.name)) || entry.name === "LICENSE")
      readableFiles.push(path);
  }
}
walk(rootPath);

const restricted = [
  ["l", "ab"].join(""),
  ["l", "abs"].join(""),
  ["labor", "atorio"].join(""),
  ["c", "tf"].join(""),
  ["vulnera", "bilidad"].join(""),
  ["vulnera", "ble"].join(""),
  ["vulnera", "bility"].join(""),
  ["b", "ug"].join(""),
  ["ex", "ploit"].join(""),
  ["by", "pass"].join(""),
  ["atta", "cker"].join(""),
];
for (const path of readableFiles) {
  const content = readFileSync(path, "utf8");
  for (const word of restricted) {
    if (new RegExp(`\\b${word}\\b`, "iu").test(content)) {
      throw new Error(`restricted public terminology in ${relative(rootPath, path)}`);
    }
  }
}

const narrativeFiles = ["README.md", "SECURITY.md", ...docs.map((name) => `docs/${name}`)];
const narrative = narrativeFiles
  .map((path) => readFileSync(new URL(path, root), "utf8"))
  .join("\n");
const diagramCount = narrative.match(/```mermaid/g)?.length ?? 0;
if (diagramCount < 20)
  throw new Error(`expected at least 20 Mermaid diagrams, found ${diagramCount}`);
if (!readFileSync(new URL("README.md", root), "utf8").includes("./assets/banner.png")) {
  throw new Error("README does not reference the approved banner");
}

const goSource = readdirSync(new URL("src/", root))
  .filter((name) => name.endsWith(".go") && !name.includes("proof"))
  .map((name) => readFileSync(new URL(`src/${name}`, root), "utf8"))
  .join("\n");
const goLines = goSource.split(/\r?\n/).filter((line) => line.trim()).length;
if (goLines < 3_600) throw new Error(`Go source depth is ${goLines}, expected 3600`);

for (const path of [
  ".gitattributes",
  ".github/CODEOWNERS",
  ".github/workflows/ci.yml",
  ".github/workflows/release-integrity.yml",
  "package-lock.json",
  "sdk/deltaForgeClient.ts",
]) {
  if (!existsSync(new URL(path, root))) throw new Error(`required artifact is missing: ${path}`);
}

console.log(
  JSON.stringify({
    bannerSha256: bannerHash,
    diagrams: diagramCount,
    docs: docs.length,
    goNonBlankLines: goLines,
    status: "verified",
  }),
);
