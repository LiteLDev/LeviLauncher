import { createContext, useContext } from "react";
import type { useBackgroundImage } from "@/hooks/useBackgroundImage";

export type BackgroundState = ReturnType<typeof useBackgroundImage>;
export const BackgroundContext = createContext<BackgroundState | null>(null);
export const useCurrentBackground = () => useContext(BackgroundContext);
