import React, { createContext, useContext, useEffect, useState, useCallback } from "react";
import {
  useLocation,
  useNavigationType,
  NavigationType,
} from "react-router-dom";

interface HistoryEntry {
  key: string;
  pathname: string;
  title: string;
}

interface NavigationHistoryContextType {
  historyStack: HistoryEntry[];
  currentIndex: number;
  canGoBack: boolean;
  canGoForward: boolean;
  getBackEntry: () => HistoryEntry | null;
  getForwardEntry: () => HistoryEntry | null;
  currentTitle: string;
  setCurrentTitle: (title: string) => void;
}

const NavigationHistoryContext = createContext<NavigationHistoryContextType>({
  historyStack: [],
  currentIndex: -1,
  canGoBack: false,
  canGoForward: false,
  getBackEntry: () => null,
  getForwardEntry: () => null,
  currentTitle: "",
  setCurrentTitle: () => {},
});

export const useNavigationHistory = () => useContext(NavigationHistoryContext);

export const NavigationHistoryProvider: React.FC<{
  children: React.ReactNode;
}> = ({ children }) => {
  const location = useLocation();
  const navType = useNavigationType();
  const [titles, setTitles] = useState<Record<string, string>>({});
  const setCurrentTitle = useCallback((title: string) => {
    setTitles((previous) => previous[location.key] === title
      ? previous
      : { ...previous, [location.key]: title });
  }, [location.key]);

  const [state, setState] = useState<{ stack: HistoryEntry[]; index: number }>({
    stack: [],
    index: -1,
  });

  useEffect(() => {
    const entry: HistoryEntry = {
      key: location.key,
      pathname: location.pathname,
      title: "",
    };

    setState((prevState) => {
      const { stack, index } = prevState;
      let newStack = [...stack];
      let newIndex = index;

      // Handle Initial Load
      if (stack.length === 0) {
        return { stack: [entry], index: 0 };
      }

      if (navType === NavigationType.Push) {
        // Remove forward history
        if (index < newStack.length - 1) {
          newStack = newStack.slice(0, index + 1);
        }
        newStack.push(entry);
        newIndex = newStack.length - 1;
      } else if (navType === NavigationType.Replace) {
        if (newIndex >= 0 && newIndex < newStack.length) {
          newStack[newIndex] = entry;
        } else {
          newStack.push(entry);
          newIndex = newStack.length - 1;
        }
      } else if (navType === NavigationType.Pop) {
        const foundIndex = newStack.findIndex((e) => e.key === entry.key);
        if (foundIndex !== -1) {
          newIndex = foundIndex;
        } else {
          // If not found in stack, it might be an external change or deep history we missed.
          // Fallback: treat as new push? Or just reset?
          // Let's just append it to be safe, so we don't break the app flow.
          newStack.push(entry);
          newIndex = newStack.length - 1;
        }
      }

      return { stack: newStack, index: newIndex };
    });
  }, [location, navType]);

  const canGoBack = state.index > 0;
  const canGoForward = state.index < state.stack.length - 1;

  const withTitle = (entry: HistoryEntry) => ({ ...entry, title: titles[entry.key] || "" });
  const getBackEntry = () => (canGoBack ? withTitle(state.stack[state.index - 1]) : null);
  const getForwardEntry = () =>
    canGoForward ? withTitle(state.stack[state.index + 1]) : null;

  return (
    <NavigationHistoryContext.Provider
      value={{
        historyStack: state.stack,
        currentIndex: state.index,
        canGoBack,
        canGoForward,
        getBackEntry,
        getForwardEntry,
        currentTitle: titles[location.key] || "",
        setCurrentTitle,
      }}
    >
      {children}
    </NavigationHistoryContext.Provider>
  );
};
