import { listDirectories } from "./fs";
import { GetUserGamertagMap } from "bindings/github.com/liteldev/LeviLauncher/userservice";

export async function listPlayers(usersRoot: string): Promise<string[]> {
  if (!usersRoot) return [];
  const entries = await listDirectories(usersRoot);
  return entries
    .map((e) => e.name)
    .filter((n) => n && n !== "9556213259376595538");
}

let cachedUsersRoot = "";
let cachedMap: Record<string, string> = {};
let cachedPromise: Promise<Record<string, string>> | null = null;

export async function getPlayerGamertagMap(
  usersRoot: string,
): Promise<Record<string, string>> {
  const root = String(usersRoot || "");
  if (!root) return {};
  if (root === cachedUsersRoot && cachedPromise) return cachedPromise;

  cachedUsersRoot = root;
  cachedMap = {};
  cachedPromise = (async () => {
    try {
      const fn = GetUserGamertagMap as any;
      if (typeof fn !== "function") return {};
      const res = await fn(root);
      if (res && typeof res === "object") {
        cachedMap = res as Record<string, string>;
        return cachedMap;
      }
    } catch {}
    return {};
  })();

  return cachedPromise;
}

export function resolvePlayerDisplayName(
  playerFolder: string,
  map: Record<string, string> | undefined,
): string {
  const key = String(playerFolder || "");
  const gt = map ? map[key] : undefined;
  return typeof gt === "string" && gt.trim() ? gt : key;
}
