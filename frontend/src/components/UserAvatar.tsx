import { Avatar, Button, Popover, Spinner, Tooltip, toast } from "@heroui/react";
import { cn } from "@/utils/cn";

import React, { useEffect, useState } from "react";
import { useTranslation } from "react-i18next";

import {
  FaSync,
  FaUser,
  FaQuestionCircle,
  FaCheckCircle,
  FaExclamationCircle,
  FaShieldAlt,
} from "react-icons/fa";
import * as userService from "bindings/github.com/liteldev/LeviLauncher/internal/app/userservice";
import { useStartupInteractive } from "@/utils/startupState";
import { useProcessElevation } from "@/hooks/useProcessElevation";
import { Browser, Events, System } from "@wailsio/runtime";
import { FiActivity, FiExternalLink } from "react-icons/fi";
import { OPEN_ACCOUNT_EVENT, useDiagnostics } from "@/utils/DiagnosticsContext";
import { publishLicenses } from "@/utils/licenseStore";
import { faqUrl, licenseInsight, type LicenseState } from "@/utils/troubleshoot";
import type { FaqId } from "@/utils/faqEntries";
import { UnifiedModal } from "@/components/UnifiedModal";

type BrowserCode = { requestID: string; userCode: string; verificationURL: string; expiresIn: number };

// FAQ question that explains each license situation.
const LICENSE_FAQ: Record<string, FaqId> = {
  preview_only: "license_not_entitled",
  release_only: "preview_license",
  trial: "preview_license",
  none: "license_not_entitled",
  error: "license_not_entitled",
  partial_error: "license_not_entitled",
  unreachable: "download_failing",
};
type LicenseResult = { xuid: string; release: LicenseState; preview: LicenseState };
const licenseState = (value: string): LicenseState =>
  ["authorized", "trial", "not_entitled"].includes(value) ? value as LicenseState : "error";

