import { useState, useEffect } from "react";
import * as minecraft from "bindings/github.com/liteldev/LeviLauncher/internal/app/minecraft";
import { useStartupInteractive } from "@/utils/startupState";
import { clampNumber } from "@/utils/backgroundAppearance";

const readNumber = (key: string, fallback: number, max: number) =>
  clampNumber(localStorage.getItem(`app.${key}`), fallback, 0, max);

export const getFitStyles = (mode: string) => {
  switch (mode) {
    case "center":
      return {
        backgroundSize: "auto",
        backgroundPosition: "center",
        backgroundRepeat: "no-repeat",
      };
    case "fit":
      return {
        backgroundSize: "contain",
        backgroundPosition: "center",
        backgroundRepeat: "no-repeat",
      };
    case "stretch":
      return {
        backgroundSize: "100% 100%",
        backgroundPosition: "center",
        backgroundRepeat: "no-repeat",
      };
    case "tile":
      return {
        backgroundSize: "auto",
        backgroundPosition: "top left",
        backgroundRepeat: "repeat",
      };
    case "top_left":
      return {
        backgroundSize: "auto",
        backgroundPosition: "top left",
        backgroundRepeat: "no-repeat",
      };
    case "top_right":
      return {
        backgroundSize: "auto",
        backgroundPosition: "top right",
        backgroundRepeat: "no-repeat",
      };
    case "smart":
    default:
      return {
        backgroundSize: "cover",
        backgroundPosition: "center",
        backgroundRepeat: "no-repeat",
      };
  }
};

const pickNextImage = async (folderPath: string) => {
  if (!folderPath) return "";
  try {
    const entries = await (minecraft as any).ListDir(folderPath);
    if (!entries || entries.length === 0) return "";
    const images = entries.filter((e: any) => {
      const name = e.name.toLowerCase();
      return (
        !e.isDir &&
        (name.endsWith(".png") ||
          name.endsWith(".jpg") ||
          name.endsWith(".jpeg") ||
          name.endsWith(".webp") ||
          name.endsWith(".gif") ||
          name.endsWith(".bmp"))
      );
    });
    if (images.length === 0) return "";
    images.sort((a: any, b: any) => a.name.localeCompare(b.name, undefined, { numeric: true }));
    const currentPath = localStorage.getItem("app.currentBackgroundImage");

    const playOrder =
      localStorage.getItem("app.backgroundPlayOrder") || "random";

    if (playOrder === "sequential") {
      const currentIndex = images.findIndex((image: any) => image.path === currentPath);
      return images[(currentIndex + 1) % images.length].path;
    } else {
      const candidates = images.length > 1 ? images.filter((image: any) => image.path !== currentPath) : images;
      return candidates[Math.floor(Math.random() * candidates.length)].path;
    }
  } catch (err) {
    console.error("Failed to pick next image:", err);
    return "";
  }
};

