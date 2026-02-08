import { useTheme } from "next-themes";
import React, { useEffect, useState } from "react";
import { Button } from "@heroui/react";
import { MoonIcon } from "@/icons/MoonIcon";
import { SunIcon } from "@/icons/SunIcon";

export interface ThemeSwitcherProps {
  className?: string;
  iconSize?: number;
}

export function ThemeSwitcher({
  className,
  iconSize = 20,
}: ThemeSwitcherProps) {
  const { theme, setTheme } = useTheme();
  const [mounted, setMounted] = useState(false);

  useEffect(() => {
    setMounted(true);
  }, []);

  if (!mounted) return <div className={className || "w-8 h-8"} />;

  const isDark = theme === "dark";

  return (
    <Button
      isIconOnly
      variant="light"
      size="sm"
      radius="lg"
      onPress={() => setTheme(isDark ? "light" : "dark")}
      aria-label="Toggle theme"
      className={
        className ||
        "w-8 h-8 text-zinc-500 hover:text-zinc-900 dark:text-zinc-400 dark:hover:text-zinc-100"
      }
    >
      {isDark ? (
        <MoonIcon className={`w-[${iconSize}px] h-[${iconSize}px]`} />
      ) : (
        <SunIcon className={`w-[${iconSize}px] h-[${iconSize}px]`} />
      )}
    </Button>
  );
}
