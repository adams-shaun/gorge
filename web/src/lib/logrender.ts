/**
 * logrender.ts turns one already-described transcript line (Transcript.svelte's
 * `e.line`, from view/describe.go) into structured, renderable pieces — the
 * client-side half of the "readable log" task (ui9), extended by lc1 for
 * owner-coloured cards and source-named abilities.
 *
 * WHERE THE WORDS COME FROM. The line text is composed SERVER-SIDE in
 * view/describe.go, not by the client; the client receives one string per
 * event plus the event's own fields (kind, obj, player, …). This task does not
 * move that boundary: nothing in here changes events.Event or any event field
 * (their binary encoding is hash-chained and replayed), and no view-side Go
 * change was needed. Instead this module parses the server's *documented
 * contract* for an object reference and re-renders it:
 *
 *   obj()  renders a card/permanent/spell as  "<Name> #<id>"       (describe.go)
 *   obj()  renders a source-named ability   as  "<Name>'s ability #<id>"
 *   obj()  renders an unresolvable ability  as  "an ability #<id>"  (describe.go)
 *   obj()  renders an unresolvable id       as  "#<id>"           (describe.go)
 *   obj()  renders the redacted id 0        as  "a card"          (describe.go)
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
 * HOW A CARD IS COLOURED (lc1, replaces the B2 mana-identity colouring). A
 * card in the log takes the colour of the player who OWNs it — the same colour
 * that player's name renders in — resolved by the card's **object id**, not by
 * its name or mana identity. Name-keying could never distinguish two seats
 * holding the same card (two Islands owned by different seats would render one
 * colour), which is exactly what owner-colouring must express. The resolver
 * (buildCardOwnerColour) is keyed by object id and maps it to the owner seat's
 * colour, taken from the same seat palette that colours a seat's name in the
 * line; it additionally carries the view's exact card-name keys so the name
 * detection above keeps working. An id absent from the view resolves to null
 * (rendered uncoloured), never to a made-up colour — the B2 promise, carried
 * over to ids.
 *
 * ABILITY TOKENS (lc1). describe.go now renders a faceless ability object as
 * `"<Source>'s ability #<id>"` (e.g. "Goblin Balloon Brigade's ability #217"),
 * degrading to `"an ability #<id>"` when its source cannot be resolved. This
 * module detects the ability token by the `'s ability` possessive before the id
 * (and keeps the older `an ability` shape for the fallback), so a line the
 * server phrases differently is not silently mis-classified as a card.
 *
 * WHAT THE LINE DOES NOT CARRY (residual gap, stated once): a card's colour is
 * not on the wire; it is looked up from the view's cards' owner field. There is
 * no match view at the lobby rail, so feed lines are rendered by the no-view
 * fallback (word-shape regex) with card names uncoloured. A richer source (the
 * server emitting colour identity on the wire) is a protocol change and out of
 * scope.
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

import type { LogSeatIdentity } from './logcolour';
import { colourSegments } from './logcolour';

/**
 * One renderable piece of a described line. The component renders each
 * according to kind; nothing here decides what an event does.
 *
 * A card piece's `colour` is the CSS colour of the seat that OWNS the card
 * (null when the view cannot resolve the card's id — rendered uncoloured,
 * never guessed). An ability piece's `name` is the server's source-speaking
 * possessive ("Goblin Balloon Brigade's ability") or the `an ability` fallback.
 */
export type LogPiece =
  | { kind: 'text'; text: string }
  | { kind: 'mana'; token: string }
  | { kind: 'card'; name: string; id: string; colour: string | null }
  | { kind: 'ability'; name: string; id: string }
  | { kind: 'seat'; text: string; colour: string };

/**
 * CardOwnerColour is the colour lookup a caller hands parseLogLine, built over
 * a set of view cards by object id. It is a function `id -> CSS colour`
 * (null for an id not in the view, never a made-up colour), and it
 * additionally carries the exact card-name keys it was built over so
 * parseLogLine can do a longest exact match against them (B2, fix round 1)
 * rather than guess at word shape.
 */
export interface CardOwnerColour {
  (id: number): string | null;
  /** the exact card-name keys this resolver was built over (the view's card names). */
  names: readonly string[];
}

/**
 * buildCardOwnerColour builds a CardOwnerColour over the current match view's
 * cards. Keyed by the card's OWNER seat — via the caller-supplied
 * `colourForSeat` (the same seat palette that colours a seat's name in the
 * line) — so two copies of the same card owned by different seats resolve to
 * different colours, which is exactly what name-keying could never express
 * (lc1). It also carries the view's exact card-name keys. An id absent from
 * the view resolves to null (rendered uncoloured), never to a made-up colour.
 * The caller is expected to have flattened the view's zones through
 * everyVisibleCard (lib/board.ts), which defends the `hand: null` public-
 * spectator shape; this builder additionally skips a null entry so it is
 * equally safe on its own.
 */
