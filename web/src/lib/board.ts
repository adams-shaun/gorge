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

/** A stack is one group of interchangeable permanents: everything a player would act on matches, so one tile with a count stands in for all of them. `key` is the deterministic identity the group was built from; cards are the members, id-sorted. A group of one is still a group — the caller renders it exactly like a single permanent. */
export interface CardStackGroup {
  key: string;
  cards: CardView[];
}

/** attachedToId returns the id of the permanent c is attached to, or null when it is unattached. The wire sends 0 for "no host" the same way it omits the field (view.go), so both mean unattached here. */
function attachedToId(c: CardView): number | null {
  return c.attached_to !== undefined && c.attached_to !== 0 ? c.attached_to : null;
}

/** stackKey is the identity two permanents must share to be interchangeable. Everything a player could act on is included; power/toughness are DERIVED on the wire, so two cards under different anthems already differ here. The string is deterministic: counter keys and the keyword set are sorted before joining, and array/list fields never depend on wire order. */
function stackKey(c: CardView): string {
  const blocked = [...(c.blocked_by ?? [])].sort((a, b) => a - b);
  const counters = Object.entries(c.counters ?? {}).sort(([a], [b]) => (a < b ? -1 : a > b ? 1 : 0));
  const keywords = [...(c.keywords ?? [])].sort();
  return [
    'p=' + c.printing.name,
    's=' + (c.printing.set ?? ''),
    'n=' + (c.printing.number ?? ''),
    't=' + c.tapped,
    'ss=' + c.summon_sick,
    'a=' + c.attacking,
    'd=' + c.damage,
    'pw=' + c.power,
    'tu=' + c.toughness,
    'ap=' + (c.attacking_player ?? ''),
    'bb=' + blocked.join(','),
    'ctr=' + c.controller,
    'ct=' + counters.map(([k, v]) => k + '=' + v).join(','),
    'kw=' + keywords.join(','),
  ].join('|');
}

/**
 * stackIdentical merges same-printing permanents into one group per identity. Merging too aggressively silently hides state, so the merge key is deliberately strict and attachment state wins outright: a permanent that is itself attached, or that another permanent is attached to, is individual by definition — its tile shows riders a group could not compose. When in doubt, do not merge. Groups come back ordered by their lowest member id, members id-sorted, so the caller's layout is stable whatever order the wire delivered.
 */
export function stackIdentical(cards: CardView[]): CardStackGroup[] {
  const hosts = new Set<number>();
  for (const c of cards) {
    const host = attachedToId(c);
    if (host !== null) hosts.add(host);
  }
  const mergeable = (c: CardView) => attachedToId(c) === null && !hosts.has(c.id);

  const byKey = new Map<string, CardView[]>();
  for (const c of cards) {
    if (!mergeable(c)) continue;
    const key = stackKey(c);
    const group = byKey.get(key);
    if (group === undefined) byKey.set(key, [c]);
    else group.push(c);
  }
  const groups: CardStackGroup[] = [];
  for (const [key, cs] of byKey) {
    cs.sort((a, b) => a.id - b.id);
    groups.push({ key, cards: cs });
  }
  // A permanent that may not merge stays its own group of one, keyed by its
  // id so it can never collide with an identity key.
  for (const c of cards) {
    if (mergeable(c)) continue;
    groups.push({ key: '#' + c.id, cards: [c] });
  }
  groups.sort((a, b) => a.cards[0].id - b.cards[0].id);
  return groups;
}

/** stackFaces picks the faces a stack group renders in a given expansion state: collapsed shows only the first card (the fan stands in for the rest), expanded shows every member. Keeping the mapping here lets the collapsed/expanded contract be tested without a DOM. */
export function stackFaces(group: CardStackGroup, expanded: boolean): CardView[] {
  return expanded ? group.cards : [group.cards[0]];
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
