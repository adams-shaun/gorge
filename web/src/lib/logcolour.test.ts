import { describe, expect, it } from 'vitest';
import { colourSegments } from './logcolour';

const identities = [
  { name: 'Storm', colour: '#e5484d' },
  { name: 'Cauldron', colour: '#3b82f6' },
  { name: 'Bo', colour: '#22c55e' },
];

describe('colourSegments', () => {
  it('colours a seat name at the head of a described line, the common case', () => {
    const segs = colourSegments('Storm casts Lightning Bolt #12', identities);
    expect(segs[0]).toEqual({ text: 'Storm', colour: '#e5484d' });
    expect(segs.map((s) => s.text).join('')).toBe('Storm casts Lightning Bolt #12'); // no text lost or duplicated
  });

  it('colours a seat name as a combat defender, after "attacks"', () => {
    const segs = colourSegments('Craterhoof Behemoth #7 attacks Storm', identities);
    const coloured = segs.filter((s) => s.colour);
    expect(coloured).toEqual([{ text: 'Storm', colour: '#e5484d' }]);
  });

  it('does NOT colour a seat name that is really the (single-word) head of a card name + id tag', () => {
    // The exact false-positive the brief calls out: a seat named "Storm"
    // must not light up part of "Storm Cauldron #12".
    const segs = colourSegments('Storm Cauldron #12 resolves', identities);
    expect(segs.some((s) => s.colour)).toBe(false);
    expect(segs.map((s) => s.text).join('')).toBe('Storm Cauldron #12 resolves');
  });

  it('does NOT colour a seat name that is the trailing word of a card name + id tag', () => {
    const segs = colourSegments('Storm Cauldron #12 resolves', [{ name: 'Cauldron', colour: '#3b82f6' }]);
    expect(segs.some((s) => s.colour)).toBe(false);
  });

  it('a lowercase MTG-name connector ("of", "the") does not break the card-name guard', () => {
    const segs = colourSegments('Lord of the Pit #9 resolves', [{ name: 'Lord', colour: '#a855f7' }]);
    expect(segs.some((s) => s.colour)).toBe(false);
  });

  it('the longer of two overlapping seat names wins ("Bo" must not eat into "Bot 3")', () => {
    const segs = colourSegments('Bot 3 has priority', [{ name: 'Bot 3', colour: '#eab308' }, { name: 'Bo', colour: '#22c55e' }]);
    const coloured = segs.filter((s) => s.colour);
    expect(coloured).toEqual([{ text: 'Bot 3', colour: '#eab308' }]);
  });

  it('multiple seats in one line (a blocks list) each get their own colour', () => {
    const segs = colourSegments('Storm has priority; Bo has priority', identities);
    const coloured = segs.filter((s) => s.colour).map((s) => s.text);
    expect(coloured).toEqual(['Storm', 'Bo']);
  });

  it('a blank seat name is never matched (it would otherwise match everywhere)', () => {
    const segs = colourSegments('Storm has priority', [{ name: '', colour: '#000' }, ...identities]);
    expect(segs.filter((s) => s.colour).map((s) => s.text)).toEqual(['Storm']);
  });

  it('no identities: the line passes through unchanged as one plain segment', () => {
    expect(colourSegments('Turn 4: Storm', [])).toEqual([{ text: 'Turn 4: Storm', colour: null }]);
  });

  it('an empty line stays an empty plain segment', () => {
    expect(colourSegments('', identities)).toEqual([{ text: '', colour: null }]);
  });

  it('a name with regex metacharacters is matched literally, not as a pattern', () => {
    const segs = colourSegments('P1 (west) has priority', [{ name: 'P1 (west)', colour: '#000' }]);
    expect(segs[0]).toEqual({ text: 'P1 (west)', colour: '#000' });
  });
});
