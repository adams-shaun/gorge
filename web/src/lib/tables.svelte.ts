import type { Frame, Hello, MatchStart, SeatInfo, TableInfo, Widget } from '../protocol';
import { fetchTables } from './api';
import { session } from './session.svelte';

export interface TableState { info: TableInfo; widget: Widget | null; seats: SeatInfo[]; match: number }

class Tables {
  #list = $state<TableState[]>([]);
  get list(): TableState[] { return this.#list; }

  constructor() {
    session.stream.onFrame((f) => this.apply(f));
  }
  private find(id: string) { return this.#list.find((t) => t.info.id === id); }

  apply(f: Frame) {
    switch (f.t) {
      case 'hello': {
        const h = f.body as Hello;
        this.#list = h.tables.map((info) => ({ info, widget: this.find(info.id)?.widget ?? null, seats: this.find(info.id)?.seats ?? [], match: info.match }));
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
        if (t) t.info = { ...t.info, state: t.info.perpetual ? 'cooldown' : 'idle' };
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
    this.#list = infos.map((info) => ({ info, widget: this.find(info.id)?.widget ?? null, seats: this.find(info.id)?.seats ?? [], match: info.match }));
  }
}

export const tables = new Tables();
