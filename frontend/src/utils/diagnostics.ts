// Presentation of diagnostics reports (internal/diagnose): which message a
// check shows, which fix buttons it offers and the plain-text report users
// copy into a bug report.

import type { TFunction } from "i18next";
import type { FixActionId } from "@/utils/troubleshoot";
import { licenseInsight, type LicenseState } from "@/utils/troubleshoot";

export type DiagnosticStatus = "ok" | "info" | "skip" | "warn" | "fail";

export type DiagnosticCheck = {
  id: string;
  status: DiagnosticStatus | string;
  params?: Record<string, string> | null;
  detail?: string;
  durationMs?: number;
};

export type DiagnosticReport = {
  topic: string;
  instance?: string;
  checks: DiagnosticCheck[] | null;
  durationMs: number;
};

const RANK: Record<string, number> = { fail: 0, warn: 1, info: 2, ok: 3, skip: 4 };

// sortChecks puts failures first and keeps the backend's order otherwise.
export const sortChecks = (checks: DiagnosticCheck[]): DiagnosticCheck[] =>
  checks
    .map((check, index) => ({ check, index }))
    .sort((a, b) => (RANK[a.check.status] ?? 5) - (RANK[b.check.status] ?? 5) || a.index - b.index)
    .map(({ check }) => check);

const GiB = 1024 ** 3;

// checkParams formats a check's raw parameters for its message.
export const checkParams = (
  check: DiagnosticCheck,
  t: TFunction,
  language?: string,
): Record<string, string> => {
  const raw = check.params ?? {};
  const out: Record<string, string> = { ...raw };
  const number = (value: number, digits = 1) =>
    new Intl.NumberFormat(language?.replace("_", "-") || undefined, {
      maximumFractionDigits: digits,
    }).format(value);
  if (raw.backend) out.backend = t(`settings.wine.graphics_${raw.backend}`, { defaultValue: raw.backend });
  if (raw.free) out.free = number(Number(raw.free) / GiB);
  if (raw.skew) {
    const seconds = Math.abs(Number(raw.skew));
    out.skew = number(seconds / 60, 0);
    out.direction = t(Number(raw.skew) > 0 ? "diagnostics.clock_ahead" : "diagnostics.clock_behind");
  }
  if (raw.latency) out.latency = number(Number(raw.latency) / 1000, 1);
  for (const channel of ["release", "preview"] as const) {
    if (raw[channel]) out[`${channel}_state`] = t(`useravatar.authorization.${raw[channel]}`, { defaultValue: raw[channel] });
  }
  if (raw.reason) out.reason_text = t(`diagnostics.reason.${raw.reason}`, { defaultValue: raw.reason });
  return out;
};

// Sentences ending in a full-width stop (Chinese, Japanese) need no space.
const joinSentences = (...parts: string[]): string =>
  parts
    .filter(Boolean)
    .reduce((out, part) => (!out ? part : /[。！？]$/.test(out) ? out + part : `${out} ${part}`), "");

const first = (t: TFunction, keys: string[], params: Record<string, string>): string => {
  for (const key of keys) {
    const text = t(key, { ...params, defaultValue: "" });
    if (text) return text;
  }
  return "";
};

// checkMessage is the sentence describing a check's result.
export const checkMessage = (
  check: DiagnosticCheck,
  t: TFunction,
  params: Record<string, string>,
): string => {
  const base = `diagnostics.check.${check.id}`;
  const reason = check.params?.reason;
  if (check.id === "license" && check.status !== "skip") {
    const insight = licenseInsight(
      (check.params?.release || "error") as LicenseState,
      (check.params?.preview || "error") as LicenseState,
    );
    const channel = check.params?.channel;
    const summary = t("diagnostics.check.license.summary", params);
    if (channel === "release" || channel === "preview") {
      const state = check.params?.[channel];
      const own = first(t, [`diagnostics.check.license.channel_${state}`], {
        ...params,
        channel: t(`diagnostics.channel.${channel}`),
      });
      return joinSentences(summary, own);
    }
    const why = insight ? t(`troubleshoot.license.${insight}.body`) : t("diagnostics.check.license.ok");
    return joinSentences(summary, why);
  }
  if (check.id.startsWith("ep_")) {
    const active = check.params?.active;
    const keys: string[] = [];
    if (active === "false") keys.push(`${base}.inactive_${check.status === "ok" ? "ok" : "fail"}`);
    if (reason) keys.push(`${base}.${reason}`, `diagnostics.endpoint.${reason}`);
    keys.push(`${base}.${check.status}`, `diagnostics.endpoint.${check.status}`);
    return first(t, keys, params);
  }
  // A check's message is picked by its status and, when it has one, the
  // parameter that tells its situations apart (reason, registration state,
  // proxy mode, mirror on/off): "<status>_<value>", then "<value>", then
  // "<status>".
  const raw = check.params ?? {};
  const discriminators = [
    reason,
    raw.state,
    raw.mode,
    raw.enabled === undefined ? undefined : raw.enabled === "true" ? "on" : "off",
  ].filter(Boolean) as string[];
  const keys: string[] = [];
  // A reason can read differently per proxy mode ("unreachable_env").
  if (reason && raw.mode) keys.push(`${base}.${reason}_${raw.mode}`);
  for (const d of discriminators) keys.push(`${base}.${check.status}_${d}`, `${base}.${d}`);
  keys.push(`${base}.${check.status}`);
  if (reason) keys.push(`diagnostics.reason.${reason}`);
  return first(t, keys, params) || t(`diagnostics.status.${check.status}`);
};

