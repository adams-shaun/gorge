/**
 * logrender.ts turns one already-described transcript line (Transcript.svelte's
 * `e.line`, from view/describe.go) into structured, renderable pieces — the
 * client-side half of the "readable log" task (ui9).
 *
 * WHERE THE WORDS COME FROM. The line text is composed SERVER-SIDE in
 * view/describe.go, not by the client; the client receives one string per
 * event plus the event's own fields (kind, obj, player, …). This task does not
 * move that boundary: nothing in here changes events.Event or any event field
 * (their binary encoding is hash-chained and replayed), and no view-side Go
 * change was needed. Instead this module parses the server's *documented
 * contract* for an object reference and re-renders it:
 *
 *   obj()  renders a card/permanent/spell as  "<Name> #<id>"   (describe.go)
 *   obj()  renders a faceless ability      as  "an ability #<id>"
 *   mana() renders mana                     as  "{R}{G}{2}{W/U}"  etc.
 *
 * So a line is not opaque prose to us — object references are exactly the two
 * shapes above, delimited by the `#<digits>` tag, and mana is anything inside
 * `{…}`. That is the structure this module reads. It does NOT guess a card out
 * of a free-word sentence.
 *
 * WHAT THE LINE DOES NOT CARRY (B2 finding, stated once):
 *   - The line carries an object's NAME and id, but NOT its mana-colour
 *     identity. A card's colour lives in its mana_cost, which the client has
 *     only on the CardView objects in the current match view — not in the
 *     event, not in the line. So a faithful colour-by-identity must be looked
 *     up from the view's cards via buildCardColour(); a name that is not in
 *     the current view (a card that has left every visible zone) renders
 *     uncoloured rather than guessed. A richer source (the server emitting
 *     colour identity on the wire) is a protocol change, out of scope; this is
 *     reported as the residual gap.
 *   - "an ability #<id>" is the ONLY ability shape describe.go emits (a
 *     minted ability object has no Face), so an ability is recognised exactly
 *     there. Cards with the word "ability" elsewhere in a sentence are
 *     untouched.
 *
 * The undecorated pieces that carry no meaning (plain text, a seat name) are
 * produced with the same two helpers the transcript already uses, so the
 * existing seat-colour work (logcolour.ts) is preserved verbatim rather than
 * replaced, and a seat name that really is the head of a card's name is
 * protected by the card token being consumed whole BEFORE seat matching ever
 * runs (see B2).
 */

import { manaSymbols } from './mana';
import { colourSegments, type LogSeatIdentity } from './logcolour';

/**
 * A card's colour identity, as a colour key the renderer maps to the design
 * system's mana-pip palette. This reuses the one established "card colour"
 * vocabulary in the client — ManaSymbols.svelte's own design note says "a
 * saturated colour means mana, card colour or seat identity" — rather than a
 * second scheme.
 *
 *   'W'|'U'|'B'|'R'|'G'  a mono-colour card
 *   'C'                  colourless / artifact / land (no coloured pips)
 *   'M'                  multicolour (more than one colour, "gold")
 */
export type CardColourKey = 'W' | 'U' | 'B' | 'R' | 'G' | 'C' | 'M';

/**
 * One renderable piece of a described line. The component renders each
 * according to kind; nothing here decides what an event does.
 */
export type LogPiece =
  | { kind: 'text'; text: string }
  | { kind: 'mana'; token: string }
  | { kind: 'card'; name: string; id: string; colour: CardColourKey | null }
  | { kind: 'ability'; name: string; id: string }
  | { kind: 'seat'; text: string; colour: string };

/**
 * CARD_COLOUR_VAR maps a colour key to the CSS colour a card name is rendered
 * in. Mono colours reuse the mana-pip palette (the design system's own
 * "card colour" vocabulary: "a saturated colour means mana, card colour or
 * seat identity") the way a card frame wears its own hue. Two values are
 * adapted for the transcript's DARK instrument ground, not replaced: black
 * (--mana-b is a dark plum that disappears as text on dark) is lifted in
 * lightness while keeping its hue, exactly as ManaSymbols inverts the glyph
 * onto the black disc rather than leaving a letter unreadable; and gold ('M')
 * is the one hue the palette has no word for — MTG frames multicolour cards
 * gold — so a gold text colour is supplied here. Colourless reuses the stone
 * the pipeline uses for a colourless pip.
 */
export const CARD_COLOUR_VAR: Record<CardColourKey, string> = {
  W: 'var(--mana-w)',
  U: 'var(--mana-u)',
  B: '#9c89bd',
  R: 'var(--mana-r)',
  G: 'var(--mana-g)',
  C: 'var(--mana-c)',
  M: '#b49a4b',
};

/** cardColourVar returns the CSS colour a card with the given key renders in, or null for uncoloured. */
export function cardColourVar(key: CardColourKey | null): string | null {
  return key === null ? null : CARD_COLOUR_VAR[key];
}

