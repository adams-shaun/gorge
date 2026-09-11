import { chromium } from 'playwright';
import { createServer } from 'vite';
import { afterAll, beforeAll, describe, expect, it } from 'vitest';
import { render } from 'svelte/server';
import type { CardView, PlayerView, SeatInfo, View } from '../protocol';
import SeatTable from './SeatTable.svelte';

const card = (id: number, name = `Card ${id}`): CardView => ({
  id, name, types: 'Instant', printing: { name }, token: `#${id}`, tapped: false,
  power: 0, toughness: 0, damage: 0, attacking: false, controller: 0, owner: 0,
  summon_sick: false,
});

const player = (over: Partial<PlayerView> = {}): PlayerView => ({
  seat: 0, name: 'Ari', life: 40, lost: false, library_size: 52, hand_size: 2, graveyard_size: 1,
  hand: [card(1), card(2)], battlefield: [], graveyard: [card(3)], exile: [card(4)],
  pool: {}, command: [], commanders: [], commander_casts: [], ...over,
});

const summaryView = (p: PlayerView): View => ({
  viewer: 0, visibility: 'seat', turn: 1, round: 1, step: 'main1', phase: 'main1',
  active: 0, priority: 0, over: false, draw: false, winner: null, stack: [], pending: [], players: [p],
});

const summarySeats: SeatInfo[] = [{ name: 'Ari', deck: 'red', colour: '#e5484d' }];

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

describe('SeatTable — compact seat summary', () => {
  it('renders the five icon/count cells in order and gives only disclosable piles a caret control', () => {
    const { html } = render(SeatTable, { props: { view: summaryView(player()), seats: summarySeats, onFocus: () => {} } });
    const kinds = ['life', 'hand', 'library', 'graveyard', 'exile'];
    let at = -1;
    for (const kind of kinds) {
      const next = html.indexOf(`data-stat=\"${kind}\"`);
      expect(next).toBeGreaterThan(at);
      at = next;
    }
    expect(html).toContain('data-icon="heart"');
    expect(html).toContain('data-icon="book"');
    expect(html).toContain('data-pile="hand"');
    expect(html).toContain('data-pile="graveyard"');
    expect(html).toContain('data-pile="exile"');
    expect(html).not.toContain('data-pile="library"');
  });

  it('stacks the four zone counts two-high: hand+library on the first line, graveyard+exile on the second', () => {
    const { html } = render(SeatTable, { props: { view: summaryView(player()), seats: summarySeats, onFocus: () => {} } });
    expect(html.match(/zone-line/g)?.length).toBe(2);
    const first = html.slice(html.indexOf('zone-line'), html.lastIndexOf('zone-line'));
    const second = html.slice(html.lastIndexOf('zone-line'));
    // life stays on the name line, outside both zone lines
    expect(first.includes('data-stat=\"life\"')).toBe(false);
    expect(second.includes('data-stat=\"life\"')).toBe(false);
    expect(first.indexOf('data-stat=\"hand\"')).toBeGreaterThan(-1);
    expect(first.indexOf('data-stat=\"library\"')).toBeGreaterThan(first.indexOf('data-stat=\"hand\"'));
    expect(first.includes('data-stat=\"graveyard\"')).toBe(false);
    expect(first.includes('data-stat=\"exile\"')).toBe(false);
    expect(second.indexOf('data-stat=\"graveyard\"')).toBeGreaterThan(-1);
    expect(second.indexOf('data-stat=\"exile\"')).toBeGreaterThan(second.indexOf('data-stat=\"graveyard\"'));
  });

  it('shows true counts but no caret for empty or redacted lists', () => {
    const hidden = player({ hand: null as unknown as CardView[], graveyard: null as unknown as CardView[], graveyard_size: 4, exile: [] });
    const { html } = render(SeatTable, { props: { view: summaryView(hidden), seats: summarySeats, onFocus: () => {} } });
    expect(html).toContain('data-hand-hidden');
    expect(html).not.toContain('data-pile="hand"');
    expect(html).not.toContain('data-pile="graveyard"');
    expect(html).not.toContain('data-pile="exile"');
  });
});

