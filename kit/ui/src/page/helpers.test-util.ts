import { createFakeApi, sampleWorkspace, type FakeApi } from "../../../../apps/zenmark/frontend/src/api/fake";
import { createStore } from "../shell/store";
import type { Ctx, Slots } from "../shell/types";

export type Env = { api: FakeApi; ctx: Ctx; slots: Slots; navigations: [string, string | undefined][] };

export function setup(api: FakeApi = createFakeApi()): Env {
  const store = createStore({ workspace: sampleWorkspace() });
  const navigations: [string, string | undefined][] = [];
  const ctx: Ctx = {
    api, store, brand: "Zenmark",
    navigate(path, anchor) {
      navigations.push([path, anchor]);
      store.set({ path, anchor: anchor ?? "" });
    },
  };
  const slot = () => document.body.appendChild(document.createElement("div"));
  return { api, ctx, navigations, slots: { nav: slot(), breadcrumb: slot(), actions: slot(), main: slot() } };
}

export const tick = () => new Promise((r) => setTimeout(r, 0));
