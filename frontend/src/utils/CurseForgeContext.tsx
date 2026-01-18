import React, { createContext, useContext, useState, useRef } from "react";
import * as types from "bindings/github.com/liteldev/LeviLauncher/internal/types/models";
import {
  GameVersion,
  Categories,
} from "bindings/github.com/liteldev/LeviLauncher/internal/curseforge/client/types";

interface CurseForgeContextValue {
  query: string;
  setQuery: (q: string) => void;
  mods: types.CurseForgeMod[];
  setMods: (mods: types.CurseForgeMod[]) => void;
  gameVersions: GameVersion[];
  setGameVersions: (v: GameVersion[]) => void;
  selectedMinecraftVersion: string;
  setSelectedMinecraftVersion: (v: string) => void;
  allCategories: Categories[];
  setAllCategories: (c: Categories[]) => void;
  selectedClass: number;
  setSelectedClass: (id: number) => void;
  selectedCategories: number[];
  setSelectedCategories: (ids: number[]) => void;
  currentPage: number;
  setCurrentPage: (p: number) => void;
  searchToken: number;
  setSearchToken: (t: number | ((prev: number) => number)) => void;
  totalCount: number;
  setTotalCount: (c: number) => void;
  selectedSort: number;
  setSelectedSort: (s: number) => void;
  initialLoaded: boolean;
  setInitialLoaded: (b: boolean) => void;
  scrollPosition: number;
  setScrollPosition: (y: number) => void;
  hasSearched: boolean;
  setHasSearched: (b: boolean) => void;
}

const CurseForgeContext = createContext<CurseForgeContextValue | undefined>(
  undefined,
);

export const CurseForgeProvider: React.FC<{ children: React.ReactNode }> = ({
  children,
}) => {
  const [query, setQuery] = useState("");
  const [mods, setMods] = useState<types.CurseForgeMod[]>([]);
  const [gameVersions, setGameVersions] = useState<GameVersion[]>([]);
  const [selectedMinecraftVersion, setSelectedMinecraftVersion] =
    useState<string>("");
  const [allCategories, setAllCategories] = useState<Categories[]>([]);
  const [selectedClass, setSelectedClass] = useState<number>(0);
  const [selectedCategories, setSelectedCategories] = useState<number[]>([]);
  const [currentPage, setCurrentPage] = useState(1);
  const [searchToken, setSearchToken] = useState(0);
  const [totalCount, setTotalCount] = useState(0);
  const [selectedSort, setSelectedSort] = useState<number>(1);
  const [initialLoaded, setInitialLoaded] = useState(false);
  const [scrollPosition, setScrollPosition] = useState(0);
  const [hasSearched, setHasSearched] = useState(false);

  const value = {
    query,
    setQuery,
    mods,
    setMods,
    gameVersions,
    setGameVersions,
    selectedMinecraftVersion,
    setSelectedMinecraftVersion,
    allCategories,
    setAllCategories,
    selectedClass,
    setSelectedClass,
    selectedCategories,
    setSelectedCategories,
    currentPage,
    setCurrentPage,
    searchToken,
    setSearchToken,
    totalCount,
    setTotalCount,
    selectedSort,
    setSelectedSort,
    initialLoaded,
    setInitialLoaded,
    scrollPosition,
    setScrollPosition,
    hasSearched,
    setHasSearched,
  };

  return (
    <CurseForgeContext.Provider value={value}>
      {children}
    </CurseForgeContext.Provider>
  );
};

export const useCurseForge = () => {
  const context = useContext(CurseForgeContext);
  if (!context) {
    throw new Error("useCurseForge must be used within a CurseForgeProvider");
  }
  return context;
};
