import { execFileSync } from "node:child_process";
import { existsSync, readFileSync } from "node:fs";
import { dirname, join, resolve } from "node:path";
import { fileURLToPath } from "node:url";
import { expect, test } from "vitest";

const root = resolve(dirname(fileURLToPath(import.meta.url)), "..");
const frontend = resolve(root, "..", "apps", "zenmark", "frontend");

// One build of the runtime: runtime/dist/zen.js is a classic IIFE (no module
// syntax, loadable by <script src>), next to zen.css; the app build no
// longer produces frontend/dist/static.
test("the runtime builds to a classic IIFE and a stylesheet", () => {
  execFileSync("npm", ["--prefix", "../../../runtime", "run", "build"], { cwd: frontend, stdio: "pipe" });
  const js = readFileSync(join(root, "dist", "zen.js"), "utf8");
  expect(js.length).toBeGreaterThan(1000);
  expect(js).not.toMatch(/^\s*import[\s{*"']/m);
  expect(js).not.toMatch(/^\s*export[\s{*]/m);
  expect(js).toMatch(/^\s*(var |\(function|!function|\(\(\)=>)/);
  expect(readFileSync(join(root, "dist", "zen.css"), "utf8")).toContain("--");
}, 60_000);

test("the app build no longer produces frontend/dist/static", () => {
  expect(existsSync(join(frontend, "dist", "static"))).toBe(false);
  const scripts = JSON.parse(readFileSync(join(frontend, "package.json"), "utf8")).scripts;
  expect(scripts.build).not.toContain("static");
  expect(scripts["build:dev"]).not.toContain("static");
});