// checkActions are the buttons offered next to a check that found a problem
// or a fact worth acting on.
export const checkActions = (check: DiagnosticCheck): FixActionId[] => {
  const status = check.status;
  const reason = check.params?.reason;
  if (status === "ok" || status === "skip") return [];
  const problem = status === "fail" || status === "warn";
  switch (check.id) {
    case "wine_rosetta":
    case "wine_runtime":
      return problem ? ["open_wine_settings"] : [];
    case "vcruntime":
      return ["install_vcruntime"];
    case "gameinput":
      return ["install_gameinput"];
    case "gaming_services":
      return reason === "disabled" ? [] : ["open_gaming_services", "open_minecraft_store"];
    case "developer_mode":
      return ["enable_developer_mode", "open_developer_settings"];
    case "data_root":
      return ["open_data_settings"];
    case "disk_space":
      return ["open_storage_settings"];
    case "account":
      return reason === "unreachable" ? [] : ["open_account"];
    case "license": {
      if (!problem) return [];
      const release = check.params?.release;
      const preview = check.params?.preview;
      const channel = check.params?.channel;
      const actions: FixActionId[] = ["open_account"];
      if (release === "error" || preview === "error") return [...actions, "open_proxy_settings", "open_date_settings"];
      if ((channel === "preview" || !channel) && preview !== "authorized" && release === "authorized") {
        actions.push("open_preview_store", "open_region_settings");
      } else {
        actions.push("open_buy");
      }
      actions.push("open_devices");
      return actions;
    }
    case "proxy":
      // Windows' proxy page cannot change a proxy set by environment variables.
      return problem && check.params?.mode !== "env" ? ["open_proxy_settings"] : [];
    case "mirror":
      return problem ? ["open_mirror_settings"] : [];
    case "clock":
      return ["open_date_settings"];
    case "ep_version_db":
    case "ep_github":
    case "ep_ll_db":
    case "ep_github_proxy":
      return problem ? ["open_mirror_settings", "open_proxy_settings"] : [];
    case "ep_microsoft_auth":
    case "ep_xbox_store":
    case "ep_xbox_packages":
    case "ep_levimc":
      return problem ? ["open_proxy_settings"] : [];
    case "instance_running":
      return ["open_process_settings"];
    case "registration":
      return problem ? ["open_apps_settings"] : [];
    case "elevation":
      return [];
  }
  return [];
};

// A proxy that does not answer makes every request through it fail; those
// failures say nothing new, so they are listed under the proxy instead of as
// problems of their own.
const isProxyConsequence = (check: DiagnosticCheck): boolean => {
  if (check.status !== "fail" && check.status !== "warn") return false;
  const reason = check.params?.reason;
  return (
    reason === "proxy" ||
    (check.id === "account" && reason === "unreachable") ||
    (check.id === "mirror" && reason === "github_unreachable")
  );
};

export type FoldedChecks = {
  checks: DiagnosticCheck[];
  // Checks that failed only because of another one, by that check's id.
  causedBy: Record<string, DiagnosticCheck[]>;
};

// foldConsequences keeps the root cause and moves what it broke under it.
export const foldConsequences = (checks: DiagnosticCheck[]): FoldedChecks => {
  const proxyDown = checks.some((c) => c.id === "proxy" && c.status === "fail" && c.params?.reason === "unreachable");
  if (!proxyDown) return { checks, causedBy: {} };
  const consequences = checks.filter((c) => c.id !== "proxy" && isProxyConsequence(c));
  if (consequences.length === 0) return { checks, causedBy: {} };
  return {
    checks: checks.filter((c) => !consequences.includes(c)),
    causedBy: { proxy: consequences },
  };
};

// reportText is the copyable report: one line per check with its raw
// parameters, so it reads the same whatever the UI language.
export const reportText = (report: DiagnosticReport, t: TFunction): string => {
  const lines = [
    `LeviLauncher diagnostics: ${report.topic}${report.instance ? ` (${report.instance})` : ""}, ${report.durationMs} ms`,
  ];
  for (const check of sortChecks(report.checks ?? [])) {
    const params = Object.entries(check.params ?? {})
      .filter(([key]) => key !== "path" && key !== "pac")
      .map(([key, value]) => `${key}=${value}`)
      .join(" ");
    lines.push(
      `[${check.status}] ${check.id}${params ? ` ${params}` : ""}${check.detail ? ` | ${check.detail}` : ""}`,
    );
    // Paths name the Windows user; the report is meant to be shared.
    const shared = { ...check, params: { ...(check.params ?? {}) } };
    for (const key of ["path", "pac"]) if (shared.params[key]) shared.params[key] = "<…>";
    const message = checkMessage(shared, t, checkParams(shared, t));
    if (message && (check.status === "fail" || check.status === "warn")) lines.push(`    ${message}`);
  }
  return lines.join("\n");
};
