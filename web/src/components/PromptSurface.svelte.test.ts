import { chromium, type Page } from 'playwright';
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

/**
 * The prompt-surface browser gate (brief Jobs 1, 2 and 4): a real browser,
 * because the asks are about what a player can SEE and do — a required
 * prompt visible with the ACTIONS dropdown never opened, and the arrange
 * popup ordering cards by click and by drag and posting the picked order.
 * The fixture mounts the strip and the board prompt surface the way Table
 * mounts them, sharing one SeatPanelState; the arrange case captures the
 * posted intent at window.__posted so the wire shape is asserted without a
 * server.
 */
describe('PromptSurface', () => {
  it('a required prompt is visible and answerable with the ACTIONS dropdown closed', async () => {
    const page = await browser.newPage();
    await page.goto(`${url}src/components/PromptSurface.fixture.html`);

    const prompt = page.locator('[data-answer-surface] [data-prompt]');
    expect(await prompt.isVisible()).toBe(true);
    expect(await prompt.textContent()).toContain('Choose a target');
    // The prompt surface is NOT inside the (closed) ACTIONS drop: it is its
    // own mount on the board. The drop's own panel — the strip's copy — is a
    // different element, so this count is 0.
    expect(await page.locator('#hot-panel-actions [data-answer-surface]').count()).toBe(0);
    // And the board surface's option list is reachable right there: the seat
    // can answer without ever opening the dropdown.
    const option = page.locator('[data-answer-surface] [data-option="0"]');
    expect(await option.isVisible()).toBe(true);
    await option.click();
    // Answered: the surface drops the DECISION (it stays mounted for the
    // waiting state), so nothing is left to answer on it. The post is async,
    // so wait for the option list to drop rather than racing it.
    await page.waitForFunction(() => document.querySelectorAll('[data-answer-surface] [data-option]').length === 0);

    await page.close();
  });

  it('an initiative decision keeps the ACTIONS drop free of the option list (the prompt/action split)', async () => {
    const page = await browser.newPage();
    await page.goto(`${url}src/components/PromptSurface.fixture.html`);

    // The drop's SeatPanel names the prompt as living on the board and offers
    // no second posting route for it.
    expect(await page.locator('[data-strip-pointer]').count()).toBe(1);
    expect(await page.locator('#hot-panel-actions [data-option]').count()).toBe(0);
    // The board surface carries the options.
    expect(await page.locator('[data-answer-surface] [data-option]').count()).toBe(1);

    await page.close();
  });
});


