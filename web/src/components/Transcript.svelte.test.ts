import { describe, expect, it } from 'vitest';
import { render } from 'svelte/server';
import type { DvrState } from '../lib/dvr';
import { buildCardOwnerColour } from '../lib/logrender';
import { everyVisibleCard } from '../lib/board';
import type { PlayerView } from '../protocol';
import Transcript from './Transcript.svelte';

/**
 * Transcript render test (ui9 fix round 1, finding F5). The repo's component
 * tests SSR via svelte/server (no DOM); the interaction under test here is the
 * render-time contract: the logrender pieces become readable text. SSR cannot
 * click the toggle, so the step-line test asserts the default (hidden); the
 * toggle wiring is logfilter's own test's job (logfilter.test.ts).
 *
 * lc1 extends this for the owner-colour resolution: a card in the log takes
 * the colour of the seat that OWNS it (by object id), and a faceless ability
 * is named by its source card.
 */

// fXC is the SEAT_COLOURS palette (lib/colours.ts); a card owned by seat N
// renders the same colour as that seat's name in the line.
const PALETTE = ['#e5484d', '#3b82f6', '#22c55e', '#eab308'] as const;
const seatColourOf = (seat: number): string => PALETTE[seat] ?? '#777777';

// The same view-of-cards shape Table.svelte feeds buildCardOwnerColour: keyed
// by object id -> owner seat's colour, and it carries the exact card-name keys
// so a comma name and a non-ASCII name resolve whole.
const cardColour = buildCardOwnerColour([
  { id: 11, name: 'Island', owner: 0 },          // Ann
  { id: 12, name: 'Island', owner: 1 },          // Bob
  { id: 4, name: 'Jace, the Mind Sculptor', owner: 0 },
  { id: 8, name: 'Æther Vial', owner: 1 },
  { id: 30, name: 'Lightning Bolt', owner: 1 },
], seatColourOf);

const identities = [
  { name: 'Ann', colour: '#e5484d' },
  { name: 'Bob', colour: '#3b82f6' },
];

const ev = (seq: number, kind: string, line: string) => ({ event: { seq, kind, player: -1 }, line });

const dvr = (events: ReturnType<typeof ev>[]): DvrState => ({
  match: 'm1', head: events.length, cursor: events.length, live: true, events, turnStarts: [], gap: false,
});

const renderLog = (d: DvrState, colours = cardColour, ids = identities) => render(Transcript, {
  props: { dvr: d, onSeek: () => {}, identities: ids, cardColour: colours },
}).html;

describe('Transcript — readable log rendering', () => {
  it('round-trips a comma card name whole, with the id on the hover title', () => {
    const html = renderLog(dvr([ev(1, 'put_on_stack', 'Ann casts Jace, the Mind Sculptor #4')]));
    // the comma name is rendered whole, not split to "Mind Sculptor"
    expect(html).toContain('Jace, the Mind Sculptor');
    // the id is off the visible text and on the hover title (B3)
    expect(html).toContain('title="Jace, the Mind Sculptor #4"');
    expect(html).not.toMatch(/Mind Sculptor #4\b(?!")/); // no bare "#4" in the visible line
    // the visible seq is the event seq, and the card carries its owner's colour
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
});

describe('Transcript — owner-coloured cards (lc1)', () => {
  it('two seats owning the same card name render in two different colours in one transcript', () => {
    // The case name-keying could never express: two Islands owned by different
    // seats must render differently.
    const html = renderLog(dvr([
      ev(1, 'land_played', 'Ann plays Island #11'),
      ev(2, 'land_played', 'Bob plays Island #12'),
    ]));
    expect(html).toContain('Island');
    expect(html).toContain('color: #e5484d'); // Ann's seat colour
    expect(html).toContain('color: #3b82f6'); // Bob's seat colour
  });

  it("a card's colour equals its owner's seat-name colour in the same line", () => {
    const html = renderLog(dvr([ev(1, 'land_played', 'Ann plays Island #11')]));
    // Ann's name and Ann's Island share the same seat colour
    expect(html).toContain('color: #e5484d');
    // both the seat-name span and the card span carry it, so the two agree
    expect(html.match(/color: #e5484d/g)?.length).toBeGreaterThanOrEqual(2);
  });

  it('an id absent from the view renders uncoloured and does not crash', () => {
    // id 999 is not in the view's id map, so its colour resolves null (B2's
    // "never a made-up colour" now on ids). The line must still render.
    const html = renderLog(dvr([ev(1, 'put_on_stack', 'Bob casts Lightning Bolt #999')]));
    expect(html).toContain('Lightning Bolt');
    // uncoloured: the Lightning Bolt card span carries no style colour, and no
    // crash (the seat name 'Bob' still renders in its own colour)
    expect(html).toContain('title="Lightning Bolt #999">Lightning Bolt');
  });

  it('a public spectator null hand does not break the id->owner map (everyVisibleCard)', () => {
    // everyVisibleCard (lib/board.ts) exists because p.hand is a JSON null for
    // a public spectator; the id->owner map must be built through it.
    const players = [
      { seat: 0, hand: null, battlefield: [{ id: 20, owner: 0, name: 'Grizzly Bears' }], graveyard: [], exile: [], command: [], commanders: [] },
    ] as unknown as PlayerView[];
    const cards = everyVisibleCard(players);
    expect(cards.map((c) => c.id)).toEqual([20]);
    const colours = buildCardOwnerColour(cards, seatColourOf);
    const html = renderLog(dvr([ev(1, 'land_played', 'Ann plays Grizzly Bears #20')]), colours);
    expect(html).toContain('Grizzly Bears');
    expect(html).toContain('color: #e5484d');
  });
});

describe('Transcript — source-named abilities (lc1)', () => {
  it("renders \"<source>'s ability #<id>\" as an ability, id on the hover title, no invented words", () => {
    const html = renderLog(dvr([ev(1, 'resolve', "Goblin Balloon Brigade's ability #217 resolves")]));
    // the source possessive is kept, and the id is on the hover title (not the
    // visible text), exactly as the server now phrases it
    expect(html).toContain("Goblin Balloon Brigade's ability");
    expect(html).toContain('title="Goblin Balloon Brigade\'s ability #217"');
    expect(html).not.toMatch(/class="obj card/); // it is an ability, never a card
    // the parser must not have invented the old "an ability" phrasing
    expect(html).not.toContain('an ability');
  });
});

describe('Transcript — step-line default (ui9)', () => {
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