describe('SeatTable — priority geometry', () => {
  it('keeps the seat row and name box at identical pixels while priority changes', async () => {
    const page = await browser.newPage();
    await page.goto(`${url}src/components/SeatTable.geometry.html`);
    const geometry = await page.evaluate(() => {
      const rect = (selector: string) => {
        const { x, y, width, height } = document.querySelector<HTMLElement>(selector)!.getBoundingClientRect();
        return { x, y, width, height };
      };
      return {
        idle: {
          row: rect('#seat-idle [data-seat-row="0"]'),
          pick: rect('#seat-idle [data-seat-row="0"] .pick'),
          name: rect('#seat-idle [data-seat-row="0"] .name'),
          table: rect('#seat-idle table'),
          identity: rect('#identity-idle .identity'),
          identityName: rect('#identity-idle .name'),
        },
        priority: {
          row: rect('#seat-priority [data-seat-row="0"]'),
          pick: rect('#seat-priority [data-seat-row="0"] .pick'),
          name: rect('#seat-priority [data-seat-row="0"] .name'),
          table: rect('#seat-priority table'),
          identity: rect('#identity-priority .identity'),
          identityName: rect('#identity-priority .name'),
        },
      };
    });
    await page.close();

    expect(geometry.priority).toEqual(geometry.idle);
  });
});
describe('SeatTable — the rail floor (stacked counts let the rail shrink)', () => {
  // The new grid floor in Table.svelte is 11rem = 176px. The fixture mounts
  // the whole rail (seat table, mana pool, decision line, stack tile, pending
  // tray) at exactly that width, with a 27-character seat name that was
  // ALREADY ellipsized at the old 17rem floor (needs 173px, had 64px there),
  // so the assertions below are measured facts about the reduced rail, not
  // about the name change.
  const FLOOR_PX = 176;

  async function atFloor() {
    const page = await browser.newPage();
    await page.goto(`${url}src/components/SeatTable.geometry.html`);
    await page.waitForSelector('#rail .rail-inner');
    await page.evaluate((px) => document.documentElement.style.setProperty('--rail-w', `${px}px`), FLOOR_PX);
    return page;
  }

  it('fits every rail section horizontally at the 11rem floor', async () => {
    const page = await atFloor();
    const overflows = await page.evaluate(() => {
      const bad: { label: string; sw: number; cw: number }[] = [];
      const check = (el: HTMLElement, label: string) => {
        if (el.scrollWidth > el.clientWidth + 1) bad.push({ label, sw: el.scrollWidth, cw: el.clientWidth });
      };
      const rail = document.querySelector<HTMLElement>('#rail .rail-inner')!;
      check(rail, 'rail-inner');
      for (const child of Array.from(rail.children)) check(child as HTMLElement, child.className || child.tagName);
      return bad;
    });
    await page.close();
    expect(overflows).toEqual([]);
  });

  it('keeps the long seat name inside its row with the ellipsis doing the work', async () => {
    const page = await atFloor();
    const name = await page.evaluate(() => {
      const el = document.querySelector<HTMLElement>('#rail [data-seat-row="0"] .name')!;
      const row = document.querySelector<HTMLElement>('#rail [data-seat-row="0"]')!;
      const nr = el.getBoundingClientRect();
      const rr = row.getBoundingClientRect();
      return { right: nr.right, rowRight: rr.right, scroll: el.scrollWidth, client: el.clientWidth };
    });
    await page.close();
    expect(name.right).toBeLessThanOrEqual(name.rowRight + 1);
    // the ellipsis is active, so the name is truncated by dots, not clipped mid-glyph
    expect(name.scroll).toBeGreaterThan(name.client);
  });

  it('leaves room for the "Concede — confirm" control Table anchors inside the rail', async () => {
    const page = await atFloor();
    const fits = await page.evaluate((floor) => {
      // Replicates Table.svelte's .concede-control button metrics: font
      // --t-12, padding sp-1/sp-2, 1px border, white-space: nowrap; the
      // control is absolutely positioned at right: sp-2 inside the rail.
      const probe = document.createElement('button');
      probe.textContent = 'Concede — confirm';
      probe.style.cssText = 'position:absolute;visibility:hidden;white-space:nowrap;padding:0.25rem 0.5rem;border:1px solid #000;font-size:0.75rem;font-family:var(--font-ui);';
      document.body.appendChild(probe);
      const w = probe.getBoundingClientRect().width;
      probe.remove();
      const rail = document.querySelector<HTMLElement>('#rail .rail-inner')!;
      return { button: w, needed: w + 8, rail: rail.clientWidth, floor };
    }, FLOOR_PX);
    await page.close();
    expect(fits.needed).toBeLessThanOrEqual(fits.rail);
  });
});
