import { beforeAll, describe, expect, it } from 'vitest';
import { render } from 'svelte/server';
import type { Browser, Page } from 'playwright';
import { browserURL, sharedBrowser } from '../test/browser';
import NewEvent from './NewEvent.svelte';
import DraftEntry from '../components/DraftEntry.svelte';

// SSR, the repo's component-test convention: onMount never runs, so nothing
// is fetched; sets0/personas0 stand in for the two discovery replies. The
// request the button sends is pinned in lib/eventmisc.test.ts (createRequest).
const SETS = [
  { code: 'BLB', name: 'Bloomburrow', pack_size: 14, ratings: 'heuristic' },
  { code: 'FDN', name: 'Foundations', pack_size: 14, ratings: '17lands' },
];
describe('/events/new', () => {
  it('renders the 4-seat pod: you plus three bot seats with a policy and persona select each, Bo1, and the server\'s sets', () => {
    const { body } = render(NewEvent, { props: { sets0: SETS, personas0: ['best23', 'rating-colour'] } });
    expect(body).toContain('Events / New');
    expect(body).toMatch(/data-testid="seat-1-human"[^>]*>You</);
    expect(body).toContain('human · you type your name next');
    for (const n of [2, 3, 4]) expect(body).toContain(`data-testid="seat-${n}-policy"`);
    expect(body.match(/>Plays: server default<\/option>/g)).toHaveLength(3);
    expect(body).toMatch(/aria-pressed="true"[^>]*data-testid="best-of-1"/);
    expect(body).toMatch(/aria-pressed="false"[^>]*data-testid="best-of-3"/);
    expect(body).not.toContain('Best of 3 (later)');
    expect(body).toContain('value="FDN"');
    expect(body).toContain('value="BLB"');
    expect(body).toContain('Bloomburrow');
    expect(body).toContain('14-card packs · ratings: card heuristic');
    expect(body).toContain('14-card packs · ratings: 17Lands');
    for (const n of [2, 3, 4]) expect(body).toContain(`data-testid="seat-${n}-drafter"`);
    expect(body.match(/>Drafter: best23<\/option>/g)).toHaveLength(3);
    expect(body.match(/>Drafter: server default \(rating-colour\)<\/option>/g)).toHaveLength(3);
    // BLB (heuristic) is the first set, so it is chosen and no 17Lands credit shows.
    expect(body).not.toContain('data-testid="attribution"');
    expect(body).toContain('Open the pod');
    expect(body).toContain('Only bots that cannot see your decklist are offered.');
    expect(body).not.toContain('Inviting more humans comes later');
  });

  it('every seat has a kind control: seat 1 starts as you (human), seats 2–4 as bots', () => {
    const { body } = render(NewEvent, { props: { sets0: SETS, personas0: [] } });
    expect(body).toMatch(/aria-pressed="true"[^>]*data-testid="seat-1-human"/);
    expect(body).toMatch(/aria-pressed="false"[^>]*data-testid="seat-1-bot"/);
    for (const n of [2, 3, 4]) {
      expect(body).toMatch(new RegExp(`aria-pressed="false"[^>]*data-testid="seat-${n}-human"`));
      expect(body).toMatch(new RegExp(`aria-pressed="true"[^>]*data-testid="seat-${n}-bot"`));
    }
    expect(body).not.toContain('data-testid="seat-1-policy"');
    expect(body).toContain('>Invite human<');
  });

  it('with one human the pick timer is disabled and shows Off', () => {
    const { body } = render(NewEvent, { props: { sets0: SETS, personas0: [] } });
    for (const m of ['off', 'standard', 'relaxed']) expect(body).toMatch(new RegExp(`disabled[^>]*data-testid="timer-${m}"`));
    expect(body).toMatch(/aria-pressed="true"[^>]*data-testid="timer-off"/);
    expect(body).toContain('A pod with one human drafts without a timer.');
  });

  it('a 17lands-rated set carries the credit; nothing is hard-coded before the sets load', () => {
    const rated = render(NewEvent, { props: { sets0: [SETS[1]], personas0: [] } }).body;
    expect(rated).toContain('data-testid="attribution"');
    const loading = render(NewEvent, {}).body;
    expect(loading).toContain('Loading the sets');
    expect(loading).not.toContain('value="FDN"');
    expect(loading).toMatch(/data-testid="open-pod"[^>]*disabled|disabled[^>]*data-testid="open-pod"/);
    const none = render(NewEvent, { props: { sets0: [] } }).body;
    expect(none).toContain('This server offers no draftable sets.');
  });
});

