import { cn } from "@/utils/cn";
import { Button } from "@heroui/react";
import React, { useEffect, useState } from "react";

import { Events, Window } from "@wailsio/runtime";
import { useTranslation } from "react-i18next";

const controlButtonClassName =
  "wails-no-drag size-8 min-w-8 shrink-0 rounded-lg p-0 hover:bg-default data-[hovered=true]:bg-default";

const WindowControlIcon = ({
  children,
}: {
  children: React.ReactNode;
}) => (
  <svg
    aria-hidden="true"
    focusable="false"
    width="16"
    height="16"
    viewBox="0 0 16 16"
    fill="none"
    stroke="currentColor"
    strokeWidth="1.25"
    strokeLinecap="round"
    strokeLinejoin="round"
    className="size-4 shrink-0"
  >
    {children}
  </svg>
);

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
    <div className={cn("flex shrink-0 items-center gap-1", className)}>
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
        className={controlButtonClassName}
      >
        <WindowControlIcon>
          <path d="M3 8h10" />
        </WindowControlIcon>
      </Button>

      <Button
        isIconOnly
        size="sm"
        aria-label={isMaximized ? t("common.collapse") : t("common.expand")}
        isDisabled={navLocked && !isOnboardingMode}
        onPress={handleToggleMaximize}
        variant={"ghost"}
        className={controlButtonClassName}
      >
        <WindowControlIcon>
          {isMaximized ? (
            <>
              <path d="M5 5V3h8v8h-2" />
              <rect x="3" y="5" width="8" height="8" rx="0.75" />
            </>
          ) : (
            <rect x="3" y="3" width="10" height="10" rx="0.75" />
          )}
        </WindowControlIcon>
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
          controlButtonClassName,
          "hover:bg-danger hover:text-(--action-solid-foreground)! data-[hovered=true]:bg-danger data-[hovered=true]:text-(--action-solid-foreground)!",
        )}
      >
        <WindowControlIcon>
          <path d="m3 3 10 10M13 3 3 13" />
        </WindowControlIcon>
      </Button>
    </div>
  );
};
