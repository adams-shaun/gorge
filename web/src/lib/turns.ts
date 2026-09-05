import type { EventBody } from '../protocol';

/** turnStartsFrom derives the scrub ticks from a finished match's events (a live match gets them in its snapshot). */
export const turnStartsFrom = (events: EventBody[]): number[] =>
  events.filter((e) => e.event.kind === 'turn').map((e) => e.event.seq);
