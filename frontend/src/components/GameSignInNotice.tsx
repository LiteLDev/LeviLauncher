import { Button, toast } from "@heroui/react";
import { Browser, Clipboard, Events, System } from "@wailsio/runtime";
import React, { useEffect, useState } from "react";
import { useTranslation } from "react-i18next";
import { FaXbox } from "react-icons/fa";
import { FiArrowUpRight, FiCheck, FiCopy, FiMinus, FiX } from "react-icons/fi";
import * as userService from "bindings/github.com/liteldev/LeviLauncher/internal/app/userservice";

type SignInState = {
  revision: number;
  requestID: string;
  instance: string;
  phase: string;
  userCode: string;
  verificationURL: string;
  expiresAt: number;
  errorCode: string;
  browserError: boolean;
};

export function GameSignInNotice() {
  const { t } = useTranslation();
  const [state, setState] = useState<SignInState | null>(null);
  const [collapsed, setCollapsed] = useState(false);
  const [hidden, setHidden] = useState(false);
  const [now, setNow] = useState(Date.now());
  const [copied, setCopied] = useState(false);
  const nativeHost = System.IsMac() || System.IsLinux();

  useEffect(() => {
    if (!nativeHost) return;
    let active = true;
    const accept = (next: SignInState) => {
      if (!active || !next || !Number.isFinite(next.revision)) return;
      setState((previous) => !previous || next.revision >= previous.revision ? next : previous);
    };
    const unsubscribe = Events.On("minecraft.account.signin", (event) => accept(event.data as SignInState));
    void userService.GetGameSignInState().then((value) => accept(value as SignInState)).catch(() => {});
    return () => { active = false; unsubscribe(); };
  }, [nativeHost]);

  useEffect(() => {
    setCollapsed(false);
    setHidden(false);
    setCopied(false);
  }, [state?.requestID]);

  useEffect(() => {
    if (state?.phase !== "pending") return;
    const timer = window.setInterval(() => setNow(Date.now()), 1000);
    return () => window.clearInterval(timer);
  }, [state?.phase]);

  useEffect(() => {
    if (state?.phase !== "success") return;
    const timer = window.setTimeout(() => setHidden(true), 5000);
    return () => window.clearTimeout(timer);
  }, [state?.phase]);

  if (!nativeHost || !state || state.phase === "idle" || !state.requestID || hidden) return null;
  const remaining = Math.max(0, state.expiresAt - Math.floor(now / 1000));
  const pending = state.phase === "pending" && remaining > 0;
  const success = state.phase === "success";
  const title = success ? t("gameaccount.success") : pending ? t("gameaccount.title") : t(`gameaccount.${state.phase === "pending" ? "expired" : state.phase}`);
  const openVerification = async () => {
    if (!pending || !/^[A-Z0-9]{8}$/.test(state.userCode) || state.verificationURL !== "https://www.microsoft.com/link") return;
    try {
      await Browser.OpenURL(`${state.verificationURL}?otc=${encodeURIComponent(state.userCode)}`);
    } catch { toast.danger(t("gameaccount.browser_error")); }
  };
  const copy = async () => {
    try {
      await Clipboard.SetText(state.userCode);
      setCopied(true);
      window.setTimeout(() => setCopied(false), 2000);
    } catch { toast.danger(t("gameaccount.copy_error")); }
  };

  if (collapsed && pending) return (
    <Button onPress={() => setCollapsed(false)} variant="secondary" className="wails-no-drag fixed bottom-6 right-6 z-[110] gap-2 rounded-full shadow-md">
      <FaXbox aria-hidden="true" />{t("gameaccount.title")}
    </Button>
  );

  return (
    <section role="region" aria-label={t("gameaccount.title")} className="wails-no-drag fixed bottom-6 right-6 z-[110] w-[360px] max-w-[calc(100vw-3rem)] rounded-2xl bg-overlay p-5 text-foreground shadow-lg">
      <div className="flex items-start gap-3">
        <span className="mt-0.5 text-lg text-accent">{success ? <FiCheck aria-hidden="true" /> : <FaXbox aria-hidden="true" />}</span>
        <div className="min-w-0 flex-1">
          <h2 className="text-base font-semibold leading-6" aria-live="polite">{title}</h2>
          <p className="mt-0.5 truncate text-xs text-muted">{state.instance}</p>
        </div>
        <Button isIconOnly size="sm" variant="ghost" aria-label={pending ? t("gameaccount.collapse") : t("common.close")} onPress={() => pending ? setCollapsed(true) : setHidden(true)}>
          {pending ? <FiMinus /> : <FiX />}
        </Button>
      </div>
      {pending ? (
        <>
          <p className="mt-4 text-sm leading-6 text-muted">{t(state.browserError ? "gameaccount.browser_error" : "gameaccount.waiting")}</p>
          <div className="my-4 select-all text-center font-mono text-3xl font-medium tracking-[0.2em]" aria-label={t("gameaccount.code")}>{state.userCode}</div>
          <div className="flex gap-2">
            <Button className="flex-1 gap-2" variant="primary" style={{ color: "var(--accent-foreground)" }} onPress={openVerification}>{t("gameaccount.open")}<FiArrowUpRight aria-hidden="true" /></Button>
            <Button className="gap-2" variant="secondary" onPress={copy}>{copied ? <FiCheck /> : <FiCopy />}{t(copied ? "gameaccount.copied" : "gameaccount.copy")}</Button>
          </div>
          <div className="mt-3 flex items-center justify-between gap-2 text-xs text-muted">
            <span>{t("gameaccount.keep_game_open")}</span>
            <span className="tabular-nums" aria-label={t("gameaccount.expires")}>{Math.floor(remaining / 60)}:{String(remaining % 60).padStart(2, "0")}</span>
          </div>
        </>
      ) : (
        <p className="mt-4 text-sm leading-6 text-muted">{t(success ? "gameaccount.return_to_game" : "gameaccount.retry_in_game")}{state.errorCode ? <span className="ml-1 font-mono text-xs">({state.errorCode})</span> : null}</p>
      )}
    </section>
  );
}
