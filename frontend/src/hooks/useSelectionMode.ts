import React from "react";

/**
 * Manages selection mode with toggle, select all, and count.
 * Shared by content list pages.
 */
export const useSelectionMode = <T extends { path?: string; Path?: string }>(
  filteredItems: T[],
  getKey: (item: T) => string = (item) =>
    (item as any).path || (item as any).Path || "",
  scopeKey = "",
  allItems: T[] = filteredItems,
) => {
  const [selectionScope, setSelectionScope] = React.useState(scopeKey);
  const [storedSelected, setSelected] = React.useState<Record<string, boolean>>({});
  const [storedSelectMode, setIsSelectMode] = React.useState<boolean>(false);
  const validKeys = new Set(allItems.map(getKey));
  const selected = Object.fromEntries(
    Object.entries(selectionScope === scopeKey ? storedSelected : {}).filter(
      ([key, value]) => value && validKeys.has(key),
    ),
  );
  const isSelectMode = selectionScope === scopeKey && storedSelectMode;

  React.useEffect(() => {
    setSelectionScope(scopeKey);
    setSelected({});
    setIsSelectMode(false);
  }, [scopeKey]);

  const selectedCount = React.useMemo(
    () => Object.keys(selected).filter((k) => selected[k]).length,
    [selected],
  );

  const toggleSelect = React.useCallback((key: string) => {
    setSelected((prev) => ({
      ...prev,
      [key]: !prev[key],
    }));
  }, []);

  const selectAll = React.useCallback(
    (val: boolean) => {
      if (val) {
        const newSel: Record<string, boolean> = {};
        filteredItems.forEach((item) => {
          newSel[getKey(item)] = true;
        });
        setSelected(newSel);
      } else {
        setSelected({});
      }
    },
    [filteredItems, getKey],
  );

  const toggleSelectMode = React.useCallback(() => {
    setIsSelectMode((prev) => {
      if (prev) setSelected({});
      return !prev;
    });
  }, []);

  const clearSelection = React.useCallback(() => {
    setSelected({});
  }, []);

  const getSelectedKeys = React.useCallback(() => {
    return Object.keys(selected).filter((k) => selected[k]);
  }, [selected]);

  const retainSelection = React.useCallback((keys: string[]) => {
    setSelected(Object.fromEntries(keys.map((key) => [key, true])));
  }, []);
  const visibleSelectedCount = filteredItems.filter((item) => selected[getKey(item)]).length;

  return {
    selected,
    isSelectMode,
    selectedCount,
    hiddenSelectedCount: selectedCount - visibleSelectedCount,
    visibleSelectedCount,
    toggleSelect,
    selectAll,
    toggleSelectMode,
    clearSelection,
    retainSelection,
    getSelectedKeys,
  };
};