/**
 * cardColourKey classifies a Forge mana cost string into a card's colour
 * identity (B2). It walks the SAME structured pip list ManaSymbols and the
 * corpus ratchet use, so a shape the pip renderer understands is classified
 * identically here: one distinct colour letter -> that colour; several ->
 * multicolour; none (lands, artifacts, colourless costs, the "no cost"
 * marker) -> colourless. Unknown pips are ignored rather than guessed.
 */
export function cardColourKey(cost: string | undefined): CardColourKey | null {
  if (!cost) return 'C'; // a card with no mana cost reads as colourless
  const colours = new Set<string>();
  for (const s of manaSymbols(cost)) {
    switch (s.kind) {
      case 'colour': colours.add(s.colour); break;
      case 'hybrid':
      case 'phyrexianHybrid':
        if (s.a !== 'C') colours.add(s.a);
        if (s.b !== 'C') colours.add(s.b);
        break;
      case 'twobrid':
      case 'phyrexian': colours.add(s.colour); break;
      case 'colourless':
      case 'generic':
      case 'variable':
      case 'snow':
      case 'unknown': break;
    }
  }
  if (colours.size === 1) return [...colours][0] as CardColourKey;
  if (colours.size > 1) return 'M';
  return 'C';
}

/**
 * buildCardColour returns a name -> colour-key resolver over the current
 * match view's cards (B2). It is built fresh per render from the cards the
 * view currently exposes, keyed by the card's own name, so a card name in a
 * log line is looked up without guessing. The later card with the same name
 * wins (id order is irrelevant: the colour identity is shared by every copy
 * of a printing). A name absent from the view resolves to null (rendered
 * uncoloured), never to a made-up colour.
 */
export function buildCardColour(cards: readonly { name: string; mana_cost?: string }[]): (name: string) => CardColourKey | null {
  const byName = new Map<string, CardColourKey | null>();
  for (const c of cards) {
    if (!c.name) continue;
    byName.set(c.name, cardColourKey(c.mana_cost));
  }
  return (name) => byName.get(name) ?? null;
}

/** The mana-token and object-reference shapes describe.go emits (see the header). */
const OBJ = '(?:an ability|[A-Z][\\w\'-]*(?:\\s+(?:[A-Z][\\w\'-]*|of|the|a|an|to|and|for))*)';

export interface LogRenderOpts {
  /** seat identities, resolved by the caller the same way the seat rail does. */
  identities?: LogSeatIdentity[];
  /** card name -> colour-key resolver; feed buildCardColour(view cards). */
  cardColour?: ((name: string) => CardColourKey | null) | null;
}

/**
 * parseLogLine splits one described line into renderable pieces.
 *
 * Two passes, deliberately ordered so a card name can never be mistaken for a
 * seat name. Pass 1 walks the line for the OBJECT and MANA tokens — the
 * structured shapes above — consuming them whole. Pass 2 runs the leftover
 * plain-text runs through the existing colourSegments seat matcher, so a seat
 * name is coloured only where it actually reads as a player. Because the card
 * token ("Storm Cauldron #12") is consumed by pass 1, the seat matcher never
 * sees "Storm" inside it, so the existing false-positive guard is preserved by
 * construction rather than by lookahead.
 */
export function parseLogLine(line: string, opts: LogRenderOpts = {}): LogPiece[] {
  if (!line) return [];
  const { identities = [], cardColour = null } = opts;
  const re = new RegExp(`(\\{[^{}]+\\})|(${OBJ})\\s*#(\\d+)|#(\\d+)`, 'g');
  const pieces: LogPiece[] = [];

  const pushText = (t: string) => {
    for (const seg of colourSegments(t, identities)) {
      pieces.push(seg.colour ? { kind: 'seat', text: seg.text, colour: seg.colour } : { kind: 'text', text: seg.text });
    }
  };

  let last = 0;
  let m: RegExpExecArray | null;
  while ((m = re.exec(line))) {
    if (m.index > last) pushText(line.slice(last, m.index));
    if (m[1] !== undefined) {
      // a mana symbol: strip the braces, hand the inner token to the pip renderer.
      pieces.push({ kind: 'mana', token: m[1].slice(1, -1) });
    } else if (m[2] !== undefined) {
      const name = m[2].trimEnd();
      const id = m[3];
      if (/^an ability$/i.test(name)) {
        pieces.push({ kind: 'ability', name, id });
      } else {
        pieces.push({ kind: 'card', name, id, colour: cardColour ? cardColour(name) : null });
      }
    } else if (m[4] !== undefined) {
      // a bare object id (an object describe.go cannot name): still an id, so
      // it gets the same hover-only treatment rather than raw "#12" noise.
      pieces.push({ kind: 'ability', name: 'an ability', id: m[4] });
    }
    last = re.lastIndex;
  }
  if (last < line.length) pushText(line.slice(last));
  return pieces;
}
