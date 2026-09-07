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
