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
 *   obj()  renders a faceless ability      as  "an ability #<id>" (describe.go)
 *   obj()  renders an unresolvable id      as  "#<id>"          (describe.go)
 *   obj()  renders the redacted id 0       as  "a card"         (describe.go)
 *   mana() renders mana                     as  "{R}{G}{2}{W/U}"  etc.
 *
 * So a line is not opaque prose to us — object references are exactly the
 * shapes above, delimited by the `#<digits>` tag, and mana is anything inside
 * `{…}`. That is the structure this module reads. It does NOT guess a card out
 * of a free-word sentence.
 *
 * HOW A CARD NAME IS FOUND (B2, fix round 1). The first round matched card
 * names with a word-shape regex (`[A-Z][\w'-]*(?: … )*`) and it was wrong in
 * three measured ways: a `,` in the `Name, Title` legendary format split the
 * name so only the tail matched ("Jace, the Mind Sculptor" → "Mind Sculptor");
 * a connector like `in` ("Sign in Blood") was outside the allowlist; and a
 * non-ASCII letter fell outside `\w`. That round's card-name regex is now only
 * the no-view fallback. When a card-name key set IS supplied, the name is
 * resolved by a **longest exact match against the view's own card-name key
 * set**, anchored to end immediately before the `#<id>` tag — a card token is
 * recognised because it is literally a name in the view, not because it looks
 * like one. This is why B2 now works for every commander (`Jace, the Mind
 * Sculptor`, `Ghalta, Primal Hunger`, …) and for `Sign in Blood`.
 *
 * WHERE THAT KEY SET COMES FROM. A card's colour lives in its mana_cost; the
 * event carries no colour and the line carries neither cost nor colour
 * identity. So when a match view is available, Table.svelte hands this module
 * the current view's cards through buildCardColour(); the resolver it returns
 * carries both a colour lookup and the exact card-name key set it was built
 * over. The card token and its colour are therefore looked up together, and a
 * name that is NOT in the current view (a card that has left every visible
 * zone) renders uncoloured rather than guessed — a name the view cannot name is
 * not coloured.
 *
 * WHAT THE LINE DOES NOT CARRY (residual gap, stated once): a card's colour is
 * not on the wire; it is looked up from the view's cards. There is no match
 * view at the lobby rail, so feed lines are rendered by the no-view fallback
 * (word-shape regex) with card names uncoloured. A richer source (the server
 * emitting colour identity on the wire) is a protocol change and out of scope.
 *
 * SEAT VS CARD (the false-positive guard). A card token is consumed WHOLE —
 * an exact name match plus its `#<id>` becomes one card piece — before the
 * leftover plain text ever runs through the seat matcher (logcolour.ts). So a
 * seat named `Jace` is never coloured as a player inside `Jace, the Mind
 * Sculptor #4`: the prefix is part of a consumed card token and never reaches
 * seat matching. This holds *whenever the exact-match path is used* (a key set
 * is present). In the no-view fallback the name is matched by word shape and a
 * seat name coinciding with a card-name prefix is left for logcolour.ts's own
 * lookahead guard, which handles the shapes it is documented for — that
 * fallback is the approximation, not this path.
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
 * "card colour" vocabulary) the way a card frame wears its own hue. Two values
 * are adapted for the transcript's DARK instrument ground, not replaced: black
 * (--mana-b is a dark plum that disappears as text on dark) is lifted in
 * lightness while keeping its hue; and gold ('M') is the one hue the palette
 * has no word for — MTG frames multicolour cards gold — so a gold text colour
 * is supplied here. Colourless reuses the stone the pipeline uses for a
 * colourless pip.
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
 * CardColourResolver is the colour lookup a caller hands parseLogLine, built
 * over a set of view cards. It is a function `name -> key` (null for a name
 * not in the view, never a made-up colour), and it additionally carries the
 * exact card-name keys it was built over so parseLogLine can do a longest
 * exact match against them (B2, fix round 1) rather than guess at word shape.
 */
export interface CardColourResolver {
  (name: string): CardColourKey | null;
  /** the exact card-name keys this resolver was built over (the view's card names). */
  names: readonly string[];
}

/**
 * buildCardColour builds a CardColourResolver over the current match view's
 * cards. Keyed by the card's own name and carrying that name set, so a card
 * name in a log line is recognised because it is really in the view. The later
 * card with the same name wins (id order is irrelevant: the colour identity is
 * shared by every copy of a printing). A name absent from the view resolves to
 * null (rendered uncoloured), never to a made-up colour.
 */
export function buildCardColour(cards: readonly { name: string; mana_cost?: string }[]): CardColourResolver {
  const byName = new Map<string, CardColourKey | null>();
  for (const c of cards) {
    if (!c.name) continue;
    byName.set(c.name, cardColourKey(c.mana_cost));
  }
  const resolver = ((name: string) => byName.get(name) ?? null) as CardColourResolver;
  resolver.names = [...byName.keys()];
  return resolver;
}

export interface LogRenderOpts {
  /** seat identities, resolved by the caller the same way the seat rail does. */
  identities?: LogSeatIdentity[];
  /** card name -> colour-key resolver; feed buildCardColour(view cards). */
  cardColour?: CardColourResolver | null;
}

/** isWordChar is the manual half of a word boundary; see logcolour.ts for why it is manual. */
function isWordChar(c: string | undefined): boolean {
  return c !== undefined && /[A-Za-z0-9]/.test(c);
}

