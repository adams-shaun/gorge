import { chromium, type Browser, type Page } from 'playwright';
import { createServer, type ViteDevServer } from 'vite';
import { afterAll, beforeAll, describe, expect, it } from 'vitest';

describe('FeedbackButton breadcrumbs', () => {
  let server: ViteDevServer;
  let browser: Browser;
  let url = '';

  beforeAll(async () => {
    server = await createServer({ root: process.cwd(), configLoader: 'runner', server: { port: 0 } });
    await server.listen();
    url = server.resolvedUrls!.local[0];
    browser = await chromium.launch();
  });

  afterAll(async () => {
    await browser?.close();
    await server?.close();
  });

  it('submits route context and a client.json with settings and an action', async () => {
    const page: Page = await browser.newPage();
    await page.goto(`${url}src/components/FeedbackButton.fixture.html`);
    await page.getByRole('button', { name: 'Feedback' }).click();
    await page.locator('textarea').fill('the target picker skipped');
    await page.getByRole('button', { name: 'Send' }).click();
    const form = await page.evaluate(() => (window as unknown as { feedbackForm: Record<string, string> | null }).feedbackForm);
    await page.close();

    expect(form).toMatchObject({ text: 'the target picker skipped', table: 'table-fixture', seat: '2' });
    const client = JSON.parse(form!.client) as {
      play: { settings: { preset: string }; active_yields: string[] };
      actions: { type: string }[];
    };
    expect(client.play.settings.preset).toBe('casual');
    expect(client.play.active_yields).toEqual(['9:trigger:yield']);
    expect(client.actions).toEqual(expect.arrayContaining([expect.objectContaining({ type: 'intent_sent' })]));
  });
});
