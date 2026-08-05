import { readdir, readFile } from "node:fs/promises";
import { basename, extname, resolve } from "node:path";
import type { Page } from "@playwright/test";

type MethodResponses = Record<string, unknown>;

const bindingRoot = resolve(
  "bindings",
  "github.com",
  "liteldev",
  "LeviLauncher",
);

const loadBindingMethodIDs = async (): Promise<Map<number, string>> => {
  const ids = new Map<number, string>();
  for (const entry of await readdir(bindingRoot, { withFileTypes: true })) {
    if (!entry.isFile() || extname(entry.name) !== ".js") continue;
    if (entry.name === "index.js" || entry.name === "models.js") continue;

    const source = await readFile(resolve(bindingRoot, entry.name), "utf8");
    const service = basename(entry.name, ".js");
    const methodPattern =
      /export function (\w+)\([^)]*\) \{[\s\S]*?\$Call\.ByID\((\d+)/g;
    for (const match of source.matchAll(methodPattern)) {
      ids.set(Number(match[2]), `${service}.${match[1]}`);
    }
  }
  return ids;
};

const defaultResponses: MethodResponses = {
  "minecraft.GetBaseRoot": "",
  "minecraft.GetImageURL": "",
  "minecraft.GetIsBeta": false,
  "minecraft.IsGameInputInstalled": true,
  "minecraft.IsGamingServicesInstalled": true,
  "minecraft.IsVcRuntimeInstalled": true,
  "minecraft.GetLanguageNames": [
    { code: "en_US", language: "English" },
    { code: "zh_CN", language: "简体中文" },
  ],
  "minecraft.GetLipStatus": {
    installed: false,
    upToDate: true,
    currentVersion: "",
    latestVersion: "",
  },
  "minecraft.CheckUpdate": {
    isUpdate: false,
    version: "",
    body: "",
  },
  "versionservice.ListVersionMetas": [],
  "versionservice.ListVersionMetasWithRegistered": [],
  "versionservice.GetVersionMenuDetails": [],
};

export const mockWailsRuntime = async (
  page: Page,
  responses: MethodResponses = {},
) => {
  const methodIDs = await loadBindingMethodIDs();
  const mergedResponses = { ...defaultResponses, ...responses };

  await page.route("**/wails/runtime", async (route) => {
    let response: unknown = null;
    try {
      const request = route.request().postDataJSON();
      const methodID = Number(request?.args?.methodID);
      const methodName = methodIDs.get(methodID);
      if (methodName && Object.hasOwn(mergedResponses, methodName)) {
        response = mergedResponses[methodName];
      }
    } catch {
      response = null;
    }

    await route.fulfill({
      status: 200,
      contentType: "application/json",
      body: JSON.stringify(response),
    });
  });
};

export const seedCompletedSetup = async (page: Page) => {
  await page.addInitScript(() => {
    localStorage.setItem("ll.onboarded", "1");
    localStorage.setItem("ll.termsAccepted", "1");
    localStorage.setItem("ll.clarity.choiceMade", "1");
    localStorage.setItem("ll.clarity.enabled", "false");
    localStorage.setItem("i18nextLng", "en_US");
  });
};
