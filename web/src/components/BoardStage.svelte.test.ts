import { chromium } from 'playwright';
import { createServer } from 'vite';
import { afterAll, beforeAll, describe, expect, it } from 'vitest';

let server: Awaited<ReturnType<typeof createServer>>;
let browser: Awaited<ReturnType<typeof chromium.launch>>;
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

type Rect = { top: number; bottom: number; height: number; left: number; right: number; width: number };
type Geometry = {
  viewport: string;
  seats: number;
  stage: Rect;
  band: Rect;
  clock: Rect;
  strip: Rect;
  above: Rect;
  below: Rect;
  clearanceAbove: number;
  clearanceBelow: number;
  minimum: number;
};

async function geometry(width: number, height: number, seats: number): Promise<Geometry> {
  const page = await browser.newPage({ viewport: { width, height } });
  await page.goto(`${url}src/components/PhaseLane.geometry.html?seats=${seats}`);
  const measured = await page.evaluate(({ width, height, seats }) => {
    const compact = (r: DOMRect) => ({
      top: r.top, bottom: r.bottom, height: r.height,
      left: r.left, right: r.right, width: r.width,
    });
    const stage = document.querySelector<HTMLElement>('[data-board-stage]')!.getBoundingClientRect();
    const band = document.querySelector<HTMLElement>('[data-centre-instrument]')!.getBoundingClientRect();
    const clock = document.querySelector<HTMLElement>('[data-phase-track]')!.getBoundingClientRect();
    const strip = document.querySelector<HTMLElement>('[data-hot-strip]')!.getBoundingClientRect();
    const rows = [...document.querySelectorAll<HTMLElement>('.quadrant .row.creatures')]
      .map((row) => row.getBoundingClientRect());
    const middle = band.top + band.height / 2;
    const above = rows.filter((r) => r.top + r.height / 2 < middle)
      .sort((a, b) => b.bottom - a.bottom)[0];
    const below = rows.filter((r) => r.top + r.height / 2 > middle)
      .sort((a, b) => a.top - b.top)[0];

    // Resolve the design-system minimum independently of the reservation.
    // Reading --phase-card-clearance here would mutate the oracle when that
    // implementation token is accidentally set to zero.
    const probe = document.createElement('i');
    probe.style.cssText = 'position:fixed;width:var(--sp-6);height:0';
    document.body.append(probe);
    const minimum = probe.getBoundingClientRect().width;
    probe.remove();

    return {
      viewport: `${width}x${height}`,
      seats,
      stage: compact(stage),
      band: compact(band),
      clock: compact(clock),
      strip: compact(strip),
      above: compact(above),
      below: compact(below),
      clearanceAbove: band.top - above.bottom,
      clearanceBelow: below.top - band.bottom,
      minimum,
      commanders: document.querySelectorAll('[data-cmd-state="command"]').length,
    };
  }, { width, height, seats });
  await page.close();

  expect(measured.commanders).toBe(seats);
  const { commanders: _, ...result } = measured;
  return result;
}