/**
 * resolveObjectAt resolves the object reference whose `#<digits>` tag is at
 * `hashIndex` in `line`. The described object is one of:
 *   "<Name> #<id>"   a card / permanent / spell whose name is in the view
 *   "an ability #id" a faceless ability object (minted, no Face)
 *   "#<id>"          an id the game could not resolve — resolved to null here,
 *                    so the caller renders the honest bare id and invents no
 *                    words (F1).
 * With a name key set, the card name is the LONGEST known name that ends
 * immediately before the tag and is preceded by a word boundary, so a comma,
 * a connector like "in", or a non-ASCII letter inside the name cannot split
 * it. Returns the token kind and name (or null for an unresolvable id).
 */
function resolveObjectAt(
  line: string,
  hashIndex: number,
  names: readonly string[],
): { kind: 'ability' } | { kind: 'card'; name: string } | null {
  const before = line.slice(0, hashIndex);
  // The name sits immediately before the "#", separated by whitespace. An
  // ability object is the literal "an ability" phrase (describe.go).
  if (/(^|\s)an ability\s*$/i.test(before)) return { kind: 'ability' };
  const nameText = before.replace(/\s+$/, '');
  let best: string | null = null;
  for (const n of names) {
    if (!n || n.length > nameText.length) continue;
    if (best && n.length <= best.length) continue;
    if (!nameText.endsWith(n)) continue;
    const idx = nameText.length - n.length;
    if (idx > 0 && isWordChar(nameText[idx - 1])) continue; // not a whole name: preceded by a word char
    best = n;
  }
  if (best) return { kind: 'card', name: best };
  return null;
}

/**
 * parseLogLine splits one described line into renderable pieces. See the file
 * header for why the exact-match path is preferred when a key set is present.
 * Two passes, deliberately ordered: pass 1 consumes the OBJECT and MANA
 * tokens whole; pass 2 runs the leftover plain-text runs through the existing
 * seat matcher (colourSegments). Because a card token (its real name plus its
 * `#<id>`) is consumed whole, the seat matcher never sees a seat name that
 * sits inside it.
 */
export function parseLogLine(line: string, opts: LogRenderOpts = {}): LogPiece[] {
  if (!line) return [];
  const { identities = [], cardColour = null } = opts;
  if (cardColour && cardColour.names !== undefined) {
    return parseByExactNames(line, identities, cardColour);
  }
  return parseByObjectShape(line, identities, cardColour);
}

/** parseByExactNames resolves objects against the view's own card-name key set. */
function parseByExactNames(line: string, identities: LogSeatIdentity[], cardColour: CardColourResolver): LogPiece[] {
  const pieces: LogPiece[] = [];
  const pushText = (t: string) => {
    for (const seg of colourSegments(t, identities)) {
      pieces.push(seg.colour ? { kind: 'seat', text: seg.text, colour: seg.colour } : { kind: 'text', text: seg.text });
    }
  };

  const re = /(\{[^{}]+\})|#(\d+)/g;
  let last = 0;
  let m: RegExpExecArray | null;
  while ((m = re.exec(line))) {
    if (m[1] !== undefined) {
      if (m.index > last) pushText(line.slice(last, m.index));
      pieces.push({ kind: 'mana', token: m[1].slice(1, -1) });
      last = re.lastIndex;
      continue;
    }
    // an object id tag: resolve the object it belongs to, and consume the
    // name region so it is not re-emitted as plain text.
    const id = m[2];
    const tok = resolveObjectAt(line, m.index, cardColour.names);
    if (tok === null) {
      if (m.index > last) pushText(line.slice(last, m.index));
      pieces.push({ kind: 'text', text: '#' + id });
    } else {
      const name = tok.kind === 'ability' ? 'an ability' : tok.name;
      // nameStart = the position where the object's name begins: the tag's
      // index minus the whitespace that separates the name from "#" minus
      // the name's own length.
      const trimmedLen = line.slice(0, m.index).replace(/\s+$/, '').length;
      const start = trimmedLen - name.length;
      if (start > last) pushText(line.slice(last, start));
      if (tok.kind === 'ability') {
        pieces.push({ kind: 'ability', name, id });
      } else {
        pieces.push({ kind: 'card', name, id, colour: cardColour(name) });
      }
    }
    last = re.lastIndex;
  }
  if (last < line.length) pushText(line.slice(last));
  return pieces;
}

/** The word-shape fallback, used only when no card-name key set was supplied. */
function parseByObjectShape(
  line: string,
  identities: LogSeatIdentity[],
  cardColour: CardColourResolver | null,
): LogPiece[] {
  const pieces: LogPiece[] = [];
  const pushText = (t: string) => {
    for (const seg of colourSegments(t, identities)) {
      pieces.push(seg.colour ? { kind: 'seat', text: seg.text, colour: seg.colour } : { kind: 'text', text: seg.text });
    }
  };

  const CAP = `\\p{Lu}[\\p{L}\\p{N}'\\-]*`;
  const LOW = `of|the|a|an|to|and|for|in|on|with|from|into|at|by`;
  const NAME = `${CAP}(?:[,\\s]+(?:${CAP}|${LOW}))*`;
  const OBJ = `(?:an ability|${NAME})`;
  const re = new RegExp(`(\\{[^{}]+\\})|(${OBJ})\\s*#(\\d+)|#(\\d+)`, 'gu');

  let last = 0;
  let m: RegExpExecArray | null;
  while ((m = re.exec(line))) {
    if (m.index > last) pushText(line.slice(last, m.index));
    if (m[1] !== undefined) {
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
      // A bare object id describe.go could not resolve: render the honest
      // "#<id>" rather than inventing a name (F1).
      pieces.push({ kind: 'text', text: '#' + m[4] });
    }
    last = re.lastIndex;
  }
  if (last < line.length) pushText(line.slice(last));
  return pieces;
}
