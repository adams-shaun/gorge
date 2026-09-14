import { chromium } from 'playwright';
import { createServer } from 'vite';
import { afterAll, beforeAll, describe, expect, it } from 'vitest';

/**
 * Arrows — the overlay's geometry pin, driven against the production table
 * hierarchy at real layout (see Arrows.geometry.ts for why a fixture mounts
 * the production components rather than SSR-ing the live route: Table.svelte
 * needs a streaming backend to render a board at all).
 *
 * fb-20260914T121642Z: a stack-to-stack target arrow — a counterspell
 * targeting the spell beneath it — was computed but invisible, because the
 * overlay lived inside Board.svelte, which the route mounts inside the felt
 * section whose `overflow: hidden` clips anything reaching into the rail.
 * The pin must fail before the overlay is moved to the table root (the
 * overlay is then a descendant of the clipped felt section and/or not the
 * only one) and pass after it, and it must also pin the rail-scroll
 * alignment: section.stack is a real scroller, and scroll does not bubble,
 * so a line whose endpoints are rail tiles goes stale the moment the stack
 * scrolls unless the overlay listens for it.
 *
 * All assertions run against ONE page load: this file shares the machine
 * with the other browser suites in the same vitest run, and every page load
 * and dev server it starts is contention another file's timing-sensitive
 * assertions have to survive (measured: the suite flapped until this file
 * was cut to a single page).
 */

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

type Rect = { left: number; top: number; right: number; bottom: number; width: number; height: number };
type Overlay = { inTableRoot: boolean; inClippedFelt: boolean; rect: Rect };
type Line = { x1: number; y1: number; x2: number; y2: number };
type Measured = { ovs: Overlay[]; counterspell: Rect; bolt: Rect; line: Line; scrollable: boolean };
type Endpoint = { bolt: Rect; endpointX: number; endpointY: number };

// Everything below runs inside page.evaluate, so it is self-contained: the
// browser context sees only the serialized callback, never this module's
// scope.
const IN_PAGE = `
  const compact = (r) => ({ left: r.left, top: r.top, right: r.right, bottom: r.bottom, width: r.width, height: r.height });
  const overlays = () => {
    const table = document.querySelector('main.table');
    const felt = document.querySelector('main.table > .board');
    return [...document.querySelectorAll('.arrows')].map((el) => {
      const r = el.getBoundingClientRect();
      return {
        inTableRoot: el.parentElement === table,
        // "inside the clipped felt section": a descendant of the element the
        // route clips, so any line reaching into the rail is cut at the seam.
        inClippedFelt: felt.contains(el),
        rect: compact(r),
      };
    });
  };
  const lineAttrs = () => {
    const l = document.querySelector('.arrows line.line--target');
    return l ? { x1: Number(l.getAttribute('x1')), y1: Number(l.getAttribute('y1')), x2: Number(l.getAttribute('x2')), y2: Number(l.getAttribute('y2')) } : null;
  };
  const tile = (id) => compact(document.querySelector('[data-obj="' + id + '"]').getBoundingClientRect());
`;

const near = (a: number, b: number): boolean => Math.abs(a - b) <= 1;

