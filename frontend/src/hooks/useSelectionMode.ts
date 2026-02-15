import React from "react";

/**
 * Manages selection mode with toggle, select all, and count.
 * Shared by content list pages.
 */
export const useSelectionMode = <T extends { path?: string; Path?: string }>(
  filteredItems: T[],
  getKey: (item: T) => string = (item) =>
    (item as any).path || (item as any).Path || "",
) => {
  const [selected, setSelected] = React.useState<Record<string, boolean>>({});
  const [isSelectMode, setIsSelectMode] = React.useState<boolean>(false);

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

  return {
    selected,
    isSelectMode,
    selectedCount,
    toggleSelect,
    selectAll,
    toggleSelectMode,
    clearSelection,
    getSelectedKeys,
  };
};
