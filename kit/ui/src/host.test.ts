import { expect, test } from "vitest";
import { hasAddon, hasFeature, type BuildInfo } from "./host";
import { FEATURE_IDS } from "./shell/types";

test("hasAddon: by id, false without a list", () => {
  const vale = { id: "vale", name: "Vale linting", summary: "s" };
  expect(hasAddon({ addons: [vale] }, "vale")).toBe(true);
  expect(hasAddon({ addons: [vale] }, "voice")).toBe(false);
  expect(hasAddon({ addons: [] }, "vale")).toBe(false);
  expect(hasAddon({}, "vale")).toBe(false);
  expect(hasAddon({ addons: undefined }, "vale")).toBe(false);
});

test("hasFeature: by id; true when the host does not say (no build info)", () => {
  const build = (features: string[]): BuildInfo => ({ app: "x", features, addons: [], plugins: [], layouts: [], templates: [], dialects: [], theme: "default" });
  expect(hasFeature({ build: build(["editor"]) }, "editor")).toBe(true);
  expect(hasFeature({ build: build([]) }, "editor")).toBe(false);
  expect(hasFeature({ build: build(["editor"]) }, "newsite")).toBe(false);
  expect(hasFeature({}, "editor")).toBe(true);
  expect(hasFeature({ build: undefined }, "editor")).toBe(true);
});

test("the feature ids are the catalog's, in order", () => {
  expect([...FEATURE_IDS]).toEqual(["reader", "nav", "find", "editor", "inspector", "print", "export", "newsite", "developer"]);
});
