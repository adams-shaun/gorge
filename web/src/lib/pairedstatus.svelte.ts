import type { PairedBotStatus } from '../protocol';
import { fetchPairedBotStatus } from './api';
import type { SeatCtx } from './seat';

/**
 * PairedStatus holds each table's paired-bot offer as the table route last
 * fetched it. The route reads it to decide where the paired bot's panel goes
 * (its own rail or the main one) and hands it to the panel, so the offer is
 * fetched once and a panel never moves after its own fetch lands.
 */
export class PairedStatus {
  #by = $state<Record<string, PairedBotStatus | null>>({});

  of(table: string): PairedBotStatus | null {
    return this.#by[table] ?? null;
  }

  set(table: string, status: PairedBotStatus | null): void {
    this.#by = { ...this.#by, [table]: status };
  }

  /** load fetches the table's offer; a failed fetch means no offer. */
  async load(table: string, ctx: SeatCtx): Promise<void> {
    let status: PairedBotStatus | null;
    try {
      status = await fetchPairedBotStatus(table, ctx);
    } catch {
      status = null;
    }
    this.set(table, status);
  }
}

export const pairedStatus = new PairedStatus();
