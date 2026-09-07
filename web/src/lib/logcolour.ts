/**
 * logcolour.ts splits one already-described transcript line (Transcript.
 * svelte's `e.line`, from view/describe.go) into plain-text and
 * seat-coloured runs, so a seat's own identity colour (the same one
 * SeatTable/IdentityBar use) marks their name wherever it appears in the
 * log (Task 3, "each log should note the player name in colour"). This is
 * client-side text matching on purpose — no wire field, no Go change — so
 * it has to guard against a false match on its own: a card's own name can
 * contain a seat's display name as a whole word ("Storm Cauldron" beside a
 * seat literally named "Storm").
 *
 * The guard leans on a real, stable convention in view/describe.go's own
 * doc comment: every OBJECT reference in a described line is rendered by
 * `obj()` as "<name> #<id>" — a card or ability name immediately suffixed
 * with its id — while a PLAYER reference (`player()`) never carries that
 * suffix. So a seat-name match is treated as part of a card's name, not the
 * player, exactly when it is immediately followed by a run of further
 * name-shaped words leading straight into a "#<digits>" tag with nothing
 * else in between — that shape only occurs inside an object token. A real
 * player reference is never followed by that shape: the next word in every
 * describe.go template is a lowercase verb ("casts", "attacks", "has
 * priority", ...), which the lookahead does not accept, so "Storm casts
 * Lightning Bolt #12" still colours "Storm" while "Storm Cauldron #12"
 * does not colour "Storm" at all.
 *
 * Residual gap, named rather than silently accepted: a card name whose
 * matching word is joined by a lowercase word outside the small connector
 * list below ("of", "the", "a", "an", "to", "and", "for" — the joiners real
 * Magic card names actually use) is not guarded. No name in the corpus has
 * been observed to trigger this.
 */

export interface LogSeatIdentity {
  name: string;
  colour: string;
}

export interface LogSegment {
  text: string;
  /** the seat colour to render this run in, or null for plain text */
  colour: string | null;
}

const OBJ_TAIL = /^(?:\s(?:[A-Z][\w'-]*|of|the|a|an|to|and|for))*\s#\d+/;

function escapeRe(s: string): string {
  return s.replace(/[.*+?^${}()|[\]\\]/g, '\\$&');
}

/**
 * isWordChar is the manual half of the boundary check: a seat's display
 * name can itself contain punctuation ("P1 (west)"), and JS's `\b` is only
 * defined at a \w/\W transition — it cannot anchor at all when a name
 * legitimately starts or ends on a non-word character, so the boundary is
 * checked by hand instead: the character just outside each end of a match
 * must not be alphanumeric (undefined, i.e. the string's own edge, always
 * counts as a boundary).
 */
function isWordChar(c: string | undefined): boolean {
  return c !== undefined && /[\w]/.test(c);
}

/**
 * colourSegments splits `line` against the given seat identities. Names are
 * matched longest-first so a seat named "Bot 3" wins over another named
 * "Bot" at the same position (JS regex alternation takes the first
 * alternative that matches, not the longest overall). An empty name is
 * skipped — a blank seat name would otherwise match everywhere.
 */
export function colourSegments(line: string, identities: LogSeatIdentity[]): LogSegment[] {
  const names = identities.filter((i) => i.name.trim().length > 0);
  if (names.length === 0 || !line) return [{ text: line, colour: null }];

  const sorted = [...names].sort((a, b) => b.name.length - a.name.length);
  const colourFor = new Map(sorted.map((s) => [s.name, s.colour]));
  const alt = sorted.map((s) => escapeRe(s.name)).join('|');
  const re = new RegExp(alt, 'g');

  const segments: LogSegment[] = [];
  let last = 0;
  let m: RegExpExecArray | null;
  while ((m = re.exec(line))) {
    const start = m.index;
    const end = start + m[0].length;
    if (start < last) continue; // overlaps a run already emitted
    if (isWordChar(line[start - 1]) || isWordChar(line[end])) continue; // mid-word: not a whole-name match
    if (OBJ_TAIL.test(line.slice(end))) continue; // part of a card's own name, not this player
    if (start > last) segments.push({ text: line.slice(last, start), colour: null });
    segments.push({ text: m[0], colour: colourFor.get(m[0]) ?? null });
    last = end;
  }
  if (last < line.length) segments.push({ text: line.slice(last), colour: null });
  return segments.length ? segments : [{ text: line, colour: null }];
}
