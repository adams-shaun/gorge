import { describe, expect, it } from 'vitest';
import { initSeatContext, getSeat, parseSeat } from './seat';

describe('seat.ts', () => {
  it('parses the join URL: ?seat=N&token=…', () => {
    expect(parseSeat('?seat=0&token=abc')).toEqual({ seat: 0, token: 'abc' });
    expect(parseSeat('?seat=3&token=x%20y')).toBeNull(); // a token with spaces is not a bearer token
  });

  it('returns null when either parameter is missing or malformed — the page stays spectator-only', () => {
    expect(parseSeat('')).toBeNull();
    expect(parseSeat('?seat=0')).toBeNull();
    expect(parseSeat('?token=abc')).toBeNull();
    expect(parseSeat('?seat=abc&token=abc')).toBeNull();
    expect(parseSeat('?seat=-1&token=abc')).toBeNull();
    expect(parseSeat('?seat=0&token=')).toBeNull();
    expect(parseSeat('?seat=2.5&token=abc')).toBeNull();
  });

  it('init/get read the seat once and keep it for the session', () => {
    initSeatContext('?seat=1&token=t1');
    expect(getSeat()).toEqual({ seat: 1, token: 't1' });
    initSeatContext('');
    expect(getSeat()).toBeNull();
  });
});