describe('BoardStage — the phase band is a reserved lane', () => {
  it('keeps the named clearance from command-card rows in two- and four-seat layouts at every acceptance viewport', async () => {
    const results: Geometry[] = [];
    for (const [width, height] of [[1440, 900], [1000, 900], [650, 700]]) {
      for (const seats of [2, 4]) results.push(await geometry(width, height, seats));
    }

    for (const result of results) {
      expect.soft(result.band.left, `${result.viewport}, ${result.seats} seats: felt left edge`).toBe(result.stage.left);
      expect.soft(result.band.right, `${result.viewport}, ${result.seats} seats: felt right edge`).toBe(result.stage.right);
      expect.soft(result.band.top + result.band.height / 2, `${result.viewport}, ${result.seats} seats: centred`).toBe(result.stage.top + result.stage.height / 2);
      expect.soft(result.clock.bottom, `${result.viewport}, ${result.seats} seats: tabs attach to clock`).toBe(result.strip.top);
      expect.soft(result.band.height, `${result.viewport}, ${result.seats} seats: reservation includes clock and tabs`).toBe(result.clock.height + result.strip.height);
      expect.soft(result.clearanceAbove, `${result.viewport}, ${result.seats} seats: above`).toBeGreaterThanOrEqual(result.minimum);
      expect.soft(result.clearanceBelow, `${result.viewport}, ${result.seats} seats: below`).toBeGreaterThanOrEqual(result.minimum);
    }
  });

  it('keeps every tab keyboard reachable, opens on focus, closes on Escape, and has a contiguous pointer path', async () => {
    const page = await browser.newPage({ viewport: { width: 1000, height: 900 } });
    await page.goto(`${url}src/components/PhaseLane.geometry.html?seats=2`);
    const tabs = page.locator('[data-hot-tab]');
    await expect.poll(() => tabs.count()).toBe(5);
    for (let i = 0; i < 5; i++) await expect.soft(tabs.nth(i).getAttribute('aria-label')).resolves.toBeTruthy();

    const actions = page.locator('[data-hot-tab="actions"]');
    const panel = page.locator('[data-hot-panel="actions"]');
    await actions.focus();
    await expect.poll(() => panel.evaluate((e) => getComputedStyle(e).visibility)).toBe('visible');
    await page.keyboard.press('Escape');
    await expect.poll(() => panel.evaluate((e) => getComputedStyle(e).visibility)).toBe('hidden');

    await actions.hover();
    const [tabRect, panelRect] = await Promise.all([actions.boundingBox(), panel.boundingBox()]);
    expect(tabRect).not.toBeNull();
    expect(panelRect).not.toBeNull();
    expect.soft(panelRect!.y).toBe(tabRect!.y + tabRect!.height);
    await page.mouse.move(panelRect!.x + panelRect!.width / 2, panelRect!.y + 2);
    await new Promise((resolve) => setTimeout(resolve, 220));
    await expect.poll(() => panel.evaluate((e) => getComputedStyle(e).visibility)).toBe('visible');
    await page.close();
  });

  it('keeps a seven-card opening hand on one non-scrolling row at every acceptance viewport', async () => {
    for (const [width, height] of [[1440, 900], [1000, 900], [650, 700]]) {
      const page = await browser.newPage({ viewport: { width, height } });
      await page.goto(`${url}src/components/OpeningHand.geometry.html`);
      const measured = await page.evaluate(() => {
        const compact = (r: DOMRect) => ({ top: r.top, bottom: r.bottom, width: r.width, height: r.height });
        const row = document.querySelector<HTMLElement>('[data-opening-hand]')!;
        const cards = [...row.querySelectorAll<HTMLElement>('[data-obj]')].map((el) => compact(el.getBoundingClientRect()));
        return {
          row: compact(row.getBoundingClientRect()), cards,
          clientWidth: row.clientWidth, scrollWidth: row.scrollWidth,
        };
      });
      expect.soft(measured.cards, `${width}x${height}: seven cards`).toHaveLength(7);
      expect.soft(new Set(measured.cards.map((r) => r.top)).size, `${width}x${height}: one row`).toBe(1);
      expect.soft(measured.scrollWidth, `${width}x${height}: no sideways scroll`).toBeLessThanOrEqual(measured.clientWidth);
      expect.soft(Math.min(...measured.cards.map((r) => r.width)), `${width}x${height}: positive thumbnails`).toBeGreaterThan(0);
      await page.close();
    }

    // Seven is the tight case measured above; every smaller London hand uses
    // the same non-wrapping contract rather than falling back to the old CSS.
    for (let count = 0; count < 7; count++) {
      const page = await browser.newPage({ viewport: { width: 650, height: 700 } });
      await page.goto(`${url}src/components/OpeningHand.geometry.html?cards=${count}`);
      const measured = await page.evaluate(() => {
        const row = document.querySelector<HTMLElement>('[data-opening-hand]')!;
        const tops = [...row.querySelectorAll<HTMLElement>('[data-obj]')]
          .map((el) => el.getBoundingClientRect().top);
        return { count: tops.length, rows: new Set(tops).size, clientWidth: row.clientWidth, scrollWidth: row.scrollWidth };
      });
      expect.soft(measured.count, `${count} cards: all rendered`).toBe(count);
      expect.soft(measured.rows, `${count} cards: at most one row`).toBeLessThanOrEqual(1);
      expect.soft(measured.scrollWidth, `${count} cards: no sideways scroll`).toBeLessThanOrEqual(measured.clientWidth);
      await page.close();
    }
  });

  it('PASS is unavailable without a pass option and posts a non-positional pass by its wire index', async () => {
    const unavailable = await browser.newPage({ viewport: { width: 1000, height: 900 } });
    await unavailable.goto(`${url}src/components/PhaseLane.geometry.html?decision=choose`);
    await expect.poll(() => unavailable.locator('[data-hot-tab="pass"]').getAttribute('aria-disabled')).toBe('true');
    await unavailable.locator('[data-hot-tab="pass"]').focus();
    await expect.poll(() => unavailable.locator('[data-hot-panel="pass"]').textContent()).toContain('not offered');
    expect(await unavailable.locator('[data-pass-action]').count()).toBe(0);
    await unavailable.close();

    const page = await browser.newPage({ viewport: { width: 1000, height: 900 } });
    let intent: { choices?: number[] } | null = null;
    await page.route('**/api/tables/fixture/matches/1/intent', async (route) => {
      intent = route.request().postDataJSON() as { choices?: number[] };
      await route.fulfill({ status: 204, body: '' });
    });
    await page.goto(`${url}src/components/PhaseLane.geometry.html`);
    await page.locator('[data-hot-tab="pass"]').focus();
    await page.locator('[data-pass-action]').click();
    await expect.poll(() => intent?.choices).toEqual([42]);
    await page.close();
  });
});
