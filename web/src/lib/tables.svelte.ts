import type { Frame, Hello, MatchStart, SeatInfo, TableInfo, TableStateBody, Widget } from '../protocol';
import { SEAT_COLOURS } from './colours';
import { fetchTables } from './api';
import { session } from './session.svelte';

export interface TableState { info: TableInfo; widget: Widget | null; seats: SeatInfo[]; match: number }
// Seat names arrive on TableInfo so a spectator who joins mid-match (or
// before any match_start) sees them without a history fetch. They are only
// deck names — colour comes from the per-seat palette, as the overview
// already colours seats, and MatchStart overrides with the richer list
// (which carries deck+colour) as soon as it arrives.
function seatsFromInfo(info: TableInfo): SeatInfo[] {
  return (info.seat_names ?? []).map((n, i) => ({ name: n, deck: n, colour: SEAT_COLOURS[i % SEAT_COLOURS.length] }));
}

// tableStateName maps a table_state frame's value onto the client's own state
// spelling: the server names a series table's between-games cooldown
// "between_games" (protocol.TableBetweenGames) while TableInfo.State and this
// store call it "cooldown". The value set is fail-closed like the rest of the
// frame handling: only the three states E3-fx2 emits (between_games, live,
// idle) are accepted, and anything else — including no state — leaves the
// store alone rather than feeding an unknown string into the idle derivation.
function tableStateName(state: string | undefined): string | null {
  switch (state) {
    case 'between_games': return 'cooldown';
    case 'live': return 'live';
    case 'idle': return 'idle';
    default: return null;
  }
}

class Tables {
  #list = $state<TableState[]>([]);
  get list(): TableState[] { return this.#list; }

  constructor() {
    session.stream.onFrame((f) => this.apply(f));
  }
  private find(id: string) { return this.#list.find((t) => t.info.id === id); }

  // Seats for a freshly-seen table: a hello/load for one this client already
  // has richer SeatInfos for (from a match_start frame) must keep them —
  // seat_names is the fallback for the common mid-match spectator arrival,
  // whose match_start went out before they connected. The cache only belongs
  // to the match it came from, though: deck assignment rotates every match
  // (host/table.go: Decks[(i+k)%len]), so a hello that lands after a
  // rollover this client missed (disconnected across match_start) carries
  // the fresh names on the wire and must win over the stale cache. Both
  // seat-writing paths write TableState.match together with the seats
  // (hello/load seed it from info.match, match_start/widget from f.match,
  // which the server always sets from the live match number m.k), so the
  // comparison is trustworthy — a mismatch means the cache is a different
  // match's.
  private seedSeats(id: string, info: TableInfo): SeatInfo[] {
    const cur = this.find(id);
    if (cur && cur.match === info.match && cur.seats.length > 0) return cur.seats;
    return seatsFromInfo(info);
  }

  apply(f: Frame) {
    switch (f.t) {
      case 'hello': {
        const h = f.body as Hello;
        this.#list = h.tables.map((info) => ({ info, widget: this.find(info.id)?.widget ?? null, seats: this.seedSeats(info.id, info), match: info.match }));
        break;
      }
      case 'widget': {
        const t = this.find(f.table ?? '');
        if (t) { t.widget = f.body as Widget; t.match = f.match ?? t.match; }
        break;
      }
      case 'match_start': {
        const t = this.find(f.table ?? '');
        // A fresh match has no widget burst yet — clear the prior match's
        // life/turn/phase/stack rather than showing it under a LIVE badge.
        if (t) { t.widget = null; t.seats = (f.body as MatchStart).seats; t.match = f.match ?? t.match; t.info = { ...t.info, state: 'live', match: t.match }; }
        break;
      }
      case 'match_end': {
        const t = this.find(f.table ?? '');
        // A series table (E3-2, TableInfo.series) goes on to its next match,
        // so its match_end is the between-games state, never idle; the
        // server's table_state frame then says whether the series goes on
        // (between_games, then live) or is over (idle). A perpetual table
        // cools down and restarts; only an ordinary table is idle.
        if (t) t.info = { ...t.info, state: t.info.perpetual || t.info.series === true ? 'cooldown' : 'idle' };
        break;
      }
      case 'table_state': {
        const t = this.find(f.table ?? '');
        const state = tableStateName((f.body as Partial<TableStateBody>).state);
        if (t && state !== null) t.info = { ...t.info, state };
        break;
      }
      case 'table_halted': {
        const t = this.find(f.table ?? '');
        if (t) t.info = { ...t.info, state: 'halted' };
        break;
      }
    }
  }
  async load() {
    const infos = await fetchTables();
    const lobby = infos.map((info) => ({ info, widget: this.find(info.id)?.widget ?? null, seats: this.seedSeats(info.id, info), match: info.match }));
    // /api/tables is the public lobby, so it omits private on-demand tables.
    // A client already seated at one keeps its TableInfo (and widget/seats)
    // across an Overview visit rather than losing Auto Mana/Restart. Entries
    // the lobby does return take the response, including its order.
    const lobbyIDs = new Set(infos.map((info) => info.id));
    this.#list = [...lobby, ...this.#list.filter((t) => !lobbyIDs.has(t.info.id))];
  }
}

export const tables = new Tables();