export const UserAvatar = () => {
  const { t, i18n } = useTranslation();
  const { openDiagnostics } = useDiagnostics();
  const startupInteractive = useStartupInteractive();
  const elevated = useProcessElevation();
  const [gamertag, setGamertag] = useState("");
  const [xuid, setXuid] = useState("");
  const [avatar, setAvatar] = useState("");
  const [loading, setLoading] = useState(true);
  const [reloadNonce, setReloadNonce] = useState(0);
  const [refreshing, setRefreshing] = useState(false);
  const [open, setOpen] = useState(false);

  const [signingIn, setSigningIn] = useState(false);
  // An account is selected but Xbox Live could not be reached to confirm it:
  // a network problem, not a sign-out.
  const [unreachable, setUnreachable] = useState(false);
  const [licenses, setLicenses] = useState<LicenseResult | null>(null);
  const [browserCode, setBrowserCode] = useState<BrowserCode | null>(null);
  const signInRequest = React.useRef<ReturnType<typeof userService.SignIn> | null>(null);
  const activeSignInID = React.useRef<string | null>(null);

  useEffect(() => Events.On("microsoft.browser.signin", (event) => {
    const code = event.data as BrowserCode;
    if (activeSignInID.current && code?.requestID === activeSignInID.current &&
        code.userCode && code.verificationURL === "https://www.microsoft.com/link") {
      setBrowserCode(code);
    }
  }), []);

  const browserDialog = (
    <UnifiedModal
      isOpen={browserCode !== null}
      size="compact"
      title="登录微软账户"
      type="info"
      showConfirmButton={false}
      showCancelButton
      cancelText="取消登录"
      onCancel={() => {
        signInRequest.current?.cancel();
        setBrowserCode(null);
      }}
      onOpenChange={(isOpen) => {
        if (!isOpen) {
          signInRequest.current?.cancel();
          setBrowserCode(null);
        }
      }}
    >
      <div className="flex flex-col gap-4">
        <p>打开微软验证页面，输入下方代码，再登录你的微软账户。</p>
        <p className="text-sm text-muted">此登录用于 Xbox 账户和 Microsoft Store 安装授权，请使用拥有游戏的微软账户。</p>
        <div className="select-all rounded-xl bg-surface-secondary p-4 text-center font-mono text-3xl tracking-widest" aria-label="微软登录代码">
          {browserCode?.userCode}
        </div>
        <a
          className="font-medium text-accent underline"
          href={browserCode?.verificationURL}
          target="_blank"
          rel="noopener noreferrer"
          onClick={(event) => {
            // WKWebView does not open target=_blank links in a browser.
            // Keep the existing WebView2/server behaviour on Windows.
            if (!System.IsMac() && !System.IsLinux()) return;
            event.preventDefault();
            if (!browserCode) return;
            void Browser.OpenURL(browserCode.verificationURL).catch(() => {
              toast.danger("无法打开浏览器", {
                description: "请手动打开 https://www.microsoft.com/link 并输入上方代码。",
              });
            });
          }}
        >打开微软验证页面</a>
        <p className="text-sm text-muted">完成后此窗口会自动关闭。代码约 {Math.ceil((browserCode?.expiresIn || 0) / 60)} 分钟内有效。</p>
        <p className="text-xs text-muted">兼容模式的登录凭据仅保留到启动器关闭。游戏安装授权将另行检查。</p>
      </div>
    </UnifiedModal>
  );

  const clearUserState = React.useCallback(() => {
    setGamertag("");
    setXuid("");
    setAvatar("");
    setLicenses(null);
  }, []);

  const signIn = async () => {
    if (activeSignInID.current) return;
    const requestID = crypto.randomUUID();
    activeSignInID.current = requestID;
    setSigningIn(true);
    try {
      const request = userService.SignIn(requestID);
      signInRequest.current = request;
      const code = await request;
      if (code === "ERR_CANCELED") return;
      if (code) {
        toast.danger(t("useravatar.signin_failed"), { description: t(`errors.${code}`) });
        return;
      }
      setOpen(false);
      clearUserState();
      setLoading(true);
      setReloadNonce((v) => v + 1);
    } catch {
      toast.danger(t("useravatar.signin_failed"));
    } finally {
      signInRequest.current = null;
      activeSignInID.current = null;
      setBrowserCode(null);
      setSigningIn(false);
    }
  };

  const refreshSessionIfNeeded = React.useCallback(async (force = false) => {
    try {
      const getState = (userService as any)?.XUserGetState;
      const reset = (userService as any)?.ResetSession;

      if (typeof reset !== "function") {
        return false;
      }

      let shouldReset = force;
      if (!shouldReset && typeof getState === "function") {
        const state = await getState();
        shouldReset = typeof state === "number" && state !== 0;
      }

      if (!shouldReset) {
        return false;
      }

      const result = await reset();
      if (result) {
        console.error("[UserAvatar] ResetSession error", result);
      }
      return true;
    } catch (e) {
      console.error("[UserAvatar] refreshSessionIfNeeded error", e);
      return false;
    }
  }, []);

  useEffect(() => {
    if (!startupInteractive) return;
    let cancelled = false;

    const fetchUser = async () => {
      try {
        if (!userService.GetLocalUserId) {
          return;
        }

        await refreshSessionIfNeeded();

        const status = typeof userService.GetLocalUserStatus === "function"
          ? await userService.GetLocalUserStatus()
          : { xuid: await userService.GetLocalUserId(), unreachable: false };
        if (cancelled) return;
        const id = String(status?.xuid || "");
        setUnreachable(!id && Boolean(status?.unreachable));
        if (!id) {
          clearUserState();
          return;
        }

        const [tag, pic] = await Promise.all([
          userService.GetLocalUserGamertag(),
          userService.GetLocalUserGamerPicture(1).catch((err) => {
            console.error("[UserAvatar] GetLocalUserGamerPicture error", err);
            return "";
          }),
        ]);
        if (cancelled) return;

        setXuid(id);
        setGamertag(String(tag || ""));
        setAvatar(pic ? `data:image/png;base64,${pic}` : "");
      } catch (e) {
        console.error("[UserAvatar] fetchUser error", e);
      } finally {
        if (!cancelled) {
          setLoading(false);
        }
      }
    };

    void fetchUser();
    return () => {
      cancelled = true;
    };
  }, [clearUserState, refreshSessionIfNeeded, reloadNonce, startupInteractive]);

  // Check immediately after identity resolves, independently of opening the popover.
  useEffect(() => {
    if (!startupInteractive || !xuid || signingIn) {
      setLicenses(null);
      return;
    }
    let cancelled = false;
    setLicenses({ xuid, release: "checking", preview: "checking" });
    const request = reloadNonce > 0
      ? userService.RefreshGameLicenses(xuid)
      : userService.CheckGameLicenses(xuid);
    void request.then((result) => {
      if (cancelled) return;
      setLicenses({
        xuid,
        release: result.xuid === xuid ? licenseState(result.release) : "error",
        preview: result.xuid === xuid ? licenseState(result.preview) : "error",
      });
    }).catch(() => {
      if (!cancelled) setLicenses({ xuid, release: "error", preview: "error" });
    });
    return () => {
      cancelled = true;
      request.cancel();
    };
  }, [reloadNonce, signingIn, startupInteractive, xuid]);

  useEffect(() => {
    if (!startupInteractive) return;
    if (!open) return;
    if (!xuid) return;

    let cancelled = false;

    const fetchDetails = async () => {
      try {
        const pic = await userService.GetLocalUserGamerPicture(1);
        if (!cancelled && pic) {
          setAvatar(`data:image/png;base64,${pic}`);
        }
      } catch (err) {
        console.error("[UserAvatar] GetLocalUserGamerPicture error", err);
      }
    };

    void fetchDetails();
    return () => {
      cancelled = true;
    };
  }, [open, reloadNonce, startupInteractive, xuid]);

  // Share the license states with pages that warn before a download the
  // account cannot license.
  useEffect(() => {
    if (!startupInteractive || loading) return;
    if (!xuid) {
      publishLicenses({ known: true, xuid: "", unreachable, release: "not_entitled", preview: "not_entitled" });
      return;
    }
    const current = licenses?.xuid === xuid ? licenses : null;
    publishLicenses({
      known: true,
      xuid,
      release: current?.release ?? "checking",
      preview: current?.preview ?? "checking",
    });
  }, [licenses, loading, startupInteractive, unreachable, xuid]);

  // Troubleshooting steps open this menu ("Check the signed-in account").
  const signInRef = React.useRef(signIn);
  signInRef.current = signIn;
  useEffect(() => {
    const onOpen = () => {
      if (gamertag || unreachable) setOpen(true);
      else void signInRef.current();
    };
    window.addEventListener(OPEN_ACCOUNT_EVENT, onOpen);
    return () => window.removeEventListener(OPEN_ACCOUNT_EVENT, onOpen);
  }, [gamertag, unreachable]);

  const retry = () => {
    setLoading(true);
    setReloadNonce((v) => v + 1);
  };

  const releaseState: LicenseState = licenses?.xuid === xuid ? licenses.release : "checking";
  const previewState: LicenseState = licenses?.xuid === xuid ? licenses.preview : "checking";
  const insight = licenseInsight(releaseState, previewState);

  if (!startupInteractive || loading) {
    return (
      <div className="flex size-10 shrink-0 items-center justify-center">
        <Avatar
          size="sm"
          className={cn(
            "ring-2 ring-border",
            "ring-2 ring-border/60 dark:ring-zinc-700/60 bg-surface-secondary text-muted dark:text-zinc-500",
          )}
        >
          <Avatar.Image alt={""} />
          <Avatar.Fallback>{"?"}</Avatar.Fallback>
        </Avatar>
      </div>
    );
  }

  if (!gamertag && unreachable) {
    const label = t("troubleshoot.license.unreachable.title");
    return (
      <Popover isOpen={open} onOpenChange={setOpen}>
        <Button
          isIconOnly
          size="sm"
          variant="ghost"
          aria-label={label}
          className="relative size-10 shrink-0 rounded-full p-0 wails-no-drag"
        >
          <FaUser size={20} aria-hidden="true" />
          <FaExclamationCircle
            aria-hidden="true"
            className="absolute bottom-1 right-1 size-3.5 rounded-full bg-background text-warning"
          />
        </Button>
        <Popover.Content
          className="p-0 bg-overlay border border-border/70 dark:border-zinc-700/60 shadow-2xl rounded-2xl"
          placement="bottom end"
        >
          <Popover.Arrow />
          <Popover.Dialog aria-label={label} className="p-0">
            <div role="alert" className="w-80 max-w-[calc(100vw-2rem)] p-4 text-sm">
              <p className="flex items-center gap-2 font-semibold text-foreground">
                <FaExclamationCircle className="shrink-0 text-warning" aria-hidden="true" />
                {label}
              </p>
              <p className="mt-2 text-xs leading-5 text-foreground/80">{t("troubleshoot.license.unreachable.body")}</p>
              <div className="mt-3 flex flex-wrap gap-2">
                <Button
                  size="sm"
                  variant="primary"
                  onPress={() => {
                    setOpen(false);
                    openDiagnostics({ topic: "license" });
                  }}
                >
                  <FiActivity aria-hidden="true" />
                  {t("troubleshoot.action.diagnose")}
                </Button>
                <Button size="sm" variant="secondary" onPress={() => { setOpen(false); retry(); }}>
                  <FaSync aria-hidden="true" />
                  {t("download_manager.actions.retry")}
                </Button>
              </div>
              <div className="mt-3 flex items-center justify-between gap-2 border-t border-border pt-3">
                <span className="text-xs text-muted">{t("troubleshoot.license.unreachable.other_account")}</span>
                <Button size="sm" variant="ghost" isPending={signingIn} onPress={() => void signIn()}>
                  {t("useravatar.sign_in")}
                </Button>
              </div>
            </div>
          </Popover.Dialog>
        </Popover.Content>
      </Popover>
    );
  }

  if (!gamertag) {
    return (
      <>
      {browserDialog}
      <div className="flex items-center gap-2">
        <Tooltip>
          <Button
            isIconOnly
            size="sm"
            aria-label={t("useravatar.sign_in")}
            onPress={() => void signIn()}
            isPending={signingIn}
            variant={"ghost"}
            className="size-10 shrink-0 rounded-full p-0 wails-no-drag"
          >
            <FaUser size={20} />
          </Button>
          <Tooltip.Content>{elevated ? t("app.administrator_description") : t("useravatar.sign_in")}</Tooltip.Content>
        </Tooltip>
      </div>
      </>
    );
  }

  return (
    <>
    {browserDialog}
    <Popover
      isOpen={open}
      onOpenChange={async (nextOpen: boolean) => {
        setOpen(nextOpen);
        if (!nextOpen) return;
        try {
          setRefreshing(true);
          const refreshed = await refreshSessionIfNeeded();
          if (refreshed) {
            setReloadNonce((v) => v + 1);
          }
        } catch (e) {
          console.error("[UserAvatar] XUserGetState error", e);
        } finally {
          setRefreshing(false);
        }
      }}
    >
      <Button
        isIconOnly
        variant="ghost"
        aria-label={gamertag}
        className="size-10 shrink-0 rounded-full p-0 wails-no-drag cursor-pointer transition-transform hover:scale-105 active:scale-95"
      >
        <Avatar
          size="sm"
          className={cn("ring-2 ring-border", "ring-2 ring-brand-500/30")}
        >
          <Avatar.Image src={avatar} alt={gamertag} />
          <Avatar.Fallback>{gamertag.slice(0, 2)}</Avatar.Fallback>
        </Avatar>
      </Button>
      <Popover.Content
        className="p-0 bg-overlay border border-border/70 dark:border-zinc-700/60 shadow-2xl rounded-2xl"
        placement="bottom end"
      >
        <Popover.Arrow />
        <Popover.Dialog aria-label={gamertag} className="p-0">
          <div className="w-80 max-w-[calc(100vw-2rem)] max-h-[calc(100dvh-6rem)] overflow-y-auto p-4">
            <div className="flex items-center gap-3">
              <Avatar
                size="lg"
                className="w-12 h-12 shrink-0 bg-transparent ring-2 ring-accent"
              >
                <Avatar.Image src={avatar} alt={gamertag} />
                <Avatar.Fallback>{gamertag.slice(0, 2)}</Avatar.Fallback>
              </Avatar>
              <div className="min-w-0 flex flex-col items-start">
                <div className="w-full break-words text-lg">
                  {
                    <span className="font-bold text-lg text-brand-700 dark:text-brand-300">
                      {gamertag}
                    </span>
                  }
                </div>
                {
                  <div className="flex flex-col gap-1">
                    <span className="break-all text-xs text-muted">
                      {t("useravatar.xuid", {
                        xuid,
                      })}
                    </span>
                  </div>
                }
              </div>
            </div>

            <section className="mt-3 border-t border-border pt-2" aria-labelledby="account-authorization-heading">
              <div className="mb-2 flex items-center justify-between gap-2">
                <h2 id="account-authorization-heading" className="flex items-center gap-2 text-sm font-semibold">
                  <FaShieldAlt className="text-accent" aria-hidden="true" />
                  {t("useravatar.authorization.title")}
                </h2>
                <Tooltip>
                  <Button
                    isIconOnly
                    size="sm"
                    className="shrink-0"
                    aria-label={t("common.refresh")}
                    onPress={async () => {
                      setRefreshing(true);
                      await refreshSessionIfNeeded(true);
                      setReloadNonce((v) => v + 1);
                      setRefreshing(false);
                    }}
                    variant="ghost"
                    isPending={refreshing}
                    isDisabled={signingIn}
                  >
                    {({ isPending }) => isPending
                      ? <Spinner size="sm" color="current" />
                      : <FaSync size={14} aria-hidden="true" />}
                  </Button>
                  <Tooltip.Content>{t("common.refresh")}</Tooltip.Content>
                </Tooltip>
              </div>
              <dl className="divide-y divide-border rounded-xl bg-surface-secondary px-3">
                {(["release", "preview"] as const).map((channel) => {
                  const state = licenses?.xuid === xuid ? licenses[channel] : "checking";
                  const StatusIcon = state === "authorized" ? FaCheckCircle :
                    state === "error" ? FaExclamationCircle : FaQuestionCircle;
                  return (
                  <div key={channel} className="flex flex-wrap items-center justify-between gap-x-3 gap-y-1 py-2.5">
                    <dt className="text-sm font-medium">{channel === "release" ? "Release" : "Preview"}</dt>
                    <dd role="status" className={cn(
                      "flex items-center gap-1.5 text-xs font-medium",
                      state === "authorized" ? "text-success"
                        : state === "trial" || state === "not_entitled" ? "text-warning"
                        : state === "error" ? "text-danger" : "text-muted",
                    )}>
                      {state === "checking" ? <Spinner size="sm" /> : <StatusIcon aria-hidden="true" />}
                      {t(`useravatar.authorization.${state}`)}
                    </dd>
                  </div>
                );})}
              </dl>
              {insight && (
                <div role="note" className="mt-2 rounded-xl border border-border bg-surface-secondary/60 p-3 text-xs leading-5">
                  <p className="font-semibold text-foreground">{t(`troubleshoot.license.${insight}.title`)}</p>
                  <p className="mt-1 text-foreground/80">{t(`troubleshoot.license.${insight}.body`)}</p>
                  <div className="mt-2 flex flex-wrap gap-2">
                    <Button
                      size="sm"
                      variant="secondary"
                      onPress={() => {
                        setOpen(false);
                        openDiagnostics({ topic: "license" });
                      }}
                    >
                      <FiActivity aria-hidden="true" />
                      {t("troubleshoot.action.diagnose")}
                    </Button>
                    <Button
                      size="sm"
                      variant="ghost"
                      onPress={() => void Browser.OpenURL(faqUrl(LICENSE_FAQ[insight] ?? "license_not_entitled", i18n.language))}
                    >
                      {t("troubleshoot.learn_more")}
                      <FiExternalLink aria-hidden="true" />
                    </Button>
                  </div>
                </div>
              )}
            </section>

            {elevated && (
              <p className="mt-3 text-xs text-muted">{t("app.administrator_description")}</p>
            )}
            <div className="mt-3 flex justify-end">
              <Button size="sm" variant="secondary" isPending={signingIn} onPress={() => void signIn()}>
                {t("useravatar.switch_account")}
              </Button>
            </div>
          </div>
        </Popover.Dialog>
      </Popover.Content>
    </Popover>
    </>
  );
};