export const useBackgroundImage = () => {
  const startupInteractive = useStartupInteractive();
  const [backgroundImagePath, setBackgroundImagePath] = useState<string>("");
  const [backgroundRevision, setBackgroundRevision] = useState(0);
  const [backgroundReady, setBackgroundReady] = useState<boolean>(false);
  const [backgroundFitMode, setBackgroundFitMode] = useState<string>(
    () => localStorage.getItem("app.backgroundFitMode") || "smart",
  );
  const [bgData, setBgData] = useState<string>("");

  const [backgroundBlur, setBackgroundBlur] = useState<number>(() =>
    readNumber("backgroundBlur", 0, 50),
  );

  const [backgroundBrightness, setBackgroundBrightness] = useState<number>(
    () => {
      const item = localStorage.getItem("app.backgroundBrightness");
      return clampNumber(item, 100, 0, 200);
    },
  );

  const [backgroundOpacity, setBackgroundOpacity] = useState<number>(() => {
    const item = localStorage.getItem("app.backgroundOpacity");
    return clampNumber(item, 100, 0, 100);
  });

  const [lightBackgroundBaseMode, setLightBackgroundBaseMode] =
    useState<string>(
      () => localStorage.getItem("app.lightBackgroundBaseMode") || "none",
    );

  const [darkBackgroundBaseMode, setDarkBackgroundBaseMode] = useState<string>(
    () => localStorage.getItem("app.darkBackgroundBaseMode") || "none",
  );

  const [lightBackgroundBaseColor, setLightBackgroundBaseColor] =
    useState<string>(
      () => localStorage.getItem("app.lightBackgroundBaseColor") || "#ffffff",
    );

  const [darkBackgroundBaseColor, setDarkBackgroundBaseColor] =
    useState<string>(
      () => localStorage.getItem("app.darkBackgroundBaseColor") || "#18181b",
    );

  const [lightBackgroundBaseOpacity, setLightBackgroundBaseOpacity] =
    useState<number>(() => {
      const item = localStorage.getItem("app.lightBackgroundBaseOpacity");
      return clampNumber(item, 50, 0, 100);
    });

  const [darkBackgroundBaseOpacity, setDarkBackgroundBaseOpacity] =
    useState<number>(() => {
      const item = localStorage.getItem("app.darkBackgroundBaseOpacity");
      return clampNumber(item, 50, 0, 100);
    });

  useEffect(() => {
    if (!startupInteractive) return;
    let request = 0;
    const initBackgrounds = async () => {
      const currentRequest = ++request;
      try {
        const folder = localStorage.getItem("app.backgroundImage") || "";
        const img = await pickNextImage(folder);
        if (currentRequest !== request) return;
        setBackgroundImagePath(img);
        setBackgroundRevision((revision) => revision + 1);
        localStorage.setItem("app.currentBackgroundImage", img);
      } catch {}
    };
    void initBackgrounds();
    window.addEventListener("app-background-changed", initBackgrounds);
    return () => {
      ++request;
      window.removeEventListener("app-background-changed", initBackgrounds);
    };
  }, [startupInteractive]);

  useEffect(() => {
    const handler = () => {
      try {
        const val = readNumber("backgroundBlur", 0, 50);
        setBackgroundBlur(val);
      } catch {}
    };
    window.addEventListener("app-blur-changed", handler);
    return () => window.removeEventListener("app-blur-changed", handler);
  }, []);

  useEffect(() => {
    const handler = () => {
      try {
        const item = localStorage.getItem("app.backgroundOpacity");
        setBackgroundOpacity(clampNumber(item, 100, 0, 100));
      } catch {}
    };
    window.addEventListener("app-opacity-changed", handler);
    return () => window.removeEventListener("app-opacity-changed", handler);
  }, []);

  useEffect(() => {
    const handler = () => {
      try {
        const item = localStorage.getItem("app.backgroundBrightness");
        setBackgroundBrightness(clampNumber(item, 100, 0, 200));
      } catch {}
    };
    window.addEventListener("app-brightness-changed", handler);
    return () => window.removeEventListener("app-brightness-changed", handler);
  }, []);

  useEffect(() => {
    const handler = () => {
      try {
        setBackgroundFitMode(
          localStorage.getItem("app.backgroundFitMode") || "smart",
        );

        setLightBackgroundBaseMode(
          localStorage.getItem("app.lightBackgroundBaseMode") || "none",
        );
        setDarkBackgroundBaseMode(
          localStorage.getItem("app.darkBackgroundBaseMode") || "none",
        );

        setLightBackgroundBaseColor(
          localStorage.getItem("app.lightBackgroundBaseColor") || "#ffffff",
        );
        setDarkBackgroundBaseColor(
          localStorage.getItem("app.darkBackgroundBaseColor") || "#18181b",
        );

        const lightOpacity = localStorage.getItem(
          "app.lightBackgroundBaseOpacity",
        );
        setLightBackgroundBaseOpacity(
          clampNumber(lightOpacity, 50, 0, 100),
        );

        const darkOpacity = localStorage.getItem(
          "app.darkBackgroundBaseOpacity",
        );
        setDarkBackgroundBaseOpacity(
          clampNumber(darkOpacity, 50, 0, 100),
        );
      } catch {}
    };
    window.addEventListener("app-background-settings-changed", handler);
    return () =>
      window.removeEventListener("app-background-settings-changed", handler);
  }, []);

  useEffect(() => {
    if (!startupInteractive) return;
    const currentImg = backgroundImagePath;
    let cancelled = false;
    let cancelDecode: (() => void) | undefined;

    if (!currentImg) {
      setBgData("");
      setBackgroundReady(true);
      return;
    }
    setBackgroundReady(false);
    const loadBackground = async () => {
      try {
        let source = /^(data:|https?:)/.test(currentImg) ? currentImg : "";
        if (!source && typeof (minecraft as any).GetImageURL === "function") {
          source = await (minecraft as any).GetImageURL(currentImg);
        }
        if (
          !source &&
          typeof (minecraft as any).GetImageBase64 === "function"
        ) {
          source = await (minecraft as any).GetImageBase64(currentImg);
        }
        if (cancelled) return;

        if (!source) {
          setBgData("");
          setBackgroundReady(true);
          return;
        }

        const loaded = await new Promise<boolean>((resolve) => {
          const image = new Image();
          const finish = (success: boolean) => {
            window.clearTimeout(timeout);
            image.onload = null;
            image.onerror = null;
            resolve(success);
          };
          const timeout = window.setTimeout(() => finish(false), 10000);
          cancelDecode = () => { finish(false); image.src = ""; };
          image.onload = () => finish(true);
          image.onerror = () => finish(false);
          image.src = source;
        });
        if (cancelled) return;

        setBgData(loaded ? source : "");
        setBackgroundReady(true);
      } catch {
        if (cancelled) return;
        setBgData("");
        setBackgroundReady(true);
      }
    };

    void loadBackground();
    return () => {
      cancelled = true;
      cancelDecode?.();
    };
  }, [backgroundImagePath, backgroundRevision, startupInteractive]);

  return {
    bgData,
    backgroundReady,
    backgroundFitMode,
    backgroundBlur,
    backgroundBrightness,
    backgroundOpacity,
    lightBackgroundBaseMode,
    darkBackgroundBaseMode,
    lightBackgroundBaseColor,
    darkBackgroundBaseColor,
    lightBackgroundBaseOpacity,
    darkBackgroundBaseOpacity,
    getFitStyles,
  };
};