describe('ArrangeModal (via the seat panel)', () => {
  /** The HTML5 drag event sequence a real drag produces, dispatched on the keep row. */
  const dragKeepCard = (page: Page, from: number, onto: number) =>
    page.evaluate(([f, t]) => {
      const dt = new DataTransfer();
      const source = document.querySelector(`[data-arrange-keep-card="${f}"]`)!;
      const target = document.querySelector(`[data-arrange-keep-card="${t}"]`)!;
      source.dispatchEvent(new DragEvent('dragstart', { dataTransfer: dt, bubbles: true }));
      target.dispatchEvent(new DragEvent('dragover', { dataTransfer: dt, bubbles: true, cancelable: true }));
      target.dispatchEvent(new DragEvent('drop', { dataTransfer: dt, bubbles: true, cancelable: true }));
      source.dispatchEvent(new DragEvent('dragend', { dataTransfer: dt, bubbles: true }));
    }, [from, onto] as const);

  it('a pure reorder opens as a reorder surface — every card kept in offered order, drop guarded, drag reorder posted', async () => {
    const page = await browser.newPage();

    await page.goto(`${url}src/components/PromptSurface.fixture.html?case=arrange`);

    await page.locator('[data-arrange-open]').click();
    const keep = page.locator('[data-arrange-keep]');
    expect(await keep.isVisible()).toBe(true);
    // The MINOR fix's initial state, in a real browser: a fresh pure reorder
    // opens with EVERY card in the keep row, in offered order, and no pool
    // row — it is a reorder surface from the first paint, not a picking one.
    expect(await keep.locator('[data-arrange-keep-card]').count()).toBe(5);
    expect(await page.locator('[data-arrange-pool-card]').count()).toBe(0);
    expect(await page.locator('[data-arrange-submit]').isDisabled()).toBe(false);

    // The drop guard: on a pure reorder nothing can leave the keep pile (the
    // pool it would drop to does not exist), so clicking a kept card is a no-op.
    await keep.locator('[data-arrange-keep-card="2"] button.face').click();
    expect(await keep.locator('[data-arrange-keep-card]').count()).toBe(5);
    expect(await page.locator('[data-arrange-submit]').isDisabled()).toBe(false);

    // Drag the keep pile's card 3 onto card 0: the pile reads 3, 0, 1, 2, 4.
    await dragKeepCard(page, 3, 0);
    expect(await keep.locator('[data-arrange-keep-card="3"] button.face').getAttribute('aria-label')).toBe('1: Spell Pierce');
    expect(await keep.locator('[data-arrange-keep-card="0"] button.face').getAttribute('aria-label')).toBe('2: Brazen Borrower');

    await page.locator('[data-arrange-submit]').click();
    const posted = JSON.parse(await page.evaluate(() => (window as unknown as { __posted?: string }).__posted ?? 'null')) as { seq: number; player: number; choices: number[] };
    expect(posted.seq).toBe(7);
    expect(posted.player).toBe(0);
    expect(posted.choices).toEqual([3, 0, 1, 2, 4]);

    await page.close();
  });

  it('a scry keeps cards by click from the pool, orders them by drag, and posts the picked order', async () => {
    const page = await browser.newPage();

    await page.goto(`${url}src/components/PromptSurface.fixture.html?case=scry`);

    await page.locator('[data-arrange-open]').click();
    const keep = page.locator('[data-arrange-keep]');
    const pool = page.locator('[data-arrange-pool]');
    expect(await keep.isVisible()).toBe(true);
    expect(await keep.locator('[data-arrange-keep-card]').count()).toBe(0);
    expect(await pool.locator('[data-arrange-pool-card]').count()).toBe(5);

    // Click three pool cards into the keep pile (click-to-place).
    await pool.locator('[data-arrange-pool-card="1"]').click();
    await pool.locator('[data-arrange-pool-card="0"]').click();
    await pool.locator('[data-arrange-pool-card="3"]').click();
    expect(await keep.locator('[data-arrange-keep-card]').count()).toBe(3);
    expect(await keep.locator('[data-arrange-keep-card="3"] button.face').getAttribute('aria-label')).toBe('3: Spell Pierce');
    expect(await keep.locator('[data-arrange-keep-card="0"] button.face').getAttribute('aria-label')).toBe('2: Brazen Borrower');

    // Drag the keep pile's last card (3) onto its first (1): the pile reads
    // 3, 1, 0.
    await dragKeepCard(page, 3, 1);
    expect(await keep.locator('[data-arrange-keep-card="3"] button.face').getAttribute('aria-label')).toBe('1: Spell Pierce');
    expect(await keep.locator('[data-arrange-keep-card="1"] button.face').getAttribute('aria-label')).toBe('2: Fabled Pass');

    // Keep the last two cards, then submit the keep pile's order.
    await pool.locator('[data-arrange-pool-card="2"]').click();
    await pool.locator('[data-arrange-pool-card="4"]').click();
    await page.locator('[data-arrange-submit]').click();

    const posted = JSON.parse(await page.evaluate(() => (window as unknown as { __posted?: string }).__posted ?? 'null')) as { seq: number; player: number; choices: number[] };
    expect(posted.seq).toBe(7);
    expect(posted.player).toBe(0);
    expect(posted.choices).toEqual([3, 1, 0, 2, 4]);

    await page.close();
  });

  it('the same final keep order posted through the old click-order picker is byte-identical', async () => {
    const page = await browser.newPage();

    await page.goto(`${url}src/components/PromptSurface.fixture.html?case=arrange`);

    // The old answer path, kept on the surface: clicking the arrange row's
    // cards in the final order appends each index, building exactly the
    // picked array the popup posts for the same ordering (lib/arrange.ts's
    // arrangeOrder). 3, 0, 1, 2, 4 — the same ordering the drag test posted.
    for (const i of [3, 0, 1, 2, 4]) {
      await page.locator(`[data-arrange-row] [data-option="${i}"]`).click();
    }
    await page.locator('[data-submit]').click();

    const posted = JSON.parse(await page.evaluate(() => (window as unknown as { __posted?: string }).__posted ?? 'null')) as { seq: number; player: number; choices: number[] };
    expect(posted.choices).toEqual([3, 0, 1, 2, 4]);

    await page.close();
  });

  it('a new decision while the modal is open closes it; the reopened modal is the NEW ask, never the old ask’s edits (stale-edits regression)', { timeout: 30_000 }, async () => {
    const page = await browser.newPage();

    await page.goto(`${url}src/components/PromptSurface.fixture.html?case=seqswap`);

    await page.locator('[data-arrange-open]').click();
    const keep = page.locator('[data-arrange-keep]');
    expect(await keep.locator('[data-arrange-keep-card]').count()).toBe(5);
    // Edit the keep order so the modal's edit state is live for seq 7.
    await dragKeepCard(page, 3, 0);
    expect(await keep.locator('[data-arrange-keep-card="3"] button.face').getAttribute('aria-label')).toBe('1: Spell Pierce');

    // The ask changes underneath the open popup — the two-tab path: another
    // tab answered seq 7 and the seat's next arrange ask (seq 9) arrived.
    // The swap button sits under the modal's backdrop, so the swap is
    // dispatched programmatically, exactly as an SSE view replacement would
    // land.
    await page.evaluate(() => (document.querySelector('[data-swap-decision]') as HTMLButtonElement).click());

    // The popup belonged to the old ask: it is closed, and the board now
    // asks decision B. The settle wait covers the paced art lookups (100 ms
    // apart in lib/images) finishing their layout-affecting resolution.
    await page.waitForFunction(() => document.querySelectorAll('[data-arrange-modal]').length === 0, null, { timeout: 10_000 });
    await page.waitForTimeout(1500);
    expect(await page.locator('[data-answer-surface] [data-prompt]').textContent()).toContain('Rearrange the top 3 card(s)');

    // Reopening is the NEW ask, fresh: three decision-B cards in offered
    // order — none of decision A's edits — and the posted intent is B's.
    await page.locator('[data-arrange-open]').click({ timeout: 10_000 });
    expect(await page.locator('[data-arrange-keep-card]').count()).toBe(3);
    expect(await page.locator('[data-arrange-keep-card="0"] button.face').getAttribute('aria-label')).toBe('1: Absorb Vis');
    expect(await page.locator('[data-arrange-keep-card="2"] button.face').getAttribute('aria-label')).toBe('3: Counterspell');
    await page.locator('[data-arrange-submit]').click({ timeout: 10_000 });
    const posted = JSON.parse(await page.evaluate(() => (window as unknown as { __posted?: string }).__posted ?? 'null')) as { seq: number; choices: number[] };
    expect(posted.seq).toBe(9);
    expect(posted.choices).toEqual([0, 1, 2]);

    await page.close();
  });
});

