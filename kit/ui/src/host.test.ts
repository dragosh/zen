import { expect, test } from "vitest";
import { hasAddon } from "./host";

test("hasAddon: by id, false without a list", () => {
  const vale = { id: "vale", name: "Vale linting", summary: "s" };
  expect(hasAddon({ addons: [vale] }, "vale")).toBe(true);
  expect(hasAddon({ addons: [vale] }, "voice")).toBe(false);
  expect(hasAddon({ addons: [] }, "vale")).toBe(false);
  expect(hasAddon({}, "vale")).toBe(false);
  expect(hasAddon({ addons: undefined }, "vale")).toBe(false);
});