// The interactive half runs the real component in the shared Chromium
// (src/test/browser.global.ts) against NewEvent.fixture.ts's recording
// fetch double: the request body the click sends, the Invites panel, and
// whether the page navigated.
describe('/events/new in a browser', () => {
  let browser: Browser;
  beforeAll(async () => {
    browser = await sharedBrowser();
  });

  async function openFixture(): Promise<{ page: Page; close: () => Promise<void> }> {
    const ctx = await browser.newContext({ permissions: ['clipboard-read', 'clipboard-write'] });
    const page = await ctx.newPage();
    await page.goto(`${browserURL}src/routes/NewEvent.fixture.html`);
    await page.getByRole('radio', { name: /FDN/ }).waitFor();
    return { page, close: () => ctx.close() };
  }
  const bodies = (page: Page) => page.evaluate(() => (window as unknown as { createBodies: unknown[] }).createBodies);

  it('one human: sends no pick_timer, no best_of for Bo1, and navigates to the invite as today', async () => {
    const { page, close } = await openFixture();
    await page.getByTestId('open-pod').click();
    await page.waitForFunction(() => location.pathname === '/e/ev7');
    const sent = await bodies(page);
    await close();
    expect(sent).toEqual([{ set: 'FDN', seats: [{ kind: 'human' }, { kind: 'bot' }, { kind: 'bot' }, { kind: 'bot' }] }]);
  });

  it('two humans, Bo3: the timer enables (standard by default, unsent), and the Invites panel shows each link once without navigating', async () => {
    const { page, close } = await openFixture();
    await expect.poll(() => page.getByTestId('timer-standard').isDisabled()).toBe(true);
    await page.getByTestId('seat-3-human').click();
    await expect.poll(() => page.getByTestId('timer-standard').isDisabled()).toBe(false);
    expect(await page.getByTestId('timer-standard').getAttribute('aria-pressed')).toBe('true');
    await page.getByTestId('best-of-3').click();
    await page.getByTestId('open-pod').click();
    await page.getByTestId('invites').waitFor();
    const rows = page.getByTestId('invite-row');
    expect(await rows.count()).toBe(2);
    expect(await page.getByTestId('copy-invite').count()).toBe(2);
    expect(await page.getByRole('note').textContent()).toContain('These links are shown once. Copy them now.');
    // The token is masked until asked for; Copy copies the full link.
    const text = await page.getByTestId('invites').textContent();
    expect(text).not.toContain('tok2secret');
    await page.getByTestId('copy-invite').nth(1).click();
    await expect.poll(() => page.getByTestId('copy-invite').nth(1).textContent()).toContain('Copied');
    const clip = await page.evaluate(() => navigator.clipboard.readText());
    expect(clip).toBe(`${new URL(browserURL).origin}/e/ev7?seat=2&token=tok2secret`);
    expect(await page.evaluate(() => location.pathname)).toBe('/events/new');
    const sent = await bodies(page);
    expect(sent).toEqual([{ set: 'FDN', best_of: 3, seats: [{ kind: 'human' }, { kind: 'bot' }, { kind: 'human' }, { kind: 'bot' }] }]);
    // "Go to my seat" follows the first invite.
    await page.getByTestId('go-to-seat').click();
    await page.waitForFunction(() => location.pathname === '/e/ev7' && location.search === '?seat=0&token=tok0secret');
    await close();
  });

  it('a chosen pick timer is sent; it is dropped again when the pod falls back to one human', async () => {
    const { page, close } = await openFixture();
    await page.getByTestId('seat-2-human').click();
    await page.getByTestId('timer-relaxed').click();
    await page.getByTestId('seat-4-human').click();
    await page.getByTestId('open-pod').click();
    await page.getByTestId('invites').waitFor();
    expect(await page.getByTestId('invite-row').count()).toBe(3);
    const sent = (await bodies(page)) as { pick_timer?: string }[];
    expect(sent[0].pick_timer).toBe('relaxed');
    await close();

    const again = await openFixture();
    await again.page.getByTestId('seat-2-human').click();
    await again.page.getByTestId('timer-relaxed').click();
    await again.page.getByTestId('seat-2-bot').click();
    expect(await again.page.getByTestId('timer-off').getAttribute('aria-pressed')).toBe('true');
    await again.page.getByTestId('open-pod').click();
    await again.page.waitForFunction(() => location.pathname === '/e/ev7');
    const sent2 = (await bodies(again.page)) as Record<string, unknown>[];
    expect(sent2[0]).not.toHaveProperty('pick_timer');
    await again.close();
  });
});

describe('Overview draft entry', () => {
  it('is hidden until the server says it serves events', () => {
    const { body } = render(DraftEntry, {});
    expect(body).not.toContain('data-testid="draft-entry"');
    expect(body).not.toContain('href="/events/new"');
  });

  it('links to /events/new beside play-vs-bot on a server that serves events', () => {
    const { body } = render(DraftEntry, { props: { initialEnabled: true } });
    expect(body).toContain('href="/events/new"');
    expect(body).toContain('>Draft<');
  });
});
