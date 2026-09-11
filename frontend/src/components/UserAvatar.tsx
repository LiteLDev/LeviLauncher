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

type LicenseState = "checking" | "authorized" | "trial" | "not_entitled" | "error";
type LicenseResult = { xuid: string; release: LicenseState; preview: LicenseState };
const licenseState = (value: string): LicenseState =>
  ["authorized", "trial", "not_entitled"].includes(value) ? value as LicenseState : "error";

export const UserAvatar = () => {
  const { t } = useTranslation();
  const startupInteractive = useStartupInteractive();
  const [gamertag, setGamertag] = useState("");
  const [xuid, setXuid] = useState("");
  const [avatar, setAvatar] = useState("");
  const [loading, setLoading] = useState(true);
  const [reloadNonce, setReloadNonce] = useState(0);
  const [refreshing, setRefreshing] = useState(false);
  const [open, setOpen] = useState(false);

  const [signingIn, setSigningIn] = useState(false);
  const [licenses, setLicenses] = useState<LicenseResult | null>(null);

  const clearUserState = React.useCallback(() => {
    setGamertag("");
    setXuid("");
    setAvatar("");
    setLicenses(null);
  }, []);

  const signIn = async () => {
    if (signingIn) return;
    setSigningIn(true);
    try {
      const code = await userService.SignIn();
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

        const id = await userService.GetLocalUserId();
        if (cancelled) return;
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
    const request = userService.CheckGameLicenses(xuid);
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

  if (!gamertag) {
    return (
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
          <Tooltip.Content>{t("useravatar.sign_in")}</Tooltip.Content>
        </Tooltip>
      </div>
    );
  }

  return (
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
                    <dd role="status" className={cn("flex items-center gap-1.5 text-xs font-medium", state === "authorized" ? "text-success" : "text-muted")}>
                      {state === "checking" ? <Spinner size="sm" /> : <StatusIcon aria-hidden="true" />}
                      {t(`useravatar.authorization.${state}`)}
                    </dd>
                  </div>
                );})}
              </dl>
              <p className="mt-2 text-xs leading-relaxed text-muted">
                {t("useravatar.authorization.description")}
              </p>
            </section>

            <div className="mt-3 flex justify-end">
              <Button size="sm" variant="secondary" isPending={signingIn} onPress={() => void signIn()}>
                {t("useravatar.switch_account")}
              </Button>
            </div>
          </div>
        </Popover.Dialog>
      </Popover.Content>
    </Popover>
  );
};
