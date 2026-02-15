import React from "react";

/**
 * Manages content sort state with localStorage persistence,
 * filtering, and pagination. Shared by content list pages.
 */
export const useContentSort = <T>(
  storageKey: string,
  items: T[],
  getNameFn: (item: T) => string,
  getTimeFn: (item: T) => number,
  pageSize = 20,
) => {
  const [query, setQuery] = React.useState<string>("");
  const [sortKey, setSortKey] = React.useState<"name" | "time">(() => {
    try {
      const saved = JSON.parse(localStorage.getItem(storageKey) || "{}");
      const k = saved?.sortKey;
      if (k === "name" || k === "time") return k;
    } catch {}
    return "name";
  });
  const [sortAsc, setSortAsc] = React.useState<boolean>(() => {
    try {
      const saved = JSON.parse(localStorage.getItem(storageKey) || "{}");
      const a = saved?.sortAsc;
      if (typeof a === "boolean") return a;
    } catch {}
    return true;
  });
  const [currentPage, setCurrentPage] = React.useState(1);

  // Reset page on filter/sort change
  React.useEffect(() => {
    setCurrentPage(1);
  }, [query, sortKey, sortAsc]);

  // Persist sort settings
  React.useEffect(() => {
    try {
      localStorage.setItem(storageKey, JSON.stringify({ sortKey, sortAsc }));
    } catch {}
  }, [sortKey, sortAsc, storageKey]);

  const filtered = React.useMemo(() => {
    const q = query.trim().toLowerCase();
    const f = items.filter((item) => {
      const nm = getNameFn(item).toLowerCase();
      return q ? nm.includes(q) : true;
    });
    return f.sort((a, b) => {
      if (sortKey === "name") {
        const an = getNameFn(a).toLowerCase();
        const bn = getNameFn(b).toLowerCase();
        const res = an.localeCompare(bn);
        return sortAsc ? res : -res;
      } else {
        const at = getTimeFn(a);
        const bt = getTimeFn(b);
        const res = at - bt;
        return sortAsc ? res : -res;
      }
    });
  }, [items, query, sortKey, sortAsc, getNameFn, getTimeFn]);

  const paginatedItems = React.useMemo(() => {
    const start = (currentPage - 1) * pageSize;
    return filtered.slice(start, start + pageSize);
  }, [filtered, currentPage, pageSize]);

  const totalPages = Math.ceil(filtered.length / pageSize);

  const setSort = React.useCallback(
    (key: "name" | "time", asc: boolean) => {
      setSortKey(key);
      setSortAsc(asc);
    },
    [],
  );

  return {
    query,
    setQuery,
    sortKey,
    sortAsc,
    setSortKey,
    setSortAsc,
    setSort,
    currentPage,
    setCurrentPage,
    filtered,
    paginatedItems,
    totalPages,
  };
};
