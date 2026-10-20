import { describe, expect, it } from 'vitest';
import { matchesURL } from './api';

const ctx = { seat: 0, token: 'g1-token' };

describe('private table matches URL', () => {
  it('appends the seat token only when seated', () => {
    expect(matchesURL('g1')).toBe('/api/tables/g1/matches');
    expect(matchesURL('g1', ctx)).toBe('/api/tables/g1/matches?seat=0&token=g1-token');
  });
});
