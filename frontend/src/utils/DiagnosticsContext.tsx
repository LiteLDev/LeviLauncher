import React from "react";
import { toast } from "@heroui/react";
import { Browser, System } from "@wailsio/runtime";
import { useTranslation } from "react-i18next";
import { useNavigate } from "react-router-dom";
import * as minecraft from "bindings/github.com/liteldev/LeviLauncher/internal/app/minecraft";
import * as diagnosticsService from "bindings/github.com/liteldev/LeviLauncher/internal/app/diagnosticsservice";
import { ROUTES } from "@/constants/routes";
import type { DiagnosticTopic, FixActionId } from "@/utils/troubleshoot";
import { GetLogsDir } from "bindings/github.com/liteldev/LeviLauncher/internal/app/minecraft";
import { openDirectory, showDirectoryOpenError } from "@/utils/explorer";

const DiagnosticsModal = React.lazy(() =>
  import("@/components/DiagnosticsModal").then((m) => ({ default: m.DiagnosticsModal })),
);

export type DiagnosticsRequest = {
  topic: DiagnosticTopic;
  // Instance folder name the failure was about.
  instance?: string;
  // "release" or "preview" for license and install failures.
  channel?: string;
};

type DiagnosticsContextValue = {
  openDiagnostics: (request: DiagnosticsRequest) => void;
};

const DiagnosticsContext = React.createContext<DiagnosticsContextValue | null>(null);

// OPEN_ACCOUNT_EVENT asks the account menu in the top bar to open, where the
// account's Release/Preview licenses are shown and can be switched.
export const OPEN_ACCOUNT_EVENT = "levi:open-account";

export const STORE_URLS = {
  gamingServices: "ms-windows-store://pdp/?ProductId=9MWPM2CQNLHN",
  minecraft: "ms-windows-store://pdp/?ProductId=9NBLGGH2JHXJ",
  preview: "ms-windows-store://pdp/?ProductId=9P5X4QVLC2XR",
  buy: "https://www.xbox.com/games/store/minecraft/9nblggh537bl",
  devices: "https://account.microsoft.com/devices/content",
  orders: "https://account.microsoft.com/billing/orders",
  releases: "https://github.com/LiteLDev/LeviLauncher/releases",
} as const;

const SETTINGS_URIS = {
  developer: "ms-settings:developers",
  proxy: "ms-settings:network-proxy",
  date: "ms-settings:dateandtime",
  region: "ms-settings:regionformatting",
  apps: "ms-settings:appsfeatures",
  storage: "ms-settings:storagesense",
} as const;

export const DiagnosticsProvider: React.FC<{ children: React.ReactNode }> = ({ children }) => {
  const [request, setRequest] = React.useState<DiagnosticsRequest | null>(null);
  const [nonce, setNonce] = React.useState(0);
  const openDiagnostics = React.useCallback((next: DiagnosticsRequest) => {
    setRequest(next);
    setNonce((n) => n + 1);
  }, []);
  const value = React.useMemo(() => ({ openDiagnostics }), [openDiagnostics]);
  return (
    <DiagnosticsContext.Provider value={value}>
      {children}
      {request && (
        <React.Suspense fallback={null}>
          <DiagnosticsModal
            key={nonce}
            request={request}
            onClose={() => setRequest(null)}
          />
        </React.Suspense>
      )}
    </DiagnosticsContext.Provider>
  );
};

export const useDiagnostics = (): DiagnosticsContextValue => {
  const ctx = React.useContext(DiagnosticsContext);
  // Outside the provider (isolated previews), diagnostics are unavailable
  // and the buttons that open them do nothing.
  return ctx ?? { openDiagnostics: () => {} };
};

export const useHasDiagnostics = (): boolean => React.useContext(DiagnosticsContext) !== null;

const openExternal = async (url: string, onError: () => void) => {
  try {
    await Browser.OpenURL(url);
  } catch {
    onError();
  }
};

const openLogs = async () => {
  try {
    const dir = String((await GetLogsDir()) || "").trim();
    if (dir) await openDirectory(dir);
  } catch (error) {
    showDirectoryOpenError(error);
  }
};

// Actions that move away from the current screen; a dialog offering one
// closes before it runs.
export const LEAVING_ACTIONS: ReadonlySet<FixActionId> = new Set<FixActionId>([
  "open_mirror_settings",
  "open_data_settings",
  "open_wine_settings",
  "open_process_settings",
  "open_update_settings",
  "open_download_page",
  "open_account",
]);

