import { useEffect, useState } from "react";

type DownloadFilters = {
  type: "all" | "Release" | "Preview";
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
      saved.type === "all" || saved.type === "Preview" ? saved.type : "Release",
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

  useEffect(() => {
    try {
      localStorage.setItem(
        STORAGE_KEY,
        JSON.stringify({
          type: typeFilter,
          status: statusFilter,
          loader: llFilter,
        }),
      );
    } catch {}
  }, [typeFilter, statusFilter, llFilter]);

  return {
    typeFilter,
    setTypeFilter,
    statusFilter,
    setStatusFilter,
    llFilter,
    setLlFilter,
  };
}
