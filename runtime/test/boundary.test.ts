import { readdirSync, readFileSync, statSync } from "node:fs";
import { dirname, join, relative, resolve } from "node:path";
import { fileURLToPath } from "node:url";
import { expect, test } from "vitest";

const root = resolve(dirname(fileURLToPath(import.meta.url)), "..");

function files(dir: string): string[] {
  return readdirSync(dir).flatMap((name) => {
    const p = join(dir, name);
    if (statSync(p).isDirectory()) return files(p);
    return /\.(ts|css)$/.test(name) ? [p] : [];
  });
}

/** Every import / @import / import() specifier of a source file. */
function specifiers(src: string): string[] {
  const out: string[] = [];
  const re = /(?:\bfrom\s+|\bimport\s*\(\s*|\bimport\s+|@import\s+(?:url\()?)["']([^"']+)["']/g;
  for (const m of src.matchAll(re)) out.push(m[1]);
  return out;
}

// The runtime knows nothing of the app: no import of frontend/, and no
// relative import that leaves runtime/. Its only package is itself.
test("the runtime imports nothing from frontend", () => {
  const src = join(root, "src");
  const found = files(src);
  expect(found.length).toBeGreaterThan(0);
  for (const file of found) {
    for (const spec of specifiers(readFileSync(file, "utf8"))) {
      const where = `${relative(root, file)}: ${spec}`;
      expect(spec, where).not.toMatch(/frontend/);
      if (spec.startsWith(".")) {
        const target = resolve(dirname(file), spec);
        expect(relative(root, target).startsWith(".."), where).toBe(false);
      } else {
        expect(spec.startsWith("node:") || spec.startsWith("@zenmd/runtime"), where).toBe(true);
      }
    }
  }
});

test("runtime/package.json is a private package without dependencies", () => {
  const pkg = JSON.parse(readFileSync(join(root, "package.json"), "utf8"));
  expect(pkg.name).toBe("@zenmd/runtime");
  expect(pkg.private).toBe(true);
  expect(pkg.dependencies ?? {}).toEqual({});
  expect(pkg.devDependencies ?? {}).toEqual({});
});