/**
 * The arrange surfaces' card detail (fb-20260914T063020Z): the strip's cards
 * get the same hover/focus CardDetail inspector every other card surface
 * has, and the modal's preview shows the printed description beside the art.
 * Browser tests, because these are asks about what a player can SEE and do
 * with a real pointer and keyboard — the SSR harness cannot hover or focus.
 */
describe('the arrange strip hover inspector (fb-20260914T063020Z Job 1)', () => {
  it('hovering a strip card opens the CardDetail inspector (portalled to <body>), describing that card', async () => {
    const page = await browser.newPage();
    await page.goto(`${url}src/components/PromptSurface.fixture.html?case=arrange`);

    const pick = page.locator('[data-arrange-row] [data-option="0"]');
    await pick.hover();
    // The panel lives on <body>, not inside the seat panel — the panel is a
    // containing block for fixed descendants (its own style NOTE), which is
    // exactly why CardDetail portals.
    const detail = page.locator('body > .card-detail');
    await detail.waitFor({ state: 'visible', timeout: 5_000 }); // the ~250ms dwell
    expect(await detail.textContent()).toContain('Brazen Borrower');
    expect(await pick.getAttribute('aria-describedby')).toBe('card-detail-11');

    // pointer leave closes it again
    await page.mouse.move(5, 5);
    await page.waitForFunction(() => document.querySelectorAll('body > .card-detail').length === 0, null, { timeout: 5_000 });

    await page.close();
  });

  it('keyboard focus opens the inspector immediately, and Escape closes it', async () => {
    const page = await browser.newPage();
    await page.goto(`${url}src/components/PromptSurface.fixture.html?case=arrange`);

    const pick = page.locator('[data-arrange-row] [data-option="0"]');
    await pick.focus();
    const detail = page.locator('body > .card-detail');
    await detail.waitFor({ state: 'visible', timeout: 5_000 });

    // No catalog in the fixture: the designed degradation is the wire-only
    // ledger — no oracle block, no art plate.
    expect(await detail.locator('.card-detail__oracle').count()).toBe(0);

    await page.keyboard.press('Escape');
    await page.waitForFunction(() => document.querySelectorAll('body > .card-detail').length === 0, null, { timeout: 5_000 });

    await page.close();
  });

  it('a stale pointerleave from card A cannot close keyboard-focused card B', async () => {
    const page = await browser.newPage();
    await page.goto(`${url}src/components/PromptSurface.fixture.html?case=arrange`);

    const cardA = page.locator('[data-arrange-row] [data-option="0"]');
    const cardB = page.locator('[data-arrange-row] [data-option="1"]');
    const detail = page.locator('body > .card-detail');

    // Open A by pointer dwell, then move keyboard ownership to B without
    // moving the pointer. Leaving A afterwards is a stale event with respect
    // to the shared panel and must not dismiss B's focus inspector.
    await cardA.hover();
    await detail.waitFor({ state: 'visible', timeout: 5_000 });
    await cardB.focus();
    expect(await detail.textContent()).toContain('Fabled Pass');
    await page.mouse.move(5, 5);
    expect(await detail.isVisible()).toBe(true);
    expect(await detail.textContent()).toContain('Fabled Pass');
    expect(await cardB.getAttribute('aria-describedby')).toBe('card-detail-12');

    // Once B actually loses focus, its inspector closes.
    await cardB.evaluate((el) => el.blur());
    await page.waitForFunction(() => document.querySelectorAll('body > .card-detail').length === 0, null, { timeout: 5_000 });
    await page.close();
  });

  it('a card leaving the ask closes the panel with no pointer event (the lifecycle contract)', async () => {
    const page = await browser.newPage();
    await page.goto(`${url}src/components/PromptSurface.fixture.html?case=seqswap`);

    await page.locator('[data-arrange-row] [data-option="0"]').hover();
    await page.locator('body > .card-detail').waitFor({ state: 'visible', timeout: 5_000 });

    // The ask changes underneath the stationary pointer (the two-tab path);
    // the swap is dispatched programmatically exactly as an SSE view
    // replacement would land. Decision B's options do not carry the card
    // the panel describes, so the panel closes without pointerleave.
    await page.evaluate(() => (document.querySelector('[data-swap-decision]') as HTMLButtonElement).click());
    await page.waitForFunction(() => document.querySelectorAll('body > .card-detail').length === 0, null, { timeout: 5_000 });

    await page.close();
  });
});

