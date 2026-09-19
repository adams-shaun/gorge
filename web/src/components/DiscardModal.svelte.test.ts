import { describe, expect, it } from 'vitest';
import { render } from 'svelte/server';
import type { Decision, Option } from '../protocol';
import DiscardModal from './DiscardModal.svelte';

// SSR via svelte/server, the repo's component-test pattern (the ArrangeModal
// harness): the portalled backdrop and dialog render as markup; the pointer
// interactions (hover preview switching, click-to-pick) are wired to the
// CALLER's decision logic in SeatPanel and pinned there in
// lib/seatpanel.discard.test.ts — this file pins the markup contract: the
// modal guard markers, the faces, the picked badges, the submit gate and its
// label wording, and the hover aside's oracle block.

const pick = (index: number, name: string, obj: number): Option =>
  ({ index, kind: 'discard', label: `Discard ${name}`, obj, player: 0 });

/** A Thoughtseize-shaped ask: KModes, Min == Max == 1. */
const thoughtseize: Decision = {
  seq: 7, player: 0, kind: 'modes', min: 1, max: 1, prompt: 'Choose 1 card(s) to discard',
  options: [
    pick(0, 'Mother of Runes', 21),
    pick(1, 'Brainstorm', 22),
    pick(2, 'Force of Will', 23),
  ],
};

/** A cleanup-step-shaped ask: KChoose, 2 of 3. */
const cleanup: Decision = {
  ...thoughtseize, kind: 'choose', min: 2, max: 2,
  prompt: 'discard 2 card(s) down to the hand-size limit',
};

describe('DiscardModal — the discard-pick ask’s "Open the card view" popup', () => {
  it('closed, it renders nothing (the surface is portalled on open)', () => {
    const { html } = render(DiscardModal, { props: { open: false, decision: thoughtseize, onPick: () => {}, onSubmit: () => {}, onClose: () => {} } });
    expect(html).not.toContain('data-discard-modal');
  });

  it('open, it is a guarded dialog carrying the prompt and the card faces', () => {
    const { html } = render(DiscardModal, { props: { open: true, decision: thoughtseize, onPick: () => {}, onSubmit: () => {}, onClose: () => {} } });
    // The modal guard: the structural markers lib/hotkeys.ts'
    // MODAL_PICKER_SELECTOR recognises (role="dialog" + aria-modal), so the
    // document hotkeys cannot act through this surface; the data markers are
    // the stable compatibility hooks (data-pile-modal as ArrangeModal's).
    expect(html).toContain('data-discard-modal');
    expect(html).toContain('role="dialog"');
    expect(html).toContain('aria-modal="true"');
    expect(html).toContain('data-pile-modal');
    expect(html).toContain('Choose 1 card(s) to discard');
    // Every offered option renders as a real face, in offered order.
    expect(html).toContain('data-discard-grid');
    expect(html).toMatch(/data-discard-card="0"/);
    expect(html).toMatch(/data-discard-card="1"/);
    expect(html).toMatch(/data-discard-card="2"/);
    expect(html).toMatch(/aria-label="Discard Force of Will"/);
  });

  it('picked faces carry the shared picked order badge and the pressed state', () => {
    const { html } = render(DiscardModal, {
      props: { open: true, decision: cleanup, picked: [2, 0], onPick: () => {}, onSubmit: () => {}, onClose: () => {} },
    });
    expect(html).toMatch(/data-discard-card="2"[^]*?class="order[^"]*"[^]*?>1</);
    expect(html).toMatch(/data-discard-card="0"[^]*?class="order[^"]*"[^]*?>2</);
    // The two pressed faces, and only those.
    expect(html.match(/aria-pressed="true"/g)).toHaveLength(2);
    expect(html).toContain('aria-pressed="false"');
  });

  it('the submit renders with the inline row’s own label wording and gate', () => {
    const label = (d: Decision, canSubmit: boolean) =>
      render(DiscardModal, { props: { open: true, decision: d, showSubmit: true, canSubmit, onPick: () => {}, onSubmit: () => {}, onClose: () => {} } }).html;
    // A range ask: "Choose 2–2" is the min==max wording the inline row uses;
    // 2 of 3 here is the min<max range wording.
    expect(label(cleanup, true)).toMatch(/data-discard-submit[^>]*>Choose 2</);
    const scryish: Decision = { ...cleanup, min: 1, max: 2 };
    expect(label(scryish, true)).toMatch(/data-discard-submit[^>]*>Choose 1–2</);
    const optional: Decision = { ...cleanup, min: 0, max: 2 };
    expect(label(optional, true)).toMatch(/data-discard-submit[^>]*>Confirm</);
    // The gate: short of the min, the submit is disabled.
    expect(label(cleanup, false)).toMatch(/data-discard-submit[^>]*disabled/);
  });

  it('a Min==Max==1 ask renders NO submit (showSubmit mirrors the inline row): the click IS the answer', () => {
    const { html } = render(DiscardModal, {
      props: { open: true, decision: thoughtseize, showSubmit: false, onPick: () => {}, onSubmit: () => {}, onClose: () => {} },
    });
    expect(html).toContain('data-discard-modal');
    expect(html).not.toContain('data-discard-submit');
  });

  // fb-20260914T120705Z's requirement, carried over: "mouseover oracle help
  // popup like every other view" — the aside resolves the hovered card's
  // printed description by name through lib/oracle.
  describe('the preview aside', () => {
    const props0 = (over: Record<string, unknown> = {}) => ({
      open: true, decision: thoughtseize, onPick: () => {}, onSubmit: () => {}, onClose: () => {}, ...over,
    });

    it('shows the printed description under the art when the catalog resolves it (a sync resolver seeds the server render)', () => {
      const { html } = render(DiscardModal, {
        props: props0({ preview0: 1, resolver: (name: string) => ({ name, oracle_text: `${name} is a blue instant.` }) }),
      });
      expect(html).toContain('data-discard-preview-oracle');
      expect(html).toContain('Brainstorm is a blue instant.');
      expect(html).toContain('data-discard-preview');
      expect(html).toMatch(/<p class="name[^"]*"[^>]*>Discard Brainstorm<\/p>/);
    });

    it('renders nothing for the oracle block when the resolver answers null — the designed no-catalog degradation', () => {
      const { html } = render(DiscardModal, { props: props0({ preview0: 1, resolver: () => null }) });
      expect(html).not.toContain('data-discard-preview-oracle');
      expect(html).toContain('data-discard-preview');
      expect(html).toContain('data-discard-modal');
    });

    it('the production default (no catalog in the test env) resolves nothing and the hint aside renders', () => {
      const { html } = render(DiscardModal, { props: props0() });
      expect(html).toContain('Hover a card for full art.');
      expect(html).not.toContain('data-discard-preview-oracle');
    });
  });
});
