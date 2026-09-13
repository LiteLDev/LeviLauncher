import assert from "node:assert/strict";
import { createServer } from "node:http";
import { test } from "node:test";
import { waitForDevServer } from "./wait-for-dev-server.mjs";

const viteHTML = '<script type="module" src="/@vite/client"></script>';

async function serve(t, handler) {
  const server = createServer(handler);
  await new Promise((resolve, reject) => {
    server.once("error", reject);
    server.listen(0, "127.0.0.1", resolve);
  });
  t.after(async () => {
    server.closeAllConnections();
    await new Promise((resolve) => server.close(resolve));
  });
  return `http://localhost:${server.address().port}/`;
}

test("normal launches do not require a frontend dev server", async () => {
  assert.equal(await waitForDevServer(undefined), undefined);
  assert.equal(await waitForDevServer("  "), undefined);
});

test("localhost readiness uses the IPv4 listener used by Wails", async (t) => {
  const address = await serve(t, (_, response) => response.end(viteHTML));
  assert.equal(
    await waitForDevServer(address),
    address.replace("localhost", "127.0.0.1"),
  );
});

test("waits through server startup errors and stale built HTML", async (t) => {
  let requests = 0;
  const address = await serve(t, (_, response) => {
    requests += 1;
    if (requests === 1) response.writeHead(503).end("Starting");
    else if (requests === 2) response.end('<script src="/assets/index.js"></script>');
    else response.end(viteHTML);
  });
  await waitForDevServer(address, { intervalMs: 10 });
  assert.equal(requests, 3);
});

test("reports an actionable error when the endpoint never serves Vite", async (t) => {
  const address = await serve(t, (_, response) => response.end("Not Vite"));
  await assert.rejects(
    waitForDevServer(address, { timeoutMs: 100, intervalMs: 10 }),
    (error) => {
      assert.match(error.message, /did not become ready within 100 ms/);
      assert.match(error.message, /127\.0\.0\.1/);
      assert.match(error.cause.message, /not a Vite development page/);
      return true;
    },
  );
});
