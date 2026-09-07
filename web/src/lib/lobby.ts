/**
 * Lobby composition (Task L2): how the overview's tables are grouped into
 * sections, and how big a cell should be drawn.
 *
 * Both are decisions, not markup, so they live here with a test rather than
 * inside `Overview.svelte`, where the only way to check them would be a
 * browser.
 */

/** The structural minimum sectionTables needs — a `TableState` satisfies it. */
export interface FormatTable {
  info: { format?: string; state: string };
}

/**
 * The order sections are shown in. Commander first: it is the format a
 * spectator is most likely to have come for (four seats, long games), and a
 * fixed order beats "whatever the server listed first" because the page must
 * not reshuffle under a viewer when a table changes state.
 *
 * A format outside this list still renders, sorted after the known ones —
 * a new server-side format must never make its tables invisible.
 */
export const FORMAT_ORDER: readonly string[] = ['commander', 'constructed'];

/**
 * The wire says `format` is always emitted and that its zero value is
 * "constructed" (protocol.TableInfo). A missing value therefore means an
 * older server, and "constructed" is the right reading of it — not an
 * "unknown" bucket that would split one server's tables into two sections
 * for no reason a viewer could see.
 */
export function normalizeFormat(f: string | undefined | null): string {
  const s = (f ?? '').trim().toLowerCase();
  return s === '' ? 'constructed' : s;
}

/** Title-cases a format name for the section heading ("commander" -> "Commander"). */
export function formatTitle(f: string): string {
  return f.charAt(0).toUpperCase() + f.slice(1);
}

export interface LobbySection<T> {
  /** Normalized format key, e.g. "commander". */
  format: string;
  /** Heading text, e.g. "Commander". */
  title: string;
  tables: T[];
  /** Tables in this section whose state is "live". */
  live: number;
  /**
   * False when this is the only section on the page. A lone "Constructed"
   * label above every table on a single-format server is noise: it separates
   * nothing from nothing.
   */
  heading: boolean;
}

/**
 * sectionTables groups tables by format, Commander first, then constructed,
 * then anything else alphabetically. Order *within* a section is the order
 * the server listed them, so the grid does not reshuffle on a state change.
 */
export function sectionTables<T extends FormatTable>(list: T[]): LobbySection<T>[] {
  const groups = new Map<string, T[]>();
  for (const t of list) {
    const f = normalizeFormat(t.info.format);
    const g = groups.get(f);
    if (g) g.push(t);
    else groups.set(f, [t]);
  }
  const keys = [...groups.keys()].sort((a, b) => {
    const ia = FORMAT_ORDER.indexOf(a);
    const ib = FORMAT_ORDER.indexOf(b);
    if (ia !== ib) return (ia < 0 ? FORMAT_ORDER.length : ia) - (ib < 0 ? FORMAT_ORDER.length : ib);
    return a.localeCompare(b);
  });
  return keys.map((format) => {
    const tables = groups.get(format) ?? [];
    return {
      format,
      title: formatTitle(format),
      tables,
      live: tables.filter((t) => t.info.state === 'live').length,
      heading: keys.length > 1,
    };
  });
}

/**
 * How a cell is sized, given how many tables the whole page is showing.
 *
 * The UI survey's finding for a multi-table overview: a cell is a state
 * widget, not a shrunken board, and SpellTable's ~21%-of-frame per cell is
 * the upper end of "still readable" while the Game Knights life tracker at
 * ~3% is the lower end of "still legible". Four tables on a 1440x900 screen
 * is the case that looked broken: four ~5% chips in a row with three
 * quarters of the page empty under them. So the page picks the cell size
 * from the count — few tables means big cells with seat names and large life
 * totals, many tables means the compact widget, and the grid fills the page
 * either way.
 *
 * `cols` is the *maximum* column count, so four tables land as a 2x2 instead
 * of a 4-wide row on a wide monitor; the CSS still collapses to fewer
 * columns when the window is narrow.
 */
export interface GridMetrics {
  /** Minimum track width, a CSS length. */
  cellMin: string;
  /** Minimum row height, a CSS length. */
  rowMin: string;
  /** Maximum columns; the grid may use fewer if the window is narrow. */
  cols: number;
  /** Life-total type size, a CSS length (the type scale's tokens). */
  lifeSize: string;
  /** True when a cell is big enough to carry seat names beside the life totals. */
  roomy: boolean;
}

export function gridMetrics(count: number): GridMetrics {
  if (count <= 4) return { cellMin: '24rem', rowMin: '13rem', cols: 2, lifeSize: 'var(--t-40)', roomy: true };
  if (count <= 12) return { cellMin: '19rem', rowMin: '10rem', cols: 4, lifeSize: 'var(--t-28)', roomy: false };
  return { cellMin: '15rem', rowMin: '8rem', cols: 6, lifeSize: 'var(--t-20)', roomy: false };
}
