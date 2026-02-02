import "./polyfills/wails";
import { Route, Routes, useLocation, useNavigate } from "react-router-dom";
import React, { useEffect, useState } from "react";
import {
  BaseModal,
  BaseModalHeader,
  BaseModalBody,
  BaseModalFooter,
} from "@/components/BaseModal";
import { Button, Tooltip, ModalContent } from "@heroui/react";
import { GlobalNavbar } from "@/components/GlobalNavbar";
import { LauncherPage } from "@/pages/LauncherPage";
import { DownloadManagerPage } from "@/pages/DownloadManagerPage";
import { DownloadsProvider } from "@/utils/DownloadsContext";
import { DownloadPage } from "@/pages/DownloadPage";
import { SplashScreen } from "@/pages/SplashScreen";
import { motion, AnimatePresence } from "framer-motion";
import { Events, Browser } from "@wailsio/runtime";
import { SettingsPage } from "@/pages/SettingsPage";
import { VersionSelectPage } from "@/pages/VersionSelectPage";
import VersionSettingsPage from "@/pages/VersionSettingsPage";
import ModsPage from "@/pages/ModsPage";
import UpdatingPage from "@/pages/UpdatingPage";
import ContentPage from "@/pages/ContentPage";
import WorldsListPage from "@/pages/WorldsListPage";
import WorldLevelDatEditorPage from "@/pages/WorldLevelDatEditorPage";
import ResourcePacksPage from "@/pages/ResourcePacksPage";
import BehaviorPacksPage from "@/pages/BehaviorPacksPage";
import SkinPacksPage from "@/pages/SkinPacksPage";
import { useTranslation } from "react-i18next";
import { VersionStatusProvider } from "@/utils/VersionStatusContext";
import { CurseForgeProvider } from "@/utils/CurseForgeContext";
import { LeviLaminaProvider } from "@/utils/LeviLaminaContext";
import InstallPage from "@/pages/InstallPage";
import * as minecraft from "bindings/github.com/liteldev/LeviLauncher/minecraft";
import ReactMarkdown from "react-markdown";
import remarkGfm from "remark-gfm";
import AboutPage from "@/pages/AboutPage";
import OnboardingPage from "@/pages/OnboardingPage";
import CurseForgePage from "@/pages/CurseForgePage";
import CurseForgeModPage from "@/pages/CurseForgeModPage";
import LIPPage from "@/pages/LIPPage";
import LIPPackagePage from "@/pages/LIPPackagePage";
import ServersPage from "@/pages/ServersPage";
import { Toaster } from "react-hot-toast";
import { useTheme } from "next-themes";
import { KeybindingProvider, useKeybinding } from "@/utils/KeybindingContext";
import { FaRocket } from "react-icons/fa";

const GlobalShortcuts = ({
  tryNavigate,
}: {
  tryNavigate: (path: string | number) => void;
}) => {
  const { register, unregister } = useKeybinding();
  const { t } = useTranslation();

  useEffect(() => {
    register(
      "go-back",
      "Escape",
      (e) => {
        e.preventDefault();
        tryNavigate(-1);
      },
      t("nav.back", { defaultValue: "返回" }),
    );

    return () => {
      unregister("go-back");
    };
  }, [register, unregister, tryNavigate, t]);

  return null;
};

