import type { CardView, EventBody, PlayerView } from '../protocol';

export type Group = 'lands' | 'creatures' | 'others';

/**
 * visibleHand reads a player's hand off the wire view. protocol.ts generates
 * PlayerView.hand as CardView[] (non-nullable), but a Go nil slice
 * serializes to JSON `null`, and any table configured Spectator: Public
 * sends exactly that for every seat: a null hand, not an omitted or
 * empty-array one. Everything that reads a hand should go through this
 * rather than `p.hand` directly, so the null case is handled once.
 */
export function visibleHand(p: PlayerView): CardView[] | null {
  return p.hand ?? null;
}

/** groupBattlefield sorts a seat's permanents into the three rows a quadrant shows. Type words come from the view; nothing here decides what a card does. Attachments are excluded — attachedTo places them under their host instead. */
export function groupBattlefield(cards: CardView[]): Record<Group, CardView[]> {
  const out: Record<Group, CardView[]> = { lands: [], creatures: [], others: [] };
  const hosts = new Set(cards.map((c) => c.id));
  for (const c of [...cards].sort((a, b) => a.id - b.id)) {
    // An Aura or Equipment whose host is on this battlefield rides under that
    // host (see attachedTo) rather than occupying a slot of its own. One whose
    // host is NOT here — a stolen or otherwise off-board host — still has to
    // be drawn somewhere, so it falls through to its own type row rather than
    // vanishing.
    if (c.attached_to !== undefined && c.attached_to !== 0 && hosts.has(c.attached_to)) continue;
    const words = c.types.split(' ');
    if (words.includes('Creature')) out.creatures.push(c);
    else if (words.includes('Land')) out.lands.push(c);
    else out.others.push(c);
  }
  return out;
}

/** attachedTo returns the permanents attached to host, in id order. The wire carries CardView.attached_to (W1); nothing here decides what an attachment does. */
export function attachedTo(cards: CardView[], host: number): CardView[] {
  return cards.filter((c) => c.attached_to === host).sort((a, b) => a.id - b.id);
}

/** quadrantFor places seat 0 bottom-left and proceeds clockwise, so turn order reads around the table. Assumes at most 4 seats — 5-8 seat tables are out of M2a's focused-view scope. */
export function quadrantFor(seat: number, seats: number): 'tl' | 'tr' | 'bl' | 'br' | 'l' | 'r' {
  if (seats <= 2) return seat === 0 ? 'l' : 'r';
  return (['bl', 'tl', 'tr', 'br'] as const)[seat % 4];
}

/** recentlyMattered is the object id of the most recent stack_resolve, for the strip. */
export function recentlyMattered(events: EventBody[]): number | null {
  for (let i = events.length - 1; i >= 0; i--) {
    const e = events[i].event;
    if (e.kind === 'stack_resolve' && e.obj) return e.obj;
  }
  return null;
}
