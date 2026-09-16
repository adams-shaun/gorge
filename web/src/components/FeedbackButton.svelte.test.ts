import { type Browser, type Page } from 'playwright';
import { browserURL, sharedBrowser } from '../test/browser';
import { beforeAll, describe, expect, it } from 'vitest';

describe('FeedbackButton breadcrumbs', () => {
  let browser: Browser;
  const url = browserURL;

  beforeAll(async () => {
    browser = await sharedBrowser();
  });

  it('submits route/seat context and production intent breadcrumbs', async () => {
    const page: Page = await browser.newPage();
    await page.goto(`${url}src/components/FeedbackButton.fixture.html`);
    await page.getByRole('button', { name: 'Feedback' }).click();
    await page.locator('textarea').fill('the target picker skipped');
    await page.getByRole('button', { name: 'Send' }).click();
    const form = await page.evaluate(() => (window as unknown as { feedbackForm: Record<string, string> | null }).feedbackForm);
    await page.close();

    expect(form).toMatchObject({ text: 'the target picker skipped', table: 'table-fixture', seat: '2' });
    const client = JSON.parse(form!.client) as {
      play: { settings: { preset: string } };
      actions: { type: string; detail?: { decision_kind?: string; choices?: { index: number }[] } }[];
    };
    expect(client.play.settings.preset).toBe('casual');
    expect(client.actions).toEqual(expect.arrayContaining([
      expect.objectContaining({
        type: 'intent_sent',
        detail: expect.objectContaining({ decision_kind: 'target', choices: [{ index: 3, kind: 'permanent' }] }),
      }),
    ]));
  });

  it('asks the browser to include and prefer the current tab when capturing a screenshot', async () => {
    const page: Page = await browser.newPage();
    // Installed before the fixture loads: replaces getDisplayMedia with a
    // recorder that captures the exact constraints object and rejects, so the
    // request is proven without any real screen grant or browser chrome. The
    // component treats the rejection as a cancelled capture.
    await page.addInitScript(() => {
      const w = window as unknown as { __displayMediaArgs?: unknown };
      Object.defineProperty(navigator, 'mediaDevices', {
        configurable: true,
        get: () => ({
          getDisplayMedia: (constraints: unknown) => {
            w.__displayMediaArgs = constraints;
            return Promise.reject(new DOMException('Permission denied', 'NotAllowedError'));
          },
        }),
      });
    });
    await page.goto(`${url}src/components/FeedbackButton.fixture.html`);
    await page.getByRole('button', { name: 'Feedback' }).click();
    await page.getByRole('button', { name: 'Attach a screenshot' }).click();
    const args = await page.evaluate(
      () => (window as unknown as { __displayMediaArgs?: unknown }).__displayMediaArgs,
    );
    await page.close();

    expect(args).toEqual({ video: true, preferCurrentTab: true, selfBrowserSurface: 'include' });
  });
});
