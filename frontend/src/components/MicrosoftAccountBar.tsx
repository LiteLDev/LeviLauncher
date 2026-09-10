import { useEffect, useState } from "react";
import { Button, Spinner } from "@heroui/react";
import { Events } from "@wailsio/runtime";
import { useTranslation } from "react-i18next";
import { FaMicrosoft } from "react-icons/fa";
import * as account from "bindings/github.com/liteldev/LeviLauncher/internal/app/microsoftaccountservice";
import type { MicrosoftAccountStatus } from "bindings/github.com/liteldev/LeviLauncher/internal/app/models";
import { UnifiedModal } from "@/components/UnifiedModal";
import { ModalDescription } from "@/components/ModalPrimitives";

const promptKey = "levilauncher.microsoft-account.prompted.v1";
let promptedThisSession = false;

export function MicrosoftAccountBar() {
  const { t } = useTranslation();
  const [status, setStatus] = useState<MicrosoftAccountStatus | null>(null);
  const [promptOpen, setPromptOpen] = useState(false);
  const [connectionError, setConnectionError] = useState(false);
  const [requestPending, setRequestPending] = useState(false);

  const update = (next: MicrosoftAccountStatus) => {
    setStatus((previous) => !previous || next.revision >= previous.revision ? next : previous);
    setConnectionError(false);
  };

  useEffect(() => {
    let mounted = true;
    const off = Events.On("microsoft-account-changed", (event) => {
      if (mounted) update(event.data);
    });
    account.GetStatus().then((next) => {
      if (mounted) update(next);
    }).catch(() => { if (mounted) setConnectionError(true); });
    return () => { mounted = false; off(); };
  }, []);

  useEffect(() => {
    if (!status || status.phase === "restoring" || status.phase === "signing_in") return;
    if (status.accountName) setPromptOpen(false);
    let alreadyPrompted = promptedThisSession;
    try { alreadyPrompted ||= localStorage.getItem(promptKey) === "1"; } catch { /* Session fallback. */ }
    if (alreadyPrompted) return;
    promptedThisSession = true;
    try { localStorage.setItem(promptKey, "1"); } catch { /* Session fallback. */ }
    if (status.phase === "signed_out") setPromptOpen(true);
  }, [status]);

  const act = async (action: () => Promise<MicrosoftAccountStatus>) => {
    if (requestPending) return;
    setRequestPending(true);
    try { update(await action()); } catch { setConnectionError(true); }
    finally { setRequestPending(false); }
  };
  const startLogin = () => { setPromptOpen(false); void act(account.StartLogin); };
  const signingIn = status?.phase === "signing_in";
  const busy = requestPending || !status || status.phase === "restoring" || status.phase === "signing_out";
  const errorCode = connectionError ? "ERR_AUTH_SERVICE" : status?.errorCode;
  const message = errorCode
    ? t(`errors.${errorCode}`, { defaultValue: t("errors.ERR_AUTH_FAILED") })
    : signingIn ? t("microsoft_account.signing_in")
    : status?.accountName || (busy ? t("microsoft_account.restoring") : t("microsoft_account.signed_out"));

  return (
    <>
      <div className="mb-3 flex min-w-0 items-center gap-3 border-b border-border/60 pb-3" data-testid="microsoft-account-bar">
        <FaMicrosoft className="size-4 shrink-0 text-muted" aria-hidden="true" />
        <div className="min-w-0 flex-1" aria-live="polite">
          <div className="text-xs text-muted">{t("microsoft_account.title")}</div>
          <div className="truncate text-sm text-foreground" title={message}>
            {message}
          </div>
          {errorCode && status?.accountName && <div className="truncate text-xs text-muted">{status.accountName}</div>}
        </div>
        {(busy || signingIn) && !connectionError && <Spinner size="sm" aria-label={t("microsoft_account.restoring")} />}
        {signingIn ? (
          <>
            <Button size="sm" variant="secondary" onPress={() => void act(account.StartLogin)}>{t("microsoft_account.show_window")}</Button>
            <Button size="sm" variant="ghost" onPress={() => void account.CancelLogin().catch(() => setConnectionError(true))}>{t("common.cancel")}</Button>
          </>
        ) : (
          <>
            <Button size="sm" variant="secondary" isDisabled={busy && !connectionError} onPress={startLogin}>
              {status?.accountName ? t("microsoft_account.switch_account") : t("microsoft_account.sign_in")}
            </Button>
            {status?.accountName && <Button size="sm" variant="ghost" isDisabled={busy} onPress={() => void act(account.SignOut)}>{t("microsoft_account.sign_out")}</Button>}
          </>
        )}
      </div>
      <UnifiedModal
        isOpen={promptOpen}
        onOpenChange={setPromptOpen}
        title={t("microsoft_account.prompt_title")}
        icon={<FaMicrosoft />}
        showCancelButton
        isDismissable
        onConfirm={startLogin}
        confirmText={t("microsoft_account.sign_in")}
        cancelText={t("microsoft_account.later")}
        onCancel={() => setPromptOpen(false)}
      >
        <ModalDescription>{t("microsoft_account.prompt_description")}</ModalDescription>
      </UnifiedModal>
    </>
  );
}
