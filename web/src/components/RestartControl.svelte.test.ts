import { type Browser } from 'playwright';
import { browserURL, sharedBrowser } from '../test/browser';
import { beforeAll, describe, expect, it } from 'vitest';
import { render } from 'svelte/server';
import RestartControl from './RestartControl.svelte';

let browser: Browser;
let url = '';

beforeAll(async () => {
  url = browserURL;
  browser = await sharedBrowser();
});

describe('RestartControl (fb-20260922T202722Z)', () => {
  it('unarmed renders the Restart button (data-restart-control) and no confirm button', () => {
    const { html } = render(RestartControl, { props: { onArm: () => {}, onConfirm: () => {} } });
    expect(html).toContain('data-restart-control');
    expect(html).toContain('Restart<');
    expect(html).not.toContain('data-confirm-restart');
  });

  it('armed renders the wider confirm button (data-confirm-restart) and no arm button', () => {
    const { html } = render(RestartControl, { props: { confirming: true, onArm: () => {}, onConfirm: () => {} } });
    expect(html).toContain('data-confirm-restart');
    expect(html).toContain('Restart — confirm');
    expect(html).not.toContain('data-restart-control');
  });

  it('clicking arm then confirm fires each callback once, through the REAL component in a browser', async () => {
    const page = await browser.newPage();
    await page.goto(`${url}src/components/RestartControl.geometry.html`);
    await page.waitForSelector('[data-restart-fixture] [data-restart-control]', { timeout: 60_000 });

    // Precondition: the probe the fixture wires exists, so later reads cannot
    // silently read undefined off a fixture that failed to mount.
    const probeReady = await page.evaluate(() => typeof (window as unknown as { __restartProbe?: unknown }).__restartProbe !== 'undefined');
    expect(probeReady, 'the fixture did not mount its probe').toBe(true);

    await page.click('[data-restart-fixture] [data-restart-control]');
    await page.waitForSelector('[data-restart-fixture] [data-confirm-restart]', { timeout: 10_000 });
    const armed = await page.evaluate(() => (window as unknown as { __restartProbe: { confirming: boolean } }).__restartProbe.confirming);
    expect(armed, 'the arm click did not set confirming').toBe(true);

    await page.click('[data-restart-fixture] [data-confirm-restart]');
    await page.waitForTimeout(50);
    const after = await page.evaluate(() => {
      const p = (window as unknown as { __restartProbe: { confirming: boolean; busy: boolean; confirms: number } }).__restartProbe;
      return { confirms: p.confirms, busy: p.busy };
    });
    await page.close();
    expect(after.confirms, 'the confirm click did not fire onConfirm exactly once').toBe(1);
    expect(after.busy, 'the confirm click did not set busy').toBe(true);
  });
});
