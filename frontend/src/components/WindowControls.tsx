import React, { useCallback, useEffect, useState } from "react";
import { Button } from "@heroui/react";
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

  const syncMaximizedState = useCallback(async () => {
    try {
      const maximized = await Window.IsMaximised();
      setIsMaximized(maximized);
    } catch {
      // ignore runtime sync errors
    }
  }, []);

  useEffect(() => {
    let disposed = false;
    const safeSync = async () => {
      if (disposed) return;
      try {
        const maximized = await Window.IsMaximised();
        if (!disposed) {
          setIsMaximized(maximized);
        }
      } catch {
        // ignore runtime sync errors
      }
    };

    void safeSync();

    const windowStateEvents = [
      "common:WindowMaximise",
      "common:WindowUnMaximise",
      "common:WindowRestore",
      "windows:WindowMaximise",
      "windows:WindowUnMaximise",
      "windows:WindowRestore",
    ];
    const offFns = windowStateEvents.map((eventName) =>
      Events.On(eventName, () => {
        void safeSync();
      }),
    );

    return () => {
      disposed = true;
      offFns.forEach((off) => off());
    };
  }, []);

  const handleToggleMaximize = () => {
    if (navLocked && !isOnboardingMode) return;
    void (async () => {
      await Window.ToggleMaximise();
      await syncMaximizedState();
    })();
  };

  return (
    <div className={`flex items-center gap-1 ${className || ""}`}>
      {!hideSeparator && (
        <div className="w-px h-6 bg-default-200 dark:bg-zinc-700 mx-2" />
      )}

      <Button
        isIconOnly
        variant="light"
        size="sm"
        radius="lg"
        aria-label={t("common.collapse")}
        isDisabled={navLocked && !isOnboardingMode}
        onPress={() => {
          if (navLocked && !isOnboardingMode) return;
          Window.Minimise();
          if (document.activeElement instanceof HTMLElement) {
            document.activeElement.blur();
          }
        }}
        className="wails-no-drag min-w-8 w-8 h-8 text-zinc-500 hover:text-zinc-900 dark:text-zinc-400 dark:hover:text-zinc-100"
      >
        <IoRemoveOutline size={20} />
      </Button>

      <Button
        isIconOnly
        variant="light"
        size="sm"
        radius="lg"
        aria-label={isMaximized ? t("common.collapse") : t("common.expand")}
        isDisabled={navLocked && !isOnboardingMode}
        onPress={handleToggleMaximize}
        className="wails-no-drag min-w-8 w-8 h-8 text-zinc-500 hover:text-zinc-900 dark:text-zinc-400 dark:hover:text-zinc-100"
      >
        {isMaximized ? (
          <IoCopyOutline size={18} />
        ) : (
          <IoSquareOutline size={18} />
        )}
      </Button>

      <Button
        isIconOnly
        variant="light"
        size="sm"
        radius="lg"
        aria-label={t("common.close")}
        isDisabled={navLocked && !isOnboardingMode}
        onPress={() => {
          if (navLocked && !isOnboardingMode) return;
          Window.Close();
          if (document.activeElement instanceof HTMLElement) {
            document.activeElement.blur();
          }
        }}
        className="wails-no-drag min-w-8 w-8 h-8 text-zinc-500 hover:text-red-600 hover:bg-red-100 dark:text-zinc-400 dark:hover:text-red-400 dark:hover:bg-red-900/20"
      >
        <IoCloseOutline size={22} />
      </Button>
    </div>
  );
};
