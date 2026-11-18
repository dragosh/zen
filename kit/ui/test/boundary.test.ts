import { readdirSync, readFileSync, statSync } from "node:fs";
import { dirname, join, relative, resolve } from "node:path";
import { fileURLToPath } from "node:url";
import { expect, test } from "vitest";

const root = resolve(dirname(fileURLToPath(import.meta.url)), "..");
const repo = resolve(root, "..", "..");

function files(dir: string): string[] {
  return readdirSync(dir).flatMap((name) => {
    const p = join(dir, name);
    if (statSync(p).isDirectory()) return files(p);
    // The tests (and their helpers) may use the app's fake api as a ready-made host, as the runtime's do;
    // the shipped sources may not.
    return /\.(ts|css)$/.test(name) && !/\.test\.ts$/.test(name) && !/test-util\.ts$/.test(name) ? [p] : [];
  });
}

/** Every import / @import / import() specifier of a source file. */
function specifiers(src: string): string[] {
  const out: string[] = [];
  const re = /(?:\bfrom\s+|\bimport\s*\(\s*|\bimport\s+|@import\s+(?:url\()?)["']([^"']+)["']/g;
  for (const m of Array.from(src.matchAll(re))) out.push(m[1]);
  return out;
}

// The kit knows nothing of any app: its sources import only @zenmd/runtime and relative files that
// stay inside kit/ui/src. The data it needs from an app is the KitHost interface it declares itself.
test("kit/ui imports only @zenmd/runtime and its own files", () => {
  const src = join(root, "src");
  const found = files(src);
  expect(found.length).toBeGreaterThan(0);
  for (const file of found) {
    for (const spec of specifiers(readFileSync(file, "utf8"))) {
      const where = `${relative(root, file)}: ${spec}`;
      expect(spec, where).not.toMatch(/apps|frontend|wailsio|bindings/);
      if (spec.startsWith(".")) {
        const target = resolve(dirname(file), spec);
        expect(relative(src, target).startsWith(".."), where).toBe(false);
      } else {
        expect(spec === "@zenmd/runtime" || spec.startsWith("@zenmd/runtime/"), where).toBe(true);
      }
    }
  }
});

// Imports point down: runtime < kit < app. The runtime imports neither (runtime/test/boundary.test.ts);
// the app imports both.
test("the runtime does not import the kit", () => {
  for (const file of files(join(repo, "runtime", "src"))) {
    for (const spec of specifiers(readFileSync(file, "utf8"))) {
      expect(spec, relative(repo, file)).not.toMatch(/kit/);
    }
  }
});

test("the app imports both the runtime and the kit", () => {
  const app = join(repo, "apps", "zenmd-app", "frontend", "src");
  const all = files(app).flatMap((f) => specifiers(readFileSync(f, "utf8")));
  expect(all.some((s) => s === "@zenmd/kit" || s.startsWith("@zenmd/kit/"))).toBe(true);
  expect(all.some((s) => s.startsWith("@zenmd/runtime"))).toBe(true);
});

test("kit/ui/package.json is a private package without dependencies", () => {
  const pkg = JSON.parse(readFileSync(join(root, "package.json"), "utf8"));
  expect(pkg.name).toBe("@zenmd/kit");
  expect(pkg.private).toBe(true);
  expect(pkg.dependencies ?? {}).toEqual({});
  expect(pkg.devDependencies ?? {}).toEqual({});
});