describe('Arrows — the overlay escapes the felt clip and tracks the rail', () => {
  it('hosts one arrows overlay at the table root (never inside the clipped felt section), its bounds include the rail target tile, the line lands on both tiles, and the endpoint follows the rail scroll', async () => {
    const page = await browser.newPage({ viewport: { width: 1200, height: 800 } });
    await page.goto(`${url}src/components/Arrows.geometry.html`);
    for (const sel of ['[data-obj="900"]', '[data-obj="890"]', '.arrows']) {
      await page.waitForSelector(sel, { state: 'attached', timeout: 60_000 });
    }
    // The first measurement is rAF-deferred behind the mount's DOM update.
    await page.waitForFunction(() => {
      const l = document.querySelector('.arrows line.line--target');
      return l !== null && Number((l as SVGLineElement).getAttribute('x2')) !== 0;
    }, undefined, { timeout: 60_000 });

    const m = (await page.evaluate(`${IN_PAGE}
      (() => {
        const scroller = document.querySelector('section.stack');
        return {
          ovs: overlays(),
          counterspell: tile(900),
          bolt: tile(890),
          line: lineAttrs(),
          scrollable: scroller.scrollHeight > scroller.clientHeight,
        };
      })()`)) as unknown as Measured;

    // --- the fixture renders the brief's case for real ---
    expect(m.ovs.length).toBeGreaterThanOrEqual(1);
    expect(m.line, 'exactly one resolved target arrow: the counterspell\u2019s').not.toBeNull();
    // The counterspell and bolt tiles sit at DIFFERENT heights in the rail
    // (the counterspell is the upper entry), and the stack genuinely
    // overflows its scroller, so the scroll leaf below is not vacuous.
    expect(m.counterspell.top).toBeLessThan(m.bolt.top - 1);
    expect(m.scrollable, 'section.stack really overflows at the test viewport').toBe(true);

    // --- fb-20260914T121642Z: the overlay escapes the felt clip ---
    // Before the fix there are two overlays: the old mount inside Board
    // (clipped by the felt section) and the table-root host this pin
    // requires. Exactly one, at the table root, is the production contract.
    expect(m.ovs, 'the arrows overlay is mounted once, as Table.svelte hosts it').toHaveLength(1);
    for (const o of m.ovs) {
      expect(o.inTableRoot, 'the overlay is a direct child of main.table').toBe(true);
      expect(o.inClippedFelt, 'the overlay is not a descendant of the clipped felt section').toBe(false);
      // The bolt tile renders in the rail, right of the felt; the overlay
      // must span at least that far.
      expect(o.rect.left).toBeLessThanOrEqual(m.bolt.left + 1);
      expect(o.rect.right).toBeGreaterThanOrEqual(m.bolt.right - 1);
      expect(o.rect.top).toBeLessThanOrEqual(m.bolt.top + 1);
      expect(o.rect.bottom).toBeGreaterThanOrEqual(m.bolt.bottom - 1);
      // The resolved target arrow runs counterspell -> bolt. Line coordinates
      // are relative to the overlay's own box, so the viewport endpoint is
      // overlay origin + (x2, y2); it must be the bolt tile's centre, and the
      // tail the counterspell's.
      expect(near(o.rect.left + m.line.x2, m.bolt.left + m.bolt.width / 2), 'line x2 is the bolt tile centre').toBe(true);
      expect(near(o.rect.top + m.line.y2, m.bolt.top + m.bolt.height / 2), 'line y2 is the bolt tile centre').toBe(true);
      expect(near(o.rect.left + m.line.x1, m.counterspell.left + m.counterspell.width / 2), 'line x1 is the counterspell tile centre').toBe(true);
      expect(near(o.rect.top + m.line.y1, m.counterspell.top + m.counterspell.height / 2), 'line y1 is the counterspell tile centre').toBe(true);
    }

    // --- and it stays aligned when the rail's stack scrolls ---
    const before = (await page.evaluate(`${IN_PAGE}
      (() => {
        const o = overlays()[0].rect;
        const l = lineAttrs();
        return { bolt: tile(890), endpointX: o.left + l.x2, endpointY: o.top + l.y2 };
      })()`)) as unknown as Endpoint;
    // The stack is the rail's one scroller and scroll does not bubble, so
    // the fixture scrolls it and dispatches the scroll event a real scroll
    // would fire; the overlay must recompute from that signal alone (no
    // polling, no timer).
    await page.evaluate(() => {
      const scroller = document.querySelector('section.stack')!;
      scroller.scrollTop += 140;
      scroller.dispatchEvent(new Event('scroll'));
    });
    await page.waitForFunction(() => {
      const l = document.querySelector('.arrows line.line--target')!;
      const b = document.querySelector('[data-obj="890"]')!.getBoundingClientRect();
      const o = document.querySelector('.arrows')!.getBoundingClientRect();
      return Math.abs(o.left + Number(l.getAttribute('x2')) - (b.left + b.width / 2)) <= 1
        && Math.abs(o.top + Number(l.getAttribute('y2')) - (b.top + b.height / 2)) <= 1;
    }, undefined, { timeout: 60_000 });
    const after = (await page.evaluate(`${IN_PAGE}
      (() => {
        const o = overlays()[0].rect;
        const l = lineAttrs();
        return { bolt: tile(890), endpointX: o.left + l.x2, endpointY: o.top + l.y2 };
      })()`)) as unknown as Endpoint;

    // The scroll actually moved the tile (otherwise the assertion is vacuous).
    expect(Math.abs(after.bolt.top - before.bolt.top), 'the scroll moved the bolt tile').toBeGreaterThanOrEqual(1);
    // And the endpoint followed it, in the overlay's own coordinate space.
    expect(near(after.endpointX, after.bolt.left + after.bolt.width / 2), 'scrolled endpoint x lands on the tile').toBe(true);
    expect(near(after.endpointY, after.bolt.top + after.bolt.height / 2), 'scrolled endpoint y lands on the tile').toBe(true);
    expect(near(before.endpointX, before.bolt.left + before.bolt.width / 2), 'pre-scroll endpoint was on the tile too').toBe(true);
    await page.close();
  }, 120_000);
});
