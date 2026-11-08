// Turns a backend failure ("ERR_X" or "ERR_X: <detail>") into what the GUI
// shows: the code, the raw technical detail and plain-language causes.
//
// The backend keeps the full text of a failure in the result string after the
// code (lip's message with its "hint:" lines, the Windows error of a failed
// launch, ...), and the same text goes to the launcher log. Nothing here
// shortens it; explainErrorDetail only adds readable explanations next to it.

export type ErrorResult = {
  // The ERR_* code, or "" when the text carries none.
  code: string;
  // Everything after the code, trimmed, line breaks kept.
  detail: string;
};

const CODE_PATTERN =
  /(?:^|[\s"'(])((?:ERR|NH_ERR)_[A-Z0-9_]+)\s*(?::\s*)?([\s\S]*)$/;

export const splitErrorResult = (value: unknown): ErrorResult => {
  const source =
    typeof value === "string"
      ? value
      : value instanceof Error
        ? value.message
        : String(value ?? "");
  const text = source.replace(/^\s*error\s*:\s*/i, "").trim();
  if (!text) return { code: "", detail: "" };
  const match = text.match(CODE_PATTERN);
  if (!match) return { code: "", detail: text };
  return { code: match[1], detail: match[2].trim() };
};

type Rule = { key: string; pattern: RegExp };

// Within a group the first matching rule wins; every group can contribute one
// cause, so a failure can be explained as "access denied" and "timed out"
// together. Keys are errors.explain.<key> in the locale files.
const RULE_GROUPS: Rule[][] = [
  [
    { key: "game_exited_early", pattern: /exited right after starting/i },
    { key: "game_no_window", pattern: /never showed a window/i },
    { key: "loader_activation", pattern: /could not activate the mod loader/i },
    {
      key: "bad_executable",
      pattern:
        /not a valid win32 application|%1 is not|bad image|exec format error|error 193\b/i,
    },
  ],
  [
    {
      key: "access_denied",
      pattern:
        /access is denied|permission denied|0x80070005|\beacces\b|operation not permitted|拒绝访问/i,
    },
    {
      key: "file_in_use",
      pattern:
        /used by another process|sharing violation|being used|file is locked|0x80070020/i,
    },
    {
      key: "disk_full",
      pattern:
        /no space left|not enough space|disk is full|0x80070070|disk full/i,
    },
    {
      key: "path_too_long",
      pattern: /path.{0,40}too long|file ?name.{0,40}too long|0x800700ce/i,
    },
    {
      key: "not_found",
      pattern:
        /cannot find the (?:file|path)|no such file|does not exist|file not found|0x80070002|0x80070003|找不到/i,
    },
    {
      key: "developer_mode",
      pattern: /developer mode|0x80073cff|0x80073d05|0x80073cfb/i,
    },
  ],
  [
    {
      key: "dns",
      pattern:
        /no such host|server misbehaving|name or service not known|could not resolve|\bdns\b/i,
    },
    { key: "tls", pattern: /\btls\b|x509|certificate|\bssl\b|handshake/i },
    { key: "timeout", pattern: /timed out|timeout|deadline exceeded/i },
    {
      key: "connection",
      pattern:
        /connection (?:refused|reset|aborted)|connectex|network is unreachable|network down|no route to host|proxyconnect|unexpected eof|wsarecv|wsasend|forcibly closed/i,
    },
    { key: "http_403", pattern: /\b403\b|forbidden/i },
    { key: "http_404", pattern: /\b404\b/i },
    { key: "http_429", pattern: /\b429\b|too many requests|rate limit/i },
    {
      key: "http_5xx",
      pattern: /\b50[0-4]\b|bad gateway|service unavailable|gateway timeout/i,
    },
  ],
  [
    { key: "lip_no_package", pattern: /no package found/i },
    { key: "lip_already_installed", pattern: /already explicitly installed/i },
    {
      key: "lip_conflict",
      pattern:
        /conflict|cannot satisfy|unsatisfiable|no version|incompatible|not compatible/i,
    },
    {
      key: "corrupt_download",
      pattern:
        /checksum|hash mismatch|sha-?\d+ mismatch|corrupt|not a valid zip|zip: |unexpected end of|invalid (?:archive|header)/i,
    },
  ],
];

// explainErrorDetail returns the errors.explain.* keys of the plain-language
// causes that match detail, most specific group first. Empty when nothing
// matches; the raw detail is then the only information, and is still shown.
export const explainErrorDetail = (detail: string): string[] => {
  const text = String(detail ?? "");
  if (!text) return [];
  const keys: string[] = [];
  for (const group of RULE_GROUPS) {
    const hit = group.find((rule) => rule.pattern.test(text));
    if (hit && !keys.includes(hit.key)) keys.push(hit.key);
  }
  return keys;
};
