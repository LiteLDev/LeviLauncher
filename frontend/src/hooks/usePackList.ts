import { useCallback, useEffect, useRef, useState } from "react";
import { useTranslation } from "react-i18next";
import { GetContentRoots, ListPackDetailsForVersion } from "bindings/github.com/liteldev/LeviLauncher/internal/app/contentservice";
import { GetPathSize } from "bindings/github.com/liteldev/LeviLauncher/internal/app/minecraft";
import type { PackType } from "bindings/github.com/liteldev/LeviLauncher/internal/packages/models";
import type { ContentRoots, PackListEntry } from "bindings/github.com/liteldev/LeviLauncher/internal/types/models";
import { EMPTY_CONTENT_ROOTS } from "@/utils/content";
import { notify } from "@/utils/activity";
import { useCurrentVersion } from "@/utils/CurrentVersionContext";

export type ListedPack = PackListEntry & { size?: number };
type SizeCache = Record<string, { modTime: number; size: number }>;
const SIZE_CACHE_KEY = "content.size.cache";

function readSizeCache(): SizeCache {
  try {
    const cache = JSON.parse(localStorage.getItem(SIZE_CACHE_KEY) || "{}");
    if (cache && typeof cache === "object" && !Array.isArray(cache)) return cache;
  } catch {}
  return {};
}

export function usePackList(packType: PackType) {
  const { t } = useTranslation();
  const { currentVersionName } = useCurrentVersion();
  const [loading, setLoading] = useState(true);
  const [roots, setRoots] = useState<ContentRoots>(EMPTY_CONTENT_ROOTS);
  const [packs, setPacks] = useState<ListedPack[]>([]);
  const generation = useRef(0);

  const refreshAll = useCallback(async (silent = false) => {
    const request = ++generation.current;
    const name = currentVersionName;
    if (!silent) setLoading(true);
    try {
      if (!name) {
        setRoots(EMPTY_CONTENT_ROOTS);
        setPacks([]);
        return;
      }
      const [contentRoots, details] = await Promise.all([
        GetContentRoots(name),
        ListPackDetailsForVersion(name, "", packType),
      ]);
      if (request !== generation.current) return;
      setRoots(contentRoots || EMPTY_CONTENT_ROOTS);
      const entries: ListedPack[] = details;

      const cache = readSizeCache();
      const missing: ListedPack[] = [];
      for (const pack of entries) {
        const cached = cache[pack.path];
        if (typeof cached?.size === "number" && cached.modTime === pack.modTime) {
          pack.size = cached.size;
        } else {
          missing.push(pack);
        }
      }
      setPacks(entries);

      // Show metadata immediately. Limit concurrent directory walks and publish the sizes together.
      const loadSizes = async () => {
        const updated: SizeCache = {};
        for (let i = 0; i < missing.length; i += 4) {
          if (request !== generation.current) return;
          await Promise.all(missing.slice(i, i + 4).map(async ({ path, modTime }) => {
            try {
              updated[path] = { modTime, size: await GetPathSize(path) };
            } catch {
              // A directory can disappear while it is being measured. Retry on refresh.
            }
          }));
        }
        if (request !== generation.current) return;
        setPacks(entries.map(pack => updated[pack.path] ? { ...pack, size: updated[pack.path].size } : pack));
        try {
          localStorage.setItem(SIZE_CACHE_KEY, JSON.stringify({ ...readSizeCache(), ...updated }));
        } catch {}
      };
      if (missing.length) void loadSizes();
    } catch (error) {
      if (request === generation.current) {
        notify({
          title: t("contentpage.error_resolve_paths"),
          description: String(error),
          variant: "danger",
          source: "content",
        });
      }
    } finally {
      if (request === generation.current) setLoading(false);
    }
  }, [currentVersionName, packType, t]);

  // Reloads whenever the current instance changes, including from the picker.
  useEffect(() => {
    setPacks([]);
    setRoots(EMPTY_CONTENT_ROOTS);
    void refreshAll();
    return () => { generation.current++; };
  }, [refreshAll]);

  return { loading, currentVersionName, roots, packs, refreshAll };
}