const WINDOWS_ACTIONS: ReadonlySet<FixActionId> = new Set([
  "install_vcruntime",
  "open_gaming_services",
  "open_minecraft_store",
  "open_preview_store",
  "enable_developer_mode",
  "open_developer_settings",
  "open_proxy_settings",
  "open_date_settings",
  "open_region_settings",
  "open_apps_settings",
  "open_storage_settings",
]);

export const isFixActionAvailable = (action: FixActionId): boolean =>
  action === "open_wine_settings" ? System.IsMac() : !System.IsMac() || !WINDOWS_ACTIONS.has(action);

// useFixAction returns the handler for a troubleshooting step's button.
// "diagnose" opens diagnostics for the given request.
export const useFixAction = (diagnose?: DiagnosticsRequest) => {
  const { t } = useTranslation();
  const navigate = useNavigate();
  const { openDiagnostics } = useDiagnostics();

  return React.useCallback(
    async (action: FixActionId) => {
      const failedToOpen = () =>
        toast.danger(t("troubleshoot.action_failed"), {
          description: t("troubleshoot.action_failed_body"),
        });
      const settingsTab = (tab: string) =>
        navigate(ROUTES.settings, { state: { tab } });
      switch (action) {
        case "diagnose":
          openDiagnostics(diagnose ?? { topic: "full" });
          return;
        case "install_vcruntime":
          minecraft.EnsureVcRuntimeInteractive();
          toast(t("troubleshoot.installer_started"), {
            description: t("troubleshoot.installer_started_body"),
            timeout: 6000,
          });
          return;
        case "install_gameinput":
          minecraft.EnsureGameInputInteractive();
          toast(t("troubleshoot.installer_started"), {
            description: t("troubleshoot.installer_started_body"),
            timeout: 6000,
          });
          return;
        case "enable_developer_mode": {
          let ok = false;
          try {
            ok = Boolean(await diagnosticsService.EnableDeveloperMode());
          } catch {
            ok = false;
          }
          if (ok) {
            toast.success(t("troubleshoot.developer_mode_enabled"));
          } else {
            toast.danger(t("troubleshoot.developer_mode_failed"), {
              description: t("troubleshoot.developer_mode_failed_body"),
            });
          }
          return;
        }
        case "open_gaming_services":
          return openExternal(STORE_URLS.gamingServices, failedToOpen);
        case "open_minecraft_store":
          return openExternal(STORE_URLS.minecraft, failedToOpen);
        case "open_preview_store":
          return openExternal(STORE_URLS.preview, failedToOpen);
        case "open_buy":
          return openExternal(STORE_URLS.buy, failedToOpen);
        case "open_devices":
          return openExternal(STORE_URLS.devices, failedToOpen);
        case "open_orders":
          return openExternal(STORE_URLS.orders, failedToOpen);
        case "open_releases":
          return openExternal(STORE_URLS.releases, failedToOpen);
        case "open_developer_settings":
          return openExternal(SETTINGS_URIS.developer, failedToOpen);
        case "open_proxy_settings":
          return openExternal(SETTINGS_URIS.proxy, failedToOpen);
        case "open_date_settings":
          return openExternal(SETTINGS_URIS.date, failedToOpen);
        case "open_region_settings":
          return openExternal(SETTINGS_URIS.region, failedToOpen);
        case "open_apps_settings":
          return openExternal(SETTINGS_URIS.apps, failedToOpen);
        case "open_storage_settings":
          return openExternal(SETTINGS_URIS.storage, failedToOpen);
        case "open_mirror_settings":
          settingsTab("components");
          return;
        case "open_wine_settings":
          settingsTab("wine");
          return;
        case "open_data_settings":
          settingsTab("general");
          return;
        case "open_process_settings":
          settingsTab("others");
          return;
        case "open_update_settings":
          settingsTab("updates");
          return;
        case "open_download_page":
          navigate(ROUTES.download);
          return;
        case "open_account":
          window.dispatchEvent(new CustomEvent(OPEN_ACCOUNT_EVENT));
          return;
        case "open_logs":
          return openLogs();
      }
    },
    [diagnose, navigate, openDiagnostics, t],
  );
};