describe('the arrange modal preview (fb-20260914T063020Z Job 2)', () => {
  it('without a catalog, hovering a card shows the large art and its name — and no oracle block (the designed degradation)', async () => {
    const page = await browser.newPage();
    await page.goto(`${url}src/components/PromptSurface.fixture.html?case=arrange`);

    await page.locator('[data-arrange-open]').click();
    await page.locator('[data-arrange-keep-card="0"] button.face').hover();
    const preview = page.locator('[data-arrange-preview]');
    await preview.locator('.name').waitFor({ state: 'visible', timeout: 5_000 });
    expect(await preview.locator('.name').textContent()).toBe('Brazen Borrower');
    expect(await preview.locator('[data-arrange-preview-oracle]').count()).toBe(0);

    await page.close();
  });

  it('with a catalog, the preview shows the printed description under the large art', async () => {
    const page = await browser.newPage();
    // A page WITH a catalog: the <meta name="gorge-cards"> tag opts in
    // (empty content = the same-origin catalog — exactly what cmd/gorged
    // injects into its served index), rewritten into the fixture response.
    await page.route('**/PromptSurface.fixture.html*', async (route) => {
      const res = await route.fetch();
      await route.fulfill({ response: res, body: (await res.text()).replace('</head>', '<meta name="gorge-cards" content=""></head>') });
    });
    await page.route('**/cards/named*', (route) =>
      route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({ name: 'Brazen Borrower', oracle_text: 'Flying, ward 2.' }),
      }));

    await page.goto(`${url}src/components/PromptSurface.fixture.html?case=arrange`);
    await page.locator('[data-arrange-open]').click();
    await page.locator('[data-arrange-keep-card="0"] button.face').hover();

    const oracle = page.locator('[data-arrange-preview-oracle]');
    await oracle.waitFor({ state: 'visible', timeout: 5_000 });
    expect(await oracle.textContent()).toContain('Flying, ward 2.');

    await page.close();
  });

  it('clears card A’s description while card B’s delayed lookup is pending, then renders only B’s result', async () => {
    const page = await browser.newPage();
    await page.route('**/PromptSurface.fixture.html*', async (route) => {
      const res = await route.fetch();
      await route.fulfill({ response: res, body: (await res.text()).replace('</head>', '<meta name="gorge-cards" content=""></head>') });
    });

    let notifyBStarted = () => {};
    const bStarted = new Promise<void>((resolve) => { notifyBStarted = resolve; });
    let releaseB = () => {};
    const bReleased = new Promise<void>((resolve) => { releaseB = resolve; });
    await page.route('**/cards/named*', async (route) => {
      const name = new URL(route.request().url()).searchParams.get('exact') ?? '';
      if (name === 'Fabled Pass') {
        notifyBStarted();
        await bReleased;
      }
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({ name, oracle_text: `${name} rules text.` }),
      });
    });

    await page.goto(`${url}src/components/PromptSurface.fixture.html?case=arrange`);
    await page.locator('[data-arrange-open]').click();
    const preview = page.locator('[data-arrange-preview]');
    const oracle = preview.locator('[data-arrange-preview-oracle]');

    // Resolve A first, then move keyboard focus directly to B. B's art/name
    // change immediately while its request remains deliberately unresolved.
    await page.locator('[data-arrange-keep-card="0"] button.face').focus();
    await oracle.waitFor({ state: 'visible', timeout: 5_000 });
    expect(await oracle.textContent()).toBe('Brazen Borrower rules text.');
    await page.locator('[data-arrange-keep-card="1"] button.face').focus();
    await bStarted;
    expect(await preview.locator('.name').textContent()).toBe('Fabled Pass');
    expect(await oracle.count()).toBe(0); // never B art/name with A text

    releaseB();
    await oracle.waitFor({ state: 'visible', timeout: 5_000 });
    expect(await oracle.textContent()).toBe('Fabled Pass rules text.');
    expect(await oracle.textContent()).not.toContain('Brazen Borrower');

    await page.close();
  });
});
