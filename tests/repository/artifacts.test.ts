import test from "node:test";
import assert from "node:assert/strict";
import { createHash } from "node:crypto";
import { readFile, readdir } from "node:fs/promises";

test("documentation inventory is explicit and complete", async () => {
  const docs = (await readdir(new URL("../../docs/", import.meta.url)))
    .filter((name) => name.endsWith(".md"))
    .sort();
  assert.deepEqual(docs, [
    "arquitectura.md",
    "ciclo-conciliacion.md",
    "gobernanza.md",
    "integracion.md",
    "modelo-economico.md",
    "observabilidad.md",
    "operaciones.md",
  ]);
});

test("README binds the approved DeltaForgeDTL banner", async () => {
  const [readme, banner] = await Promise.all([
    readFile(new URL("../../README.md", import.meta.url), "utf8"),
    readFile(new URL("../../assets/banner.png", import.meta.url)),
  ]);
  assert.match(readme, /\.\/assets\/banner\.png/);
  assert.equal(
    createHash("sha256").update(banner).digest("hex"),
    "b3a0cf66e238726add1bdf4da550439787ca42ed5048ee7a6b3c3ad832675385",
  );
});

test("promotion workflow verifies all immutable references", async () => {
  const workflow = await readFile(
    new URL("../../.github/workflows/release-integrity.yml", import.meta.url),
    "utf8",
  );
  assert.match(workflow, /origin\/production/);
  assert.match(workflow, /GITHUB_REF_TYPE/);
  assert.match(workflow, /github\.event\.release\.tag_name/);
});
