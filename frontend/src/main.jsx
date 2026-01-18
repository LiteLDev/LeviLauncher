import "./style.css";
import i18n from "./i18n";
import { I18nextProvider } from "react-i18next";
import React from "react";
import { createRoot } from "react-dom/client";
import { ThemeProvider as NextThemesProvider } from "next-themes";
import { HeroUIProvider } from "@heroui/react";
import App from "./App";
import { HashRouter } from "react-router-dom";

const container = document.getElementById("root");

const root = createRoot(container);

root.render(
  <HeroUIProvider>
    <NextThemesProvider attribute="class" defaultTheme="light">
      <I18nextProvider i18n={i18n}>
        <React.StrictMode>
          <HashRouter>
            <App />
          </HashRouter>
        </React.StrictMode>
      </I18nextProvider>
    </NextThemesProvider>
  </HeroUIProvider>,
);
