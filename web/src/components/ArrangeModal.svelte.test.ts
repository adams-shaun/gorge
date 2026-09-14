import { describe, expect, it } from 'vitest';
import { render } from 'svelte/server';
import type { Decision, Option } from '../protocol';
import ArrangeModal from './ArrangeModal.svelte';

// SSR via svelte/server, the repo's component-test pattern: the portalled
// backdrop and dialog render as markup; the interactions (click-to-place,
// drag reorder, submit) are driven by the browser gate in
// PromptSurface.svelte.test.ts, and the arrangement math is pinned pure in
// lib/arrange.test.ts.

const opt = (index: number, name: string, obj: number, kind = 'bottom'): Option =>
  ({ index, kind, label: name, obj, player: 0 });
const reorder: Decision = {
  seq: 7, player: 0, kind: 'arrange', min: 5, max: 5,
  prompt: 'Rearrange the top 5 card(s); the first card you pick goes on top',
  source: 9,
  options: [
    opt(0, 'Brazen Borrower', 11),
    opt(1, 'Fabled Pass', 12),
    opt(2, 'Gitaxian Probe', 13),
    opt(3, 'Spell Pierce', 14),
    opt(4, 'Unholy Heat', 15),
  ],
};
const scry: Decision = {
  ...reorder, min: 0,
  prompt: 'Scry 5: pick the cards to keep on top, in order; the rest go to the bottom of your library',
};

describe('ArrangeModal — the card-face popup for a KArrange ask', () => {
  it('closed, it renders nothing (the surface is portalled on open)', () => {
    const { html } = render(ArrangeModal, { props: { open: false, decision: reorder, onSubmit: () => {}, onClose: () => {} } });
    expect(html).not.toContain('data-arrange-modal');
  });

  it('open, it is a guarded dialog carrying the prompt, the keep row and the submit', () => {
    const { html } = render(ArrangeModal, {
      props: { open: true, decision: reorder, seed: [3, 1, 0, 2, 4], onSubmit: () => {}, onClose: () => {} },
    });
    expect(html).toContain('data-arrange-modal');
    expect(html).toContain('role="dialog"');
    expect(html).toContain('aria-modal="true"');
    expect(html).toContain('Rearrange the top 5 card(s)');
    // the keep pile, in seed order, with its ordinals
    expect(html).toContain('data-arrange-keep');
    expect(html).toContain('data-arrange-keep-card="3"');
    expect(html).toMatch(/aria-label="1: Spell Pierce"/);
    // a pure reorder hides the pool row: every card must stay
    expect(html).not.toContain('data-arrange-pool-card');
    expect(html).toContain('data-arrange-submit');
  });

  it('a scry (Min 0) shows the pool row and names its destination', () => {
    const { html } = render(ArrangeModal, { props: { open: true, decision: scry, seed: [], onSubmit: () => {}, onClose: () => {} } });
    expect(html).toContain('data-arrange-pool');
    expect(html).toContain('the bottom of the library');
    expect([...html.matchAll(/data-arrange-pool-card="\d+"/g)]).toHaveLength(5);
    expect(html).toContain('Nothing kept');
  });

  it('the submit is disabled while the keep pile violates the decision’s min/max', () => {
    const closed = render(ArrangeModal, { props: { open: true, decision: reorder, seed: [], onSubmit: () => {}, onClose: () => {} } }).html;
    expect(closed).toMatch(/data-arrange-submit[^>]*disabled/);
    const open5 = render(ArrangeModal, { props: { open: true, decision: reorder, seed: [0, 1, 2, 3, 4], onSubmit: () => {}, onClose: () => {} } }).html;
    expect(open5).not.toMatch(/data-arrange-submit[^>]*disabled/);
  });
});
