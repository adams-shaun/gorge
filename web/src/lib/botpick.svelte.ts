import type { BotPick } from './pairedbot';

/**
 * botPick is the paired bot's pick for the decision in front of the player,
 * table-wide, so the controls that answer that decision can mark the one the
 * bot would press: a prompt row, a target chip, the pass button, a card in
 * hand or on the board. PairedBotPanel is its one writer and clears it when
 * the advice no longer matches the decision or the player turns the bot or
 * its marks off. It is display-only state: nothing here is ever posted.
 */
export class BotPickMarks {
  current = $state<BotPick | null>(null);

  set(p: BotPick | null): void {
    this.current = p;
  }

  /** of is the pick for decision seq, null for any other decision. */
  of(seq: number | null | undefined): BotPick | null {
    const p = this.current;
    return p !== null && seq !== null && seq !== undefined && p.seq === seq ? p : null;
  }

  /** option reports whether the pick for decision seq answers with option index. */
  option(seq: number | null | undefined, index: number): BotPick | null {
    const p = this.of(seq);
    return p !== null && p.options.includes(index) ? p : null;
  }

  /** action reports whether the pick for decision seq is the planned cast with payment action id. */
  action(seq: number | null | undefined, id: string): BotPick | null {
    const p = this.of(seq);
    return p !== null && p.payment === id ? p : null;
  }

  /** obj is the pick when it names object id, for whatever decision is pending. */
  obj(id: number): BotPick | null {
    const p = this.current;
    return p !== null && p.objs.includes(id) ? p : null;
  }
}

export const botPick = new BotPickMarks();