function App() {
  const { theme, resolvedTheme } = useTheme();
  const [splashVisible, setSplashVisible] = useState(true);
  const [revealStarted, setRevealStarted] = useState(false);
  const [isFirstLoad, setIsFirstLoad] = useState(false);
  const [count, setCount] = useState(0);
  const { t, i18n } = useTranslation();
  const hasBackend = minecraft !== undefined;
  const [isBeta, setIsBeta] = useState(false);
  const [navLocked, setNavLocked] = useState<boolean>(() => {
    try {
      const h =
        typeof window !== "undefined" ? String(window.location.hash || "") : "";
      const initLock =
        h.startsWith("#/updating") ||
        h.startsWith("#/onboarding") ||
        Boolean((window as any).llNavLock);
      return initLock;
    } catch {
      return false;
    }
  });
  const [termsOpen, setTermsOpen] = useState<boolean>(false);
  const [termsCountdown, setTermsCountdown] = useState<number>(0);
  const [updateOpen, setUpdateOpen] = useState<boolean>(false);
  const [updateVersion, setUpdateVersion] = useState<string>("");
  const [updateBody, setUpdateBody] = useState<string>("");
  const [updateLoading, setUpdateLoading] = useState<boolean>(false);

  const refresh = () => {
    setCount((prevCount) => {
      return prevCount + 1;
    });
  };

  const location = useLocation();
  const navigate = useNavigate();
  const isUpdatingMode = (() => {
    const p = String(location?.pathname || "");
    if (p === "/updating") return true;
    const h =
      typeof window !== "undefined" ? String(window.location.hash || "") : "";
    return h.startsWith("#/updating");
  })();
  const isOnboardingMode = (() => {
    const p = String(location?.pathname || "");
    if (p === "/onboarding") return true;
    const h =
      typeof window !== "undefined" ? String(window.location.hash || "") : "";
    return h.startsWith("#/onboarding");
  })();

  const [hasEnteredLauncher, setHasEnteredLauncher] = useState(false);

  useEffect(() => {
    if (location.pathname === "/" && revealStarted) {
      try {
        const onboarded = localStorage.getItem("ll.onboarded");
        if (onboarded) {
          setHasEnteredLauncher(true);
        }
      } catch {}
    }
  }, [location.pathname, revealStarted]);

  useEffect(() => {
    if (isUpdatingMode || isOnboardingMode) setNavLocked(true);
    else setNavLocked(Boolean((window as any).llNavLock));
  }, [isUpdatingMode, isOnboardingMode]);

  useEffect(() => {
    if (isUpdatingMode) {
      setSplashVisible(false);
      setRevealStarted(true);
      return;
    }
    const splashDurationMs = 1400;
    const overlayFadeMs = 600;

    setIsFirstLoad(false);

    const tHide = setTimeout(() => setSplashVisible(false), splashDurationMs);
    const tHeader = setTimeout(
      () => setRevealStarted(true),
      splashDurationMs - 200,
    );
    return () => {
      clearTimeout(tHide);
      clearTimeout(tHeader);
    };
  }, [isUpdatingMode]);

  useEffect(() => {
    if (!revealStarted) return;
    if (isUpdatingMode) return;
    try {
      const onboarded = localStorage.getItem("ll.onboarded");
      if (!onboarded && location.pathname !== "/onboarding") {
        setNavLocked(true);
        navigate("/onboarding", { replace: true });
      }
    } catch {}
  }, [revealStarted, isUpdatingMode, location?.pathname]);

  useEffect(() => {
    try {
      setNavLocked(Boolean((window as any).llNavLock));
    } catch {}
    const handler = (e: any) => {
      try {
        if (isUpdatingMode) return;
        setNavLocked(Boolean(e?.detail?.lock ?? (window as any).llNavLock));
      } catch {}
    };
    window.addEventListener("ll-nav-lock-changed", handler as any);
    return () =>
      window.removeEventListener("ll-nav-lock-changed", handler as any);
  }, [isUpdatingMode]);

  useEffect(() => {
    if (!hasBackend) return;
    if (!revealStarted) return;
    if (isUpdatingMode) return;
    try {
      const accepted = localStorage.getItem("ll.termsAccepted");
      if (!accepted) {
        setTermsOpen(true);
        setNavLocked(true);
        return;
      }
      const ignored = localStorage.getItem("ll.ignoreVersion") || "";
      minecraft
        ?.CheckUpdate?.()
        .then((res: any) => {
          const ver = String(res?.version || "");
          const body = String(res?.body || "");
          const is = Boolean(res?.isUpdate);
          if (is && ver && ver !== ignored) {
            setUpdateVersion(ver);
            setUpdateBody(body);
            setUpdateOpen(true);
            setNavLocked(true);
          }
        })
        .catch(() => {});
    } catch {}
  }, [hasBackend, revealStarted, isUpdatingMode]);

  useEffect(() => {
    if (!termsOpen) return;
    setTermsCountdown(10);
    const iv = setInterval(() => {
      setTermsCountdown((v) => (v > 0 ? v - 1 : 0));
    }, 1000);
    return () => clearInterval(iv);
  }, [termsOpen]);

  useEffect(() => {
    try {
      if (updateOpen) {
        document.body.style.overflow = "hidden";
        document.documentElement.style.overflow = "hidden";
        const root = document.getElementById("root");
        if (root) (root as HTMLElement).style.overflow = "hidden";
      } else {
        document.body.style.overflow = "";
        document.documentElement.style.overflow = "";
        const root = document.getElementById("root");
        if (root) (root as HTMLElement).style.overflow = "";
      }
    } catch {}
    return () => {
      try {
        document.body.style.overflow = "";
        document.documentElement.style.overflow = "";
        const root = document.getElementById("root");
        if (root) (root as HTMLElement).style.overflow = "";
      } catch {}
    };
  }, [updateOpen]);

  const acceptTerms = () => {
    try {
      localStorage.setItem("ll.termsAccepted", "1");
    } catch {}
    setTermsOpen(false);
    setNavLocked(Boolean((window as any).llNavLock));
  };

  useEffect(() => {
    if (!hasBackend) return;
    try {
      minecraft
        ?.GetIsBeta?.()
        .then((v: boolean) => setIsBeta(!!v))
        .catch(() => {});
    } catch {}
  }, [hasBackend]);

  const tryNavigate = (path: string | number) => {
    if (navLocked) return;
    if (
      location.pathname === "/settings" ||
      location.pathname === "/version-settings"
    ) {
      try {
        window.dispatchEvent(
          new CustomEvent("ll-try-nav", { detail: { path } }),
        );
        return;
      } catch {}
    }
    navigate(path as any);
  };

  useEffect(() => {
    if (!hasBackend) return;

    const off1 = Events.On("msixvc_download_progress", () => {});
    const off2 = Events.On("msixvc_download_status", () => {});
    const off3 = Events.On("msixvc_download_error", () => {});
    const off4 = Events.On("msixvc_download_done", () => {});
    return () => {
      try {
        off1 && off1();
      } catch {}
      try {
        off2 && off2();
      } catch {}
      try {
        off3 && off3();
      } catch {}
      try {
        off4 && off4();
      } catch {}
    };
  }, [hasBackend]);

  useEffect(() => {
    const isFileDrag = (e: DragEvent) => {
      try {
        const types = e?.dataTransfer?.types;
        if (!types) return false;
        return Array.from(types).includes("Files");
      } catch {
        return false;
      }
    };
    const onDocDragOverCapture = (e: DragEvent) => {
      if (isFileDrag(e)) {
        e.preventDefault();
        try {
          (e.dataTransfer as any).dropEffect = "copy";
        } catch {}
      }
    };
    const onDocDropCapture = (e: DragEvent) => {
      if (isFileDrag(e)) {
        e.preventDefault();
      }
    };
    document.addEventListener("dragover", onDocDragOverCapture, true);
    document.addEventListener("drop", onDocDropCapture, true);
    return () => {
      document.removeEventListener("dragover", onDocDragOverCapture, true);
      document.removeEventListener("drop", onDocDropCapture, true);
    };
  }, []);

  useEffect(() => {
    const onDragStartCapture = (e: DragEvent) => {
      try {
        const target = e.target;
        if (target instanceof HTMLImageElement) {
          e.preventDefault();
        }
      } catch {}
    };
    document.addEventListener("dragstart", onDragStartCapture, true);
    return () => {
      document.removeEventListener("dragstart", onDragStartCapture, true);
    };
  }, []);

  useEffect(() => {
    const onKeyDown = (e: KeyboardEvent) => {
      if (e.key === "Tab") {
        e.preventDefault();
      }
    };
    document.addEventListener("keydown", onKeyDown, true);
    return () => {
      document.removeEventListener("keydown", onKeyDown, true);
    };
  }, []);

  return (
    <KeybindingProvider>
      <GlobalShortcuts tryNavigate={tryNavigate} />
      <VersionStatusProvider>
        <DownloadsProvider>
          <LeviLaminaProvider>
            <CurseForgeProvider>
              <Toaster
                containerStyle={{ zIndex: 99999, top: 80 }}
                toastOptions={{
                  style: {
                    maxWidth: 500,
                    background: resolvedTheme === "dark" ? "#18181b" : "#fff",
                    color: resolvedTheme === "dark" ? "#fff" : "#333",
                    border:
                      resolvedTheme === "dark"
                        ? "1px solid #27272a"
                        : "1px solid #e4e4e7",
                  },
                }}
              />
              <AnimatePresence>
                {splashVisible && (
                  <motion.div
                    key="splash-overlay"
                    className="fixed inset-0 z-9999"
                    initial={{ opacity: 1 }}
                    animate={{ opacity: 1 }}
                    exit={{ opacity: 0 }}
                    transition={{ duration: 0.6, ease: "easeOut" }}
                  >
                    <SplashScreen />
                  </motion.div>
                )}
              </AnimatePresence>

              <div
                className={`w-full min-h-dvh flex flex-col overflow-x-hidden bg-background text-foreground ${
                  updateOpen ? "overflow-y-hidden" : ""
                }`}
              >
                <GlobalNavbar
                  isBeta={isBeta}
                  navLocked={navLocked}
                  revealStarted={revealStarted}
                  isUpdatingMode={isUpdatingMode}
                  isOnboardingMode={isOnboardingMode}
                  hasEnteredLauncher={hasEnteredLauncher}
                  tryNavigate={tryNavigate}
                />

                <div className="h-[69px]" />

                <motion.div
                  className="w-full flex-1 min-h-0 overflow-hidden"
                  initial={{ opacity: 0 }}
                  animate={{ opacity: revealStarted ? 1 : 0 }}
                  transition={{ duration: 0.6, ease: "easeOut" }}
                  style={{ pointerEvents: revealStarted ? "auto" : "none" }}
                >
                  {revealStarted &&
                    (isFirstLoad ? (
                      <></>
                    ) : (
                      <Routes>
                        <Route
                          path="/"
                          element={
                            <LauncherPage refresh={refresh} count={count} />
                          }
                        />
                        <Route path="/download" element={<DownloadPage />} />
                        <Route
                          path="/tasks"
                          element={<DownloadManagerPage />}
                        />
                        <Route path="/install" element={<InstallPage />} />
                        <Route path="/settings" element={<SettingsPage />} />
                        <Route
                          path="/versions"
                          element={<VersionSelectPage refresh={refresh} />}
                        />
                        <Route
                          path="/version-settings"
                          element={<VersionSettingsPage />}
                        />
                        <Route path="/mods" element={<ModsPage />} />
                        <Route
                          path="/curseforge"
                          element={<CurseForgePage />}
                        />
                        <Route
                          path="/curseforge/mod/:id"
                          element={<CurseForgeModPage />}
                        />
                        <Route path="/lip" element={<LIPPage />} />
                        <Route
                          path="/lip/package/:id"
                          element={<LIPPackagePage />}
                        />
                        <Route path="/updating" element={<UpdatingPage />} />
                        <Route
                          path="/onboarding"
                          element={<OnboardingPage />}
                        />
                        <Route path="/content" element={<ContentPage />} />
                        <Route
                          path="/content/worlds"
                          element={<WorldsListPage />}
                        />
                        <Route
                          path="/content/world-edit"
                          element={<WorldLevelDatEditorPage />}
                        />
                        <Route
                          path="/content/resource-packs"
                          element={<ResourcePacksPage />}
                        />
                        <Route
                          path="/content/behavior-packs"
                          element={<BehaviorPacksPage />}
                        />
                        <Route
                          path="/content/skin-packs"
                          element={<SkinPacksPage />}
                        />
                        <Route path="/servers" element={<ServersPage />} />
                        <Route path="/about" element={<AboutPage />} />
                      </Routes>
                    ))}
                </motion.div>

                <BaseModal
                  size="lg"
                  isOpen={termsOpen}
                  hideCloseButton
                  isDismissable={false}
                >
                  <ModalContent>
                    {() => (
                      <>
                        <BaseModalHeader className="text-primary-700 text-[18px] sm:text-[20px] font-bold antialiased">
                          {t("terms.title", { defaultValue: "用户协议" })}
                        </BaseModalHeader>
                        <BaseModalBody>
                          <div className="text-[15px] sm:text-[16px] leading-7 text-default-900 font-medium antialiased whitespace-pre-wrap wrap-break-word max-h-[56vh] overflow-y-auto pr-1">
                            {t("terms.body", {
                              defaultValue:
                                "在使用本启动器之前，请仔细阅读并同意《用户协议》和相关条款。继续使用即表示您已同意。",
                            })}
                          </div>
                        </BaseModalBody>
                        <BaseModalFooter>
                          <Button
                            variant="light"
                            onPress={() => {
                              Window.Close();
                            }}
                          >
                            {t("terms.decline", {
                              defaultValue: "不同意，退出",
                            })}
                          </Button>
                          <Button
                            color="primary"
                            isDisabled={termsCountdown > 0}
                            onPress={acceptTerms}
                          >
                            {termsCountdown > 0
                              ? `${t("terms.agree", {
                                  defaultValue: "同意并继续",
                                })} (${termsCountdown}s)`
                              : t("terms.agree", {
                                  defaultValue: "同意并继续",
                                })}
                          </Button>
                        </BaseModalFooter>
                      </>
                    )}
                  </ModalContent>
                </BaseModal>

                <BaseModal size="md" isOpen={updateOpen} hideCloseButton>
                  <ModalContent>
                    {(onClose) => (
                      <>
                        <BaseModalHeader className="flex-row items-center gap-2 text-primary-600 min-w-0">
                          <FaRocket className="w-5 h-5" />
                          <span className="truncate">
                            {t("settings.body.version.hasnew", {
                              defaultValue: "有新的版本更新！",
                            })}
                            {updateVersion}
                          </span>
                        </BaseModalHeader>
                        <BaseModalBody>
                          {updateBody ? (
                            <div className="rounded-md bg-default-100/60 border border-default-200 px-3 py-2">
                              <div className="text-small font-semibold mb-1">
                                {t("downloadpage.changelog.title", {
                                  defaultValue: "最新更新日志",
                                })}
                              </div>
                              <div className="text-small wrap-break-word leading-6 max-h-[32vh] sm:max-h-[40vh] lg:max-h-[44vh] overflow-y-auto pr-1">
                                <ReactMarkdown
                                  remarkPlugins={[remarkGfm]}
                                  components={{
                                    h1: ({ children }) => (
                                      <h1 className="text-xl font-semibold mt-2 mb-2">
                                        {children}
                                      </h1>
                                    ),
                                    h2: ({ children }) => (
                                      <h2 className="text-lg font-semibold mt-2 mb-2">
                                        {children}
                                      </h2>
                                    ),
                                    h3: ({ children }) => (
                                      <h3 className="text-base font-semibold mt-2 mb-2">
                                        {children}
                                      </h3>
                                    ),
                                    p: ({ children }) => (
                                      <p className="my-1">{children}</p>
                                    ),
                                    ul: ({ children }) => (
                                      <ul className="list-disc pl-6 my-2">
                                        {children}
                                      </ul>
                                    ),
                                    ol: ({ children }) => (
                                      <ol className="list-decimal pl-6 my-2">
                                        {children}
                                      </ol>
                                    ),
                                    li: ({ children }) => (
                                      <li className="my-1">{children}</li>
                                    ),
                                    a: ({ href, children }) => {
                                      const cleanUrl = (url: string) => {
                                        const target = "https://github.com";
                                        const idx = url.lastIndexOf(target);
                                        return idx > 0
                                          ? url.substring(idx)
                                          : url;
                                      };

                                      return (
                                        <a
                                          href={href}
                                          target="_blank"
                                          rel="noreferrer"
                                          className="text-primary underline cursor-pointer"
                                          onClick={(e) => {
                                            e.preventDefault();
                                            if (href) {
                                              Browser.OpenURL(cleanUrl(href));
                                            }
                                          }}
                                        >
                                          {Array.isArray(children)
                                            ? children.map((child) =>
                                                typeof child === "string"
                                                  ? cleanUrl(child)
                                                  : child,
                                              )
                                            : typeof children === "string"
                                              ? cleanUrl(children)
                                              : children}
                                        </a>
                                      );
                                    },
                                    hr: () => (
                                      <hr className="my-3 border-default-200" />
                                    ),
                                  }}
                                >
                                  {updateBody}
                                </ReactMarkdown>
                              </div>
                            </div>
                          ) : null}
                        </BaseModalBody>
                        <BaseModalFooter>
                          <Button
                            variant="light"
                            onPress={() => {
                              setUpdateOpen(false);
                              setNavLocked(Boolean((window as any).llNavLock));
                              onClose();
                            }}
                          >
                            {t("common.cancel", { defaultValue: "取消" })}
                          </Button>
                          <Button
                            variant="flat"
                            onPress={() => {
                              try {
                                localStorage.setItem(
                                  "ll.ignoreVersion",
                                  updateVersion || "",
                                );
                              } catch {}
                              setUpdateOpen(false);
                              setNavLocked(Boolean((window as any).llNavLock));
                              onClose();
                            }}
                          >
                            {t("settings.body.version.ignore", {
                              defaultValue: "屏蔽该版本",
                            })}
                          </Button>
                          <Button
                            color="primary"
                            isLoading={updateLoading}
                            onPress={async () => {
                              setUpdateLoading(true);
                              try {
                                setUpdateOpen(false);
                                setNavLocked(true);
                                onClose();
                                navigate("/updating", { replace: true });
                              } finally {
                                setUpdateLoading(false);
                              }
                            }}
                          >
                            {t("settings.modal.2.footer.download_button", {
                              defaultValue: "更新",
                            })}
                          </Button>
                        </BaseModalFooter>
                      </>
                    )}
                  </ModalContent>
                </BaseModal>
              </div>
            </CurseForgeProvider>
          </LeviLaminaProvider>
        </DownloadsProvider>
      </VersionStatusProvider>
    </KeybindingProvider>
  );
}

export default App;
