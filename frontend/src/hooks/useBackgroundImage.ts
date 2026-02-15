import { useState, useEffect } from "react";
import * as minecraft from "bindings/github.com/liteldev/LeviLauncher/minecraft";

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

    const playOrder =
      localStorage.getItem("app.backgroundPlayOrder") || "random";

    if (playOrder === "sequential") {
      const lastIdxKey = "app.backgroundLastIndex";
      let lastIdx = parseInt(localStorage.getItem(lastIdxKey) || "-1");
      let nextIdx = lastIdx + 1;
      if (nextIdx >= images.length) {
        nextIdx = 0;
      }
      localStorage.setItem(lastIdxKey, String(nextIdx));
      return images[nextIdx].path;
    } else {
      const randomIdx = Math.floor(Math.random() * images.length);
      return images[randomIdx].path;
    }
  } catch (err) {
    console.error("Failed to pick next image:", err);
    return "";
  }
};

export const useBackgroundImage = () => {
  const [backgroundImagePath, setBackgroundImagePath] = useState<string>(
    () => localStorage.getItem("app.backgroundImage") || "",
  );
  const [backgroundFitMode, setBackgroundFitMode] = useState<string>(
    () => localStorage.getItem("app.backgroundFitMode") || "smart",
  );
  const [bgData, setBgData] = useState<string>("");

  const [backgroundBlur, setBackgroundBlur] = useState<number>(() =>
    Number(localStorage.getItem("app.backgroundBlur") || "0"),
  );

  const [backgroundBrightness, setBackgroundBrightness] = useState<number>(
    () => {
      const item = localStorage.getItem("app.backgroundBrightness");
      return item !== null ? Number(item) : 100;
    },
  );

  const [backgroundOpacity, setBackgroundOpacity] = useState<number>(() => {
    const item = localStorage.getItem("app.backgroundOpacity");
    return item !== null ? Number(item) : 100;
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
      return item !== null ? Number(item) : 50;
    });

  const [darkBackgroundBaseOpacity, setDarkBackgroundBaseOpacity] =
    useState<number>(() => {
      const item = localStorage.getItem("app.darkBackgroundBaseOpacity");
      return item !== null ? Number(item) : 50;
    });

  useEffect(() => {
    const initBackgrounds = async () => {
      try {
        const folder = localStorage.getItem("app.backgroundImage") || "";
        const img = await pickNextImage(folder);

        setBackgroundImagePath(img);
        localStorage.setItem("app.currentBackgroundImage", img);
      } catch {}
    };
    initBackgrounds();
  }, []);

  useEffect(() => {
    const handler = () => {
      try {
        const val = Number(localStorage.getItem("app.backgroundBlur") || "0");
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
        setBackgroundOpacity(item !== null ? Number(item) : 100);
      } catch {}
    };
    window.addEventListener("app-opacity-changed", handler);
    return () => window.removeEventListener("app-opacity-changed", handler);
  }, []);

  useEffect(() => {
    const handler = () => {
      try {
        const item = localStorage.getItem("app.backgroundBrightness");
        setBackgroundBrightness(item !== null ? Number(item) : 100);
      } catch {}
    };
    window.addEventListener("app-brightness-changed", handler);
    return () => window.removeEventListener("app-brightness-changed", handler);
  }, []);

  useEffect(() => {
    const handler = async () => {
      try {
        const folder = localStorage.getItem("app.backgroundImage") || "";
        const img = await pickNextImage(folder);

        setBackgroundImagePath(img);
        localStorage.setItem("app.currentBackgroundImage", img);
      } catch {}
    };
    window.addEventListener("app-background-changed", handler);
    return () => window.removeEventListener("app-background-changed", handler);
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
          lightOpacity !== null ? Number(lightOpacity) : 50,
        );

        const darkOpacity = localStorage.getItem(
          "app.darkBackgroundBaseOpacity",
        );
        setDarkBackgroundBaseOpacity(
          darkOpacity !== null ? Number(darkOpacity) : 50,
        );
      } catch {}
    };
    window.addEventListener("app-background-settings-changed", handler);
    return () =>
      window.removeEventListener("app-background-settings-changed", handler);
  }, []);

  useEffect(() => {
    const currentImg = backgroundImagePath;

    if (!currentImg) {
      setBgData("");
      return;
    }
    if (currentImg.startsWith("data:") || currentImg.startsWith("http")) {
      setBgData(currentImg);
      return;
    }

    (minecraft as any)
      .GetImageBase64?.(currentImg)
      .then((res: string) => {
        if (res) setBgData(res);
      })
      .catch(() => {});
  }, [backgroundImagePath]);

  return {
    bgData,
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
