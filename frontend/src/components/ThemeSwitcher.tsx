import { useTheme } from "next-themes";
import React, { useEffect, useState } from "react";
import { Button } from "@heroui/react";
import { MoonIcon } from "@/icons/MoonIcon";
import { SunIcon } from "@/icons/SunIcon";
import { useThemeManager } from "@/utils/useThemeManager";

export interface ThemeSwitcherProps {
  className?: string;
  iconSize?: number;
  isDisabled?: boolean;
}

export function ThemeSwitcher({
  className,
  iconSize = 20,
  isDisabled,
}: ThemeSwitcherProps) {
  const { toggleTheme, resolvedTheme } = useThemeManager();
  const [mounted, setMounted] = useState(false);

  useEffect(() => {
    setMounted(true);
  }, []);

  if (!mounted) return <div className={className || "w-8 h-8"} />;

  const isDark = resolvedTheme === "dark";

  return (
    <Button
      isIconOnly
      variant="light"
      size="sm"
      radius="lg"
      onPress={toggleTheme}
      isDisabled={isDisabled}
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