export function buildCardOwnerColour(
  cards: readonly ({ id?: number; name?: string; owner?: number } | null | undefined)[],
  colourForSeat: (owner: number) => string,
): CardOwnerColour {
  const byId = new Map<number, string | null>();
  const byName = new Set<string>();
  for (const c of cards) {
    if (!c || c.id === undefined || c.owner === undefined) continue;
    byId.set(c.id, colourForSeat(c.owner));
    if (c.name) byName.add(c.name);
  }
  const resolver = ((id: number) => byId.get(id) ?? null) as CardOwnerColour;
  resolver.names = [...byName];
  return resolver;
}

export interface LogRenderOpts {
  /** seat identities, resolved by the caller the same way the seat rail does. */
  identities?: LogSeatIdentity[];
  /** object-id -> owner seat colour resolver; feed buildCardOwnerColour(view cards). */
  cardColour?: CardOwnerColour | null;
}

/** isWordChar is the manual half of a word boundary; see logcolour.ts for why it is manual. */
function isWordChar(c: string | undefined): boolean {
  return c !== undefined && /[A-Za-z0-9]/.test(c);
}

// The name-shape for a card's own name, shared by the ability-source
// extraction below and the no-view fallback: a capitalized run of words joined
// by a comma, a space, or a small connector list, so `Jace, the Mind Sculptor`
// and `Sign in Blood` are matched whole. `\p{Lu}` requires the `u` flag.
const NAME_WORD = `\\p{Lu}[\\p{L}\\p{N}'\\-]*`;
const NAME_LOW = `of|the|a|an|to|and|for|in|on|with|from|into|at|by`;
const CARD_NAME = `${NAME_WORD}(?:[,\\s]+(?:${NAME_WORD}|${NAME_LOW}))*`;
// ABILITY_NAME_RE captures a faceless ability's source possessive
// `"<Source>'s ability"` from the text immediately before the `#<id>` tag,
// so a mid-line reference ("Ann copies <Source>'s ability #id") keeps just the
// possessive rather than swallowing the words that precede it.
const ABILITY_NAME_RE = new RegExp(`(${CARD_NAME})'s ability$`, 'u');

/**
 * resolveObjectAt resolves the object reference whose `#<digits>` tag is at
 * `hashIndex` in `line`. The described object is one of:
 *   "<Name> #<id>"         a card / permanent / spell whose name is in the view
 *   "<Name>'s ability #id" a faceless ability object naming its source (lc1)
 *   "an ability #id"       a faceless ability object with an unresolvable source
 *   "#<id>"                an id the game could not resolve — resolved to null
 *                          here, so the caller renders the honest bare id and
 *                          invents no words (F1).
 * With a name key set, a card name is the LONGEST known name that ends
 * immediately before the tag and is preceded by a word boundary, so a comma,
 * a connector like "in", or a non-ASCII letter inside the name cannot split
 * it. Returns the token kind and name (or null for an unresolvable id).
 */
function resolveObjectAt(
  line: string,
  hashIndex: number,
  names: readonly string[],
): { kind: 'ability'; name: string } | { kind: 'card'; name: string } | null {
  const before = line.slice(0, hashIndex);
  const trimmed = before.replace(/\s+$/, '');
  // An ability is the server's possessive "<Source>'s ability" (describe.go,
  // lc1) or the older bare "an ability" fallback. Matched before the name key
  // set so a card name never swallows an ability whose source is not a card
  // in the view. The possessive is extracted by name shape (not taken as the
  // whole prefix) so a mid-line reference keeps just "<Source>'s ability".
  const am = ABILITY_NAME_RE.exec(trimmed);
  if (am) return { kind: 'ability', name: am[1] + "'s ability" };
  if (/^an ability$/i.test(trimmed)) return { kind: 'ability', name: trimmed };
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
function parseByExactNames(line: string, identities: LogSeatIdentity[], cardColour: CardOwnerColour): LogPiece[] {
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
      const name = tok.name;
      // nameStart = the position where the object's name begins: the tag's
      // index minus the whitespace that separates the name from "#" minus
      // the name's own length.
      const trimmedLen = line.slice(0, m.index).replace(/\s+$/, '').length;
      const start = trimmedLen - name.length;
      if (start > last) pushText(line.slice(last, start));
      if (tok.kind === 'ability') {
        pieces.push({ kind: 'ability', name, id });
      } else {
        pieces.push({ kind: 'card', name, id, colour: cardColour(Number(id)) });
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
  cardColour: CardOwnerColour | null,
): LogPiece[] {
  const pieces: LogPiece[] = [];
  const pushText = (t: string) => {
    for (const seg of colourSegments(t, identities)) {
      pieces.push(seg.colour ? { kind: 'seat', text: seg.text, colour: seg.colour } : { kind: 'text', text: seg.text });
    }
  };

  const OBJ = `(?:${CARD_NAME}|an ability)(?:'s ability)?`;
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
      if (/'s ability$/i.test(name) || /^an ability$/i.test(name)) {
        pieces.push({ kind: 'ability', name, id });
      } else {
        pieces.push({ kind: 'card', name, id, colour: cardColour ? cardColour(Number(id)) : null });
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
