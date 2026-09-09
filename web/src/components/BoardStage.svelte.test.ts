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
    const band = document.querySelector<HTMLElement>('[data-phase-track]')!.getBoundingClientRect();
    const rows = [...document.querySelectorAll<HTMLElement>('.quadrant .row.creatures')]
      .map((row) => row.getBoundingClientRect());
    const middle = band.top + band.height / 2;
    const above = rows.filter((r) => r.top + r.height / 2 < middle)
      .sort((a, b) => b.bottom - a.bottom)[0];
    const below = rows.filter((r) => r.top + r.height / 2 > middle)
      .sort((a, b) => a.top - b.top)[0];

    // Resolve the named CSS token through layout rather than duplicating its
    // value in the test; getPropertyValue would preserve `var(--sp-6)`.
    const probe = document.createElement('i');
    probe.style.cssText = 'position:fixed;width:var(--phase-card-clearance);height:0';
    document.body.append(probe);
    const minimum = probe.getBoundingClientRect().width;
    probe.remove();

    return {
      viewport: `${width}x${height}`,
      seats,
      stage: compact(stage),
      band: compact(band),
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
      expect.soft(result.clearanceAbove, `${result.viewport}, ${result.seats} seats: above`).toBeGreaterThanOrEqual(result.minimum);
      expect.soft(result.clearanceBelow, `${result.viewport}, ${result.seats} seats: below`).toBeGreaterThanOrEqual(result.minimum);
    }
  });
});
