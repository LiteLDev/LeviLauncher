import React, {
  createContext,
  useContext,
  useEffect,
  useState,
  useCallback,
} from "react";

type KeybindingCallback = (e: KeyboardEvent) => void;

interface Keybinding {
  id: string;
  combo: string;
  callback: KeybindingCallback;
  description?: string;
}

interface KeybindingContextType {
  register: (
    id: string,
    combo: string,
    callback: KeybindingCallback,
    description?: string,
  ) => void;
  unregister: (id: string) => void;
  bindings: Keybinding[];
}

const KeybindingContext = createContext<KeybindingContextType | undefined>(
  undefined,
);

export const useKeybinding = () => {
  const context = useContext(KeybindingContext);
  if (!context) {
    throw new Error("useKeybinding must be used within a KeybindingProvider");
  }
  return context;
};

const parseCombo = (combo: string) => {
  const parts = combo
    .toLowerCase()
    .split("+")
    .map((p) => p.trim());
  return {
    ctrl: parts.includes("ctrl") || parts.includes("control"),
    shift: parts.includes("shift"),
    alt: parts.includes("alt"),
    meta:
      parts.includes("meta") ||
      parts.includes("cmd") ||
      parts.includes("command"),
    key: parts.filter(
      (p) =>
        !["ctrl", "control", "shift", "alt", "meta", "cmd", "command"].includes(
          p,
        ),
    )[0],
  };
};

/** Elements that own the keyboard while they are open. */
export const OVERLAY_SELECTOR = '[role="dialog"], [role="menu"], [role="listbox"]';

export const isOverlayOpen = () =>
  typeof document !== "undefined" &&
  Boolean(document.querySelector(OVERLAY_SELECTOR));

const KEY_LABELS: Record<string, string> = {
  ctrl: "Ctrl",
  control: "Ctrl",
  shift: "Shift",
  alt: "Alt",
  meta: "Win",
  cmd: "Win",
  command: "Win",
  escape: "Esc",
  arrowleft: "←",
  arrowright: "→",
  arrowup: "↑",
  arrowdown: "↓",
  enter: "Enter",
};

/** Splits a combo into display labels, e.g. "ctrl+arrowleft" -> ["Ctrl", "←"]. */
export const formatCombo = (combo: string): string[] => {
  const parts = combo
    .split("+")
    .map((p) => p.trim())
    .filter(Boolean);
  // Shift is implied by printable symbols such as "?".
  const key = parts[parts.length - 1] || "";
  const visible =
    key.length === 1 && !/[a-z0-9]/i.test(key)
      ? parts.filter((p) => p.toLowerCase() !== "shift")
      : parts;
  return visible.map(
    (p) => KEY_LABELS[p.toLowerCase()] ?? (p.length === 1 ? p.toUpperCase() : p),
  );
};

export const KeybindingProvider: React.FC<{ children: React.ReactNode }> = ({
  children,
}) => {
  const [bindings, setBindings] = useState<Keybinding[]>([]);

  const register = useCallback(
    (
      id: string,
      combo: string,
      callback: KeybindingCallback,
      description?: string,
    ) => {
      setBindings((prev) => {
        const filtered = prev.filter((b) => b.id !== id);
        return [...filtered, { id, combo, callback, description }];
      });
    },
    [],
  );

  const unregister = useCallback((id: string) => {
    setBindings((prev) => prev.filter((b) => b.id !== id));
  }, []);

  useEffect(() => {
    const handleKeyDown = (e: KeyboardEvent) => {
      // Overlay and input handlers own keys they have already consumed.
      if (e.defaultPrevented) return;
      const target = e.target as HTMLElement;
      if (
        e.key === "Escape" &&
        (target.closest(OVERLAY_SELECTOR) || isOverlayOpen())
      ) {
        return;
      }
      const isInput = ["INPUT", "TEXTAREA", "SELECT"].includes(target.tagName);

      bindings.forEach((binding) => {
        const parsed = parseCombo(binding.combo);

        const eventKey = e.key.toLowerCase();
        const bindingKey = parsed.key;

        if (parsed.ctrl !== e.ctrlKey) return;
        if (parsed.shift !== e.shiftKey) return;
        if (parsed.alt !== e.altKey) return;
        if (parsed.meta !== e.metaKey) return;

        if (bindingKey !== eventKey) {
          if (bindingKey !== eventKey) return;
        }

        const hasModifier = parsed.ctrl || parsed.alt || parsed.meta;
        if (isInput && !hasModifier && !/^F\d+$/.test(binding.combo)) {
          return;
        }

        binding.callback(e);
      });
    };

    window.addEventListener("keydown", handleKeyDown);
    return () => window.removeEventListener("keydown", handleKeyDown);
  }, [bindings]);

  return (
    <KeybindingContext.Provider value={{ register, unregister, bindings }}>
      {children}
    </KeybindingContext.Provider>
  );
};
