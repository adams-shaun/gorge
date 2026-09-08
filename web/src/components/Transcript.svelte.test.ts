import { describe, expect, it } from 'vitest';
import { render } from 'svelte/server';
import type { DvrState } from '../lib/dvr';
import { buildCardColour } from '../lib/logrender';
import Transcript from './Transcript.svelte';

/**
 * Transcript render test (ui9 fix round 1, finding F5). The repo's component
 * tests SSR via svelte/server (no DOM); the interaction under test here is the
 * render-time contract: the logrender pieces become readable text. SSR cannot
 * click the toggle, so the step-line test asserts the default (hidden); the
 * toggle wiring is logfilter's own test's job (logfilter.test.ts).
 */

// The same view-of-cards shape Table.svelte feeds buildCardColour: it carries
// the exact card-name keys, so a comma name and a non-ASCII name resolve whole.
const cardColour = buildCardColour([
  { name: 'Jace, the Mind Sculptor', mana_cost: '2 U U' },
  { name: 'Æther Vial', mana_cost: '1' },
  { name: 'Lightning Bolt', mana_cost: 'R' },
  { name: 'Island', mana_cost: undefined },
]);

const identities = [{ name: 'Ann', colour: '#e5484d' }];

const ev = (seq: number, kind: string, line: string) => ({ event: { seq, kind, player: -1 }, line });

const dvr = (events: ReturnType<typeof ev>[]): DvrState => ({
  match: 'm1', head: events.length, cursor: events.length, live: true, events, turnStarts: [], gap: false,
});

const renderLog = (d: DvrState) => render(Transcript, {
  props: { dvr: d, onSeek: () => {}, identities, cardColour },
}).html;

describe('Transcript — readable log rendering', () => {
  it('round-trips a comma card name whole, with the id on the hover title', () => {
    const html = renderLog(dvr([ev(1, 'put_on_stack', 'Ann casts Jace, the Mind Sculptor #4')]));
    // the comma name is rendered whole, not split to "Mind Sculptor"
    expect(html).toContain('Jace, the Mind Sculptor');
    // the id is off the visible text and on the hover title (B3)
    expect(html).toContain('title="Jace, the Mind Sculptor #4"');
    expect(html).not.toMatch(/Mind Sculptor #4\b(?!")/); // no bare "#4" in the visible line
    // the visible seq is the event seq, and the card carries a colour
    expect(html).toMatch(/data-seq="1"/);
    expect(html).toContain('color:');
  });

  it('renders a non-ASCII card name whole', () => {
    const html = renderLog(dvr([ev(1, 'move_zone', 'Ann plays Æther Vial #8')]));
    expect(html).toContain('Æther Vial');
    expect(html).toContain('title="Æther Vial #8"');
  });

  it('renders mana symbols as pips, not the {G} braces', () => {
    const html = renderLog(dvr([ev(1, 'mana_add', 'Ann adds {G}{G}')]));
    expect(html).toContain('mana-symbols');
    expect(html).toContain('p-g');
    expect(html).not.toContain('{G}');
  });

  it('suppresses step lines by default (the step filter)', () => {
    const html = renderLog(dvr([
      ev(1, 'move_zone', 'Ann plays Island #2'),
      ev(2, 'step', 'Step: main-1'),
    ]));
    expect(html).toContain('Island');
    // the step event's whole row is hidden: its seq is not rendered at all
    expect(html).not.toMatch(/data-seq="2"/);
    // and exactly one log row is rendered (the Island line)
    expect(html.match(/class="line /g)?.length).toBe(1);
    // (the step text does appear in the toggle's title, which documents the
    // filter; it must not appear as a rendered log row)
  });
});
