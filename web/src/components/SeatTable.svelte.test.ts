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
