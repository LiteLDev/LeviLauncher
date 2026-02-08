import React, { useState } from "react";
import { Button } from "@heroui/react";
import {
  IoRemoveOutline,
  IoSquareOutline,
  IoCopyOutline,
  IoCloseOutline,
} from "react-icons/io5";
import { Window } from "@wailsio/runtime";

interface WindowControlsProps {
  navLocked: boolean;
  isOnboardingMode: boolean;
}

export const WindowControls: React.FC<WindowControlsProps> = ({
  navLocked,
  isOnboardingMode,
}) => {
  const [isMaximized, setIsMaximized] = useState(false);

  const handleToggleMaximize = () => {
    if (navLocked && !isOnboardingMode) return;
    Window.ToggleMaximise();
    setIsMaximized(!isMaximized);
  };

  return (
    <div className="flex items-center gap-1">
      <div className="w-px h-6 bg-default-200 dark:bg-zinc-700 mx-2" />

      <Button
        isIconOnly
        variant="light"
        size="sm"
        radius="lg"
        aria-label="Minimize"
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
        aria-label="Maximize"
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
        aria-label="Close"
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
