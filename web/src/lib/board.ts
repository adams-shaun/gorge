import type { CardView, EventBody } from '../protocol';

export type Group = 'lands' | 'creatures' | 'others';

/** groupBattlefield sorts a seat's permanents into the three rows a quadrant shows. Type words come from the view; nothing here decides what a card does. */
export function groupBattlefield(cards: CardView[]): Record<Group, CardView[]> {
  const out: Record<Group, CardView[]> = { lands: [], creatures: [], others: [] };
  for (const c of [...cards].sort((a, b) => a.id - b.id)) {
    const words = c.types.split(' ');
    if (words.includes('Creature')) out.creatures.push(c);
    else if (words.includes('Land')) out.lands.push(c);
    else out.others.push(c);
  }
  return out;
}

/** quadrantFor places seat 0 bottom-left and proceeds clockwise, so turn order reads around the table. */
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
