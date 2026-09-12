import { useEffect, useState } from "react";
import { normalizePackageType, type PackageType } from "@/utils/packageType";

type DownloadFilters = {
  type: "all" | "Release" | "Preview" | "Beta";
  packageType: PackageType;
  status: "all" | "downloaded" | "not_downloaded";
  loader: "all" | "levilamina";
};

const STORAGE_KEY = "download.filters";

function readFilters(): DownloadFilters {
  let saved: Partial<DownloadFilters> = {};
  try {
    saved = JSON.parse(localStorage.getItem(STORAGE_KEY) || "{}") || {};
  } catch {}
  return {
    type:
      saved.type === "all" || saved.type === "Preview" || saved.type === "Beta" ? saved.type : "Release",
    packageType: normalizePackageType(saved.packageType),
    status:
      saved.status === "downloaded" || saved.status === "not_downloaded"
        ? saved.status
        : "all",
    loader: saved.loader === "levilamina" ? "levilamina" : "all",
  };
}

export function useDownloadFilters() {
  const [initial] = useState(readFilters);
  const [typeFilter, setTypeFilter] = useState(initial.type);
  const [statusFilter, setStatusFilter] = useState(initial.status);
  const [llFilter, setLlFilter] = useState(initial.loader);
  const [packageFilter, setPackageFilter] = useState(initial.packageType);

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
