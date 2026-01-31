import { useTheme } from "next-themes";
import React, { useEffect, useState } from "react";
import { Switch } from "@heroui/react";
import { MoonIcon } from "@/icons/MoonIcon";
import { SunIcon } from "@/icons/SunIcon";

export function ThemeSwitcher() {
  const { theme, setTheme } = useTheme();

  const [mounted, setMounted] = useState(false);

  useEffect(() => {
    setMounted(true);
  }, []);

  if (!mounted) return null;

  return (
    <Switch
      size="lg"
      color="secondary"
      isSelected={theme === "dark"}
      thumbIcon={({ isSelected, className }) =>
        isSelected ? (
          <MoonIcon className={className} />
        ) : (
          <SunIcon className={className} />
        )
      }
      onChange={(e) => {
        setTheme(theme === "light" ? "dark" : "light");
        e.target.blur();
      }}
      className="outline-none"
      classNames={{
        wrapper: "group-data-[focus-visible=true]:ring-0 group-data-[focus-visible=true]:ring-offset-0",
      }}
    ></Switch>
  );
}
