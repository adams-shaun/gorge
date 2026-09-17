import { describe, expect, it } from 'vitest';
import { render } from 'svelte/server';
import type { CardView } from '../protocol';
import type { AnchorRect } from '../lib/carddetail.svelte';
import CardDetail from './CardDetail.svelte';

// SSR render of the object inspector, same deterministic harness as
// CardTile.svelte.test.ts. This file pins the loyalty ledger row: the
// walker's current loyalty is its own row (CR 306.5b), pulled out of the
// generic counter chips so it never prints twice.

const anchor: AnchorRect = { left: 100, top: 100, right: 190 };

const card = (over: Partial<CardView> = {}): CardView => ({
  id: 21, name: 'Jace, the Mind Sculptor', types: 'Legendary Planeswalker Jace', mana_cost: '2 U U',
  printing: { name: 'Jace, the Mind Sculptor' }, token: '', tapped: false, power: 0, toughness: 0,
  damage: 0, attacking: false, controller: 0, owner: 0, summon_sick: false,
  ...over,
});

describe('CardDetail loyalty ledger row', () => {
  it('a walker prints a Loyalty row and no generic LOYALTY counter chip', () => {
    const { html } = render(CardDetail, { props: { card: card({ counters: { LOYALTY: 3 } }), anchor } });
    expect(html).toContain('Loyalty</dt>');
    expect(html.match(/LOYALTY/g) ?? []).toHaveLength(0);
  });

  it('a non-walker prints no Loyalty row', () => {
    const { html } = render(CardDetail, { props: { card: card({ types: 'Creature Bear', counters: { LOYALTY: 3 } }), anchor } });
    expect(html).not.toContain('Loyalty</dt>');
  });

  it('a walker with no loyalty counter yet prints no Loyalty row', () => {
    const { html } = render(CardDetail, { props: { card: card(), anchor } });
    expect(html).not.toContain('Loyalty</dt>');
  });
});

describe('CardDetail summoning sick state chip (fb-20260917T004545Z)', () => {
  it('a noncreature land with summon_sick prints no summoning sick chip', () => {
    // summoning sickness is a creature fact (CR 302.6): a sick land still taps
    // for mana, so the chip made a basic land read as unable to act
    const { html } = render(CardDetail, { props: { card: card({ types: 'Basic Land Island', summon_sick: true }), anchor } });
    expect(html).not.toContain('summoning sick');
  });

  it('a sick creature still prints the chip', () => {
    const { html } = render(CardDetail, { props: { card: card({ types: 'Creature Bear', summon_sick: true }), anchor } });
    expect(html).toContain('summoning sick');
  });
});
