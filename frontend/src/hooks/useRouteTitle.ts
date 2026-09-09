import { useEffect } from "react";
import { useNavigationHistory } from "@/utils/NavigationHistoryContext";

// Titles are keyed by history entry, so returning to a detail keeps its name.
export const useRouteTitle = (title: string | null | undefined) => {
  const { setCurrentTitle } = useNavigationHistory();
  useEffect(() => {
    setCurrentTitle(title?.trim() || "");
  }, [title, setCurrentTitle]);
};
