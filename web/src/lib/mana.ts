/** A colour that can appear as a face of a mana symbol. */
export type ManaColour = 'W' | 'U' | 'B' | 'R' | 'G';

/** A hybrid face: a colour or colourless (C). */
export type ManaFace = ManaColour | 'C';

/**
 * One parsed mana pip. `text` is the pip exactly as written in the cost (so a
 * renderer shows what was authored: "W/U", "2W", "GUP"). The discriminant
 * `kind` carries the structure a renderer needs to draw the pip — a hybrid
 * knows its two faces, a twobrid its colour, a Phyrexian pip its colour — so
 * nothing has to be re-parsed out of a string downstream.
 */
export type ManaSymbol =
  | { kind: 'generic'; value: number; text: string }
  | { kind: 'variable'; letter: 'X' | 'Y' | 'Z'; text: string }
  | { kind: 'colour'; colour: ManaColour; text: string }
  | { kind: 'colourless'; text: string }
  | { kind: 'snow'; text: string }
  | { kind: 'twobrid'; colour: ManaColour; text: string }
  | { kind: 'hybrid'; a: ManaFace; b: ManaFace; text: string }
  | { kind: 'phyrexian'; colour: ManaColour; text: string }
  | { kind: 'phyrexianHybrid'; a: ManaColour; b: ManaColour; text: string }
  | { kind: 'unknown'; text: string };

const COLOURS = 'WUBR G';
const FACES = COLOURS + 'C';

/**
 * Classify one whitespace-delimited token as a mana pip. Every token yields a
 * symbol — a token that matches nothing becomes `{ kind: 'unknown' }` rather
 * than being dropped, so one unparseable pip can never blank the rest of a
 * cost. Unknown pips render as themselves so the corruption is visible.
 */
function parseSymbol(t: string): ManaSymbol {
  if (/^\d+$/.test(t)) return { kind: 'generic', value: Number(t), text: t };
  if (/^[XYZ]$/.test(t)) return { kind: 'variable', letter: t as 'X' | 'Y' | 'Z', text: t };
  if (/^[WUBRG]$/.test(t)) return { kind: 'colour', colour: t as ManaColour, text: t };
  if (t === 'C') return { kind: 'colourless', text: t };
  if (t === 'S') return { kind: 'snow', text: t };
  // Phyrexian hybrids — two colours plus a Phyrexian face, P leading or
  // trailing (the corpus writes both: "GUP"/"RWP" and "PRG").
  const col = (s: string): ManaColour => s as ManaColour;
  const face = (s: string): ManaFace => s as ManaFace;
  let m = t.match(/^([WUBRG])([WUBRG])P$/);
  if (m) return { kind: 'phyrexianHybrid', a: col(m[1]), b: col(m[2]), text: t };
  m = t.match(/^P([WUBRG])([WUBRG])$/);
  if (m) return { kind: 'phyrexianHybrid', a: col(m[1]), b: col(m[2]), text: t };
  // Phyrexian single colour.
  m = t.match(/^([WUBRG])P$/);
  if (m) return { kind: 'phyrexian', colour: col(m[1]), text: t };
  // Twobrid — {2/X} in both corpus notations, slashed and unslashed.
  m = t.match(/^2\/([WUBRG])$/);
  if (m) return { kind: 'twobrid', colour: col(m[1]), text: t };
  m = t.match(/^2([WUBRG])$/);
  if (m) return { kind: 'twobrid', colour: col(m[1]), text: t };
  // Two-face hybrids — {W/U} slashed and {WU} unslashed, face can be
  // colourless too ({C/W}, corpus: "CW").
  m = t.match(new RegExp(`^([${FACES}])/([${FACES}])$`));
  if (m) return { kind: 'hybrid', a: face(m[1]), b: face(m[2]), text: t };
  m = t.match(new RegExp(`^([${FACES}])([${FACES}])$`));
  if (m) return { kind: 'hybrid', a: face(m[1]), b: face(m[2]), text: t };
  return { kind: 'unknown', text: t };
}

/**
 * manaSymbols parses Forge's cost notation ("1 W", "X G G", "WU", "2W", "GUP")
 * into structured, renderable symbols. Unlike the old validator, one unknown
 * pip is kept (as `kind: 'unknown'`) and never blanks the rest of the cost.
 * The whole-string marker "no cost" — how the corpus writes cards that have no
 * mana cost — yields an empty list.
 */
export function manaSymbols(cost: string): ManaSymbol[] {
  if (!cost) return [];
  const trimmed = cost.trim();
  if (trimmed === '' || /^no cost$/i.test(trimmed)) return [];
  return trimmed.split(/\s+/).map(parseSymbol);
}
