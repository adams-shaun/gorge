import { describe, expect, it } from 'vitest';
import { eventsURL, matchesURL, tablesURL, viewURL } from './api';

describe('api urls', () => {
  it('builds the documented paths', () => {
    expect(tablesURL()).toBe('/api/tables');
    expect(matchesURL('t1')).toBe('/api/tables/t1/matches');
    expect(viewURL('t1', 3)).toBe('/api/tables/t1/matches/3/view');
    expect(viewURL('t1', 3, 0)).toBe('/api/tables/t1/matches/3/view?seq=0');
    expect(eventsURL('t 1', 3, 42)).toBe('/api/tables/t%201/matches/3/events?since=42');
  });
});
