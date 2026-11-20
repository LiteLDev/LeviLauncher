// GitHub proxies offered in the GitHub proxy pickers; any other URL can
// still be typed in by hand. lip appends the repository path to the proxy, so
// proxies that take a whole GitHub URL (gh-proxy.org, ghproxy.vip) end in
// "/https://github.com".
export const GITHUB_PROXY_PRESETS = [
  "https://gh.bibk.top/https://github.com",
  "https://gh-proxy.org/https://github.com",
  "https://v4.gh-proxy.org/https://github.com",
  "https://v6.gh-proxy.org/https://github.com",
  "https://cdn.gh-proxy.org/https://github.com",
  "https://axisnow.gh-proxy.org/https://github.com",
  "https://ghproxy.vip/https://github.com",
];

// Picker label of a proxy preset: its host, without the GitHub URL suffix.
export const githubProxyLabel = (url: string) =>
  url.replace(/^https?:\/\//, "").replace(/\/https:\/\/github\.com$/, "");
