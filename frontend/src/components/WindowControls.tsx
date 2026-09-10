import { cn } from "@/utils/cn";
import { Button } from "@heroui/react";
import React, { useEffect, useState } from "react";

import {
  IoRemoveOutline,
  IoSquareOutline,
  IoCopyOutline,
  IoCloseOutline,
} from "react-icons/io5";
import { Events, Window } from "@wailsio/runtime";
import { useTranslation } from "react-i18next";

interface WindowControlsProps {
  navLocked: boolean;
  isOnboardingMode: boolean;
  hideSeparator?: boolean;
  className?: string;
}

export const WindowControls: React.FC<WindowControlsProps> = ({
  navLocked,
  isOnboardingMode,
  hideSeparator,
  className,
}) => {
  const { t } = useTranslation();
  const [isMaximized, setIsMaximized] = useState(false);

  useEffect(() => {
    let disposed = false;
    let revision = 0;
    const safeSync = async () => {
      if (disposed) return;
      const currentRevision = ++revision;
      try {
        const maximized = await Window.IsMaximised();
        if (!disposed && currentRevision === revision) {
          setIsMaximized(maximized);
        }
      } catch {
        // ignore runtime sync errors
      }
    };

    const windowStateEvents = [
      Events.Types.Common.WindowMaximise,
      Events.Types.Common.WindowUnMaximise,
      Events.Types.Common.WindowRestore,
    ];
    const offFns = windowStateEvents.map((eventName) =>
      Events.On(eventName, () => {
        void safeSync();
      }),
    );
    void safeSync();

    return () => {
      disposed = true;
      offFns.forEach((off) => off());
    };
  }, []);

  const handleToggleMaximize = () => {
    if (navLocked && !isOnboardingMode) return;
    void Window.ToggleMaximise().catch((error) => {
      console.warn("Failed to toggle window maximization", error);
    });
  };

  return (
    <div className={`flex items-center gap-1 ${className || ""}`}>
      {!hideSeparator && (
        <div className="w-px h-6 bg-surface-tertiary mx-2" />
      )}

      <Button
        isIconOnly
        size="sm"
        aria-label={t("common.collapse")}
        isDisabled={navLocked && !isOnboardingMode}
        onPress={() => {
          if (navLocked && !isOnboardingMode) return;
          Window.Minimise();
          if (document.activeElement instanceof HTMLElement) {
            document.activeElement.blur();
          }
        }}
        variant={"ghost"}
        className={cn(
          "rounded-lg",
          "wails-no-drag min-w-8 w-8 h-8 text-zinc-500 hover:text-zinc-900 dark:text-zinc-400 dark:hover:text-zinc-100",
        )}
      >
        <IoRemoveOutline size={20} />
      </Button>

      <Button
        isIconOnly
        size="sm"
        aria-label={isMaximized ? t("common.collapse") : t("common.expand")}
        isDisabled={navLocked && !isOnboardingMode}
        onPress={handleToggleMaximize}
        variant={"ghost"}
        className={cn(
          "rounded-lg",
          "wails-no-drag min-w-8 w-8 h-8 text-zinc-500 hover:text-zinc-900 dark:text-zinc-400 dark:hover:text-zinc-100",
        )}
      >
        {isMaximized ? (
          <IoCopyOutline size={18} />
        ) : (
          <IoSquareOutline size={18} />
        )}
      </Button>

      <Button
        isIconOnly
        size="sm"
        aria-label={t("common.close")}
        isDisabled={navLocked && !isOnboardingMode}
        onPress={() => {
          if (navLocked && !isOnboardingMode) return;
          Window.Close();
          if (document.activeElement instanceof HTMLElement) {
            document.activeElement.blur();
          }
        }}
        variant={"ghost"}
        className={cn(
          "rounded-lg",
          "wails-no-drag min-w-8 w-8 h-8 text-zinc-500 hover:text-red-600 hover:bg-red-100 dark:text-zinc-400 dark:hover:text-red-400 dark:hover:bg-red-900/20",
        )}
      >
        <IoCloseOutline size={22} />
      </Button>
    </div>
  );
};
