import { setTimeout as delay } from "node:timers/promises";
import { resolve } from "node:path";
import { pathToFileURL } from "node:url";

export async function waitForDevServer(
  address,
  { timeoutMs = 30_000, requestTimeoutMs = 1_000, intervalMs = 250 } = {},
) {
  if (!address?.trim()) return;

  const url = new URL(address);
  // Match the Wails asset proxy's IPv4 transport and the Vite listen address.
  if (url.hostname === "localhost") url.hostname = "127.0.0.1";

  const deadline = Date.now() + timeoutMs;
  let lastError;
  while (Date.now() < deadline) {
    try {
      const response = await fetch(url, {
        signal: AbortSignal.timeout(
          Math.max(1, Math.min(requestTimeoutMs, deadline - Date.now())),
        ),
      });
      const html = await response.text();
      if (response.ok && html.includes("/@vite/client")) return url.href;
      lastError = new Error(
        `HTTP ${response.status}: response is not a Vite development page`,
      );
    } catch (error) {
      lastError = error;
    }
    const remaining = deadline - Date.now();
    if (remaining > 0) await delay(Math.min(intervalMs, remaining));
  }

  throw new Error(
    `Frontend dev server did not become ready within ${timeoutMs} ms: ${url.href}`,
    { cause: lastError },
  );
}

if (
  process.argv[1] &&
  import.meta.url === pathToFileURL(resolve(process.argv[1])).href
) {
  try {
    const url = await waitForDevServer(process.env.FRONTEND_DEVSERVER_URL);
    if (url) console.log(`Frontend dev server ready: ${url}`);
  } catch (error) {
    console.error(error.message);
    if (error.cause) console.error(error.cause.message);
    process.exitCode = 1;
  }
}
