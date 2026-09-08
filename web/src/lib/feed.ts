export interface FeedLine { table: string; match: number; seq: number; line: string }

export function pushFeed(lines: FeedLine[], l: FeedLine, cap = 200): FeedLine[] {
  if (!l.line) return lines;
  if (lines.some((x) => x.table === l.table && x.match === l.match && x.seq === l.seq)) return lines;
  const out = [...lines, l];
  return out.length > cap ? out.slice(out.length - cap) : out;
}

/*
 * What the lobby rail actually receives (Task L2).
 *
 * The rail is fed by `widget.last` — one line per widget burst per table,
 * host-side. Sampled against the live demo server, 130 consecutive bursts
 * across four tables produced 130 lines and every one of them had the shape
 * "<deck> is asked: priority" (one "is asked: attackers"). That is not a
 * transcript, it is a heartbeat: near-zero entropy per line, and because the
 * four tables tick in rotation, adjacent-duplicate collapsing merges
 * nothing.
 *
 * Fixing that at the source is Go-side (host/fanout.go picks the last
 * described line of a burst) and is deliberately out of scope here. So the
 * rail is built from two views of the same stream instead of one scroll:
 *
 *   latestPerTable  — the current line for each table, with a repeat count.
 *                     A stable row per table; it does not reshuffle.
 *   notableLines    — the chronological lines that are not routine.
 *
 * Both degrade correctly the day the host sends real event lines: "now"
 * becomes each table's most recent action, and "notable" becomes the
 * table-tagged merged transcript the UI survey asks for (rec 33).
 */

/**
 * A line is routine when it reports only that the engine reached a priority
 * window — the lobby's entire volume today. The match is deliberately narrow
 * (these two shapes, anchored at the end of the line) rather than a general
 * "decision" filter: "is asked: attackers", "is asked: mulligan" and
 * "answers ..." all report that something is actually happening, and hiding
 * a real line is worse than showing a dull one.
 *
 * The shapes come from view/describe.go: events.Priority renders as
 * "<player> has priority" and a priority DecisionAsk as
 * "<player> is asked: priority".
 */
const ROUTINE = /(?: has priority| is asked: priority)$/;

export function isRoutineLine(line: string): boolean {
  return ROUTINE.test(line);
}

/**
 * isStepLine reports whether a line is a phase/step boundary. view/describe.go
 * renders a StepChange event literally as "Step: <phase>", so this is a match
 * against the server's own documented contract, not a guess. There is no event
 * kind on the rail (host/fanout.go puts a bare string in Widget.Last), so the
 * literal is the only handle; the transcript filters the same lines by the
 * 'step' event kind (logfilter.ts).
 */
export function isStepLine(line: string): boolean {
  return /^Step: /.test(line);
}

/**
 * The key a repeat run is counted over. Two routine lines that name
 * different players still say the same thing — "this table is passing
 * priority around" — and in a four-seat game the name changes every burst,
 * so counting exact text would count to one forever. Everything else is
 * counted by its exact text.
 */
function runKey(line: string): string {
  // The sentinel starts with NUL so it can never equal a described line.
  return isRoutineLine(line) ? '\u0000priority' : line;
}

/** One rail row: a table's current line and how many bursts have repeated it. */
export interface FeedNow {
  table: string;
  /** The table's most recent line, or "" if it has not reported yet. */
  line: string;
  /** How many consecutive most-recent lines from this table say the same thing (1 = new). */
  count: number;
  routine: boolean;
}

/**
 * latestPerTable returns one row per id in `order` — including tables that
 * have said nothing yet, so the rail's row count matches the grid's table
 * count and rows never jump position. `count` collapses the repeat run at
 * the end of that table's own lines (see runKey), ignoring the other tables
 * interleaved between them. A table's current line skips step lines (ui9,
 * B4): a phase boundary is clock noise, so a table whose only recent lines
 * are "Step: …" reports an empty line rather than flashing the clock.
 */
export function latestPerTable(lines: FeedLine[], order: readonly string[]): FeedNow[] {
  const byTable = new Map<string, string[]>();
  for (const l of lines) {
    const g = byTable.get(l.table);
    if (g) g.push(l.line);
    else byTable.set(l.table, [l.line]);
  }
  return order.map((table) => {
    const own = byTable.get(table) ?? [];
    const last = [...own].reverse().find((x) => !isStepLine(x));
    const line = last ?? '';
    const key = runKey(line);
    let count = 0;
    for (let i = own.length - 1; i >= 0; i--) {
      if (isStepLine(own[i])) continue; // step lines are not part of an action's repeat run
      if (runKey(own[i]) === key) count++;
      else break;
    }
    return { table, line, count, routine: isRoutineLine(line) };
  });
}

/**
 * lastNotableByTable is "what last happened here", per table: the most recent
 * non-routine line each table produced. A roomy cell prints it under its life
 * grid — the overview's version of SpellTable's Last Card pane (UI survey rec
 * 26), and the one thing on the page that says *why* a life total moved.
 *
 * It is deliberately not the table's *current* line: that changes several
 * times a second and names a different seat each time, so a cell showing it
 * would flicker. A notable line changes on the order of seconds and holds
 * still in between.
 */
export function lastNotableByTable(lines: FeedLine[]): Map<string, string> {
  const out = new Map<string, string>();
  for (const l of lines) if (!isRoutineLine(l.line) && !isStepLine(l.line)) out.set(l.table, l.line);
  return out;
}

/**
 * notableLines is the chronological half of the rail: oldest first, newest
 * last, routine lines dropped unless `all` is set. `cap` keeps the rendered
 * list bounded independently of the feed's own cap.
 */
export function notableLines(lines: FeedLine[], all = false, cap = 80): FeedLine[] {
  const out = all ? lines.slice() : lines.filter((l) => !isRoutineLine(l.line) && !isStepLine(l.line));
  return out.length > cap ? out.slice(out.length - cap) : out;
}
