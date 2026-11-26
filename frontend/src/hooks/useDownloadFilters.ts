import { useEffect, useState } from "react";
import { System } from "@wailsio/runtime";
import { normalizePackageType, type PackageType } from "@/utils/packageType";

type DownloadFilters = {
  type: "all" | "Release" | "Preview" | "Beta";
  packageType: PackageType | "all";
  status: "all" | "downloaded" | "not_downloaded";
  loader: "all" | "levilamina";
};

const STORAGE_KEY = "download.filters";

function readFilters(isMac: boolean): DownloadFilters {
  let saved: Partial<DownloadFilters> = {};
  try {
    saved = JSON.parse(localStorage.getItem(STORAGE_KEY) || "{}") || {};
  } catch {}
  const savedType =
    saved.type === "all" || saved.type === "Preview" || (!isMac && saved.type === "Beta")
      ? saved.type
      : "Release";
  return {
    type: savedType,
    packageType: isMac
      ? "gdk"
      : saved.packageType === "all"
        ? "all"
        : normalizePackageType(saved.packageType),
    status:
      saved.status === "downloaded" || saved.status === "not_downloaded"
        ? saved.status
        : "all",
    loader: saved.loader === "levilamina" ? "levilamina" : "all",
  };
}

export function useDownloadFilters() {
  const isMac = System.IsMac();
  const [initial] = useState(() => readFilters(isMac));
  const [typeFilter, setTypeFilterState] = useState(initial.type);
  const [statusFilter, setStatusFilter] = useState(initial.status);
  const [llFilter, setLlFilter] = useState(initial.loader);
  const [packageFilter, setPackageFilterState] = useState(initial.packageType);

  const setTypeFilter = (next: DownloadFilters["type"]) => {
    setTypeFilterState(isMac && next === "Beta" ? "Release" : next);
  };

  const setPackageFilter = (next: DownloadFilters["packageType"]) => {
    setPackageFilterState(isMac ? "gdk" : next);
  };

  useEffect(() => {
    try {
      localStorage.setItem(
        STORAGE_KEY,
        JSON.stringify({
          type: typeFilter,
          status: statusFilter,
          loader: llFilter,
          packageType: packageFilter,
        }),
      );
    } catch {}
  }, [typeFilter, statusFilter, llFilter, packageFilter]);

  return {
    typeFilter,
    setTypeFilter,
    statusFilter,
    setStatusFilter,
    llFilter,
    setLlFilter,
    packageFilter,
    setPackageFilter,
  };
}
