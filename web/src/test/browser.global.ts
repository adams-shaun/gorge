import { chromium, type BrowserServer } from 'playwright';
import { createServer, type ViteDevServer } from 'vite';
import type { ProvidedContext } from 'vitest';

type TestProject = {
  provide<T extends keyof ProvidedContext & string>(key: T, value: ProvidedContext[T]): void;
};

/**
 * Start the browser test infrastructure once for the whole Vitest invocation.
 * Warming the production entry waits for Vite's automatic dependency
 * optimizer before any fixture page can race its first dependency transform.
 */
export default async function setup(project: TestProject): Promise<() => Promise<void>> {
  const server: ViteDevServer = await createServer({
    root: process.cwd(),
    configLoader: 'runner',
    server: { host: '127.0.0.1', port: 0 },
  });
  await server.listen();
  await server.warmupRequest('/src/main.ts');
  await server.environments.client.waitForRequestsIdle();

  const browserServer: BrowserServer = await chromium.launchServer({ headless: true });
  const browserURL = server.resolvedUrls?.local[0];
  if (!browserURL) {
    await browserServer.close();
    await server.close();
    throw new Error('shared browser Vite server has no local URL');
  }

  project.provide('browserURL', browserURL);
  project.provide('browserWSEndpoint', browserServer.wsEndpoint());

  return async () => {
    await browserServer.close();
    await server.close();
  };
}
