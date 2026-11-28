import { readFileSync } from 'node:fs';
import { describe, expect, it } from 'vitest';
import { render } from 'svelte/server';
import type { DeckReport } from '../protocol';
import DeckReportPanel from './DeckReportPanel.svelte';

// The panel shows only what the server computed: rendered straight from the
// shared fixture (the same file TestWebDeckCheckFixturesDecode pins in the
// server's shape), it carries the status line, the problems, the unsupported
// cards, the swaps and the notice — and never a rate: the fixture report and
// every string the panel builds are free of a percent sign, so their absence
// in the HTML is the assertion that no simulation figure is ever shown.
const report = JSON.parse(readFileSync(new URL('../../../server/host/httpapi/testdata/web-deck-report.json', import.meta.url), 'utf8')) as DeckReport;

describe('DeckReportPanel', () => {
  it('renders the fixture report verbatim, with the substitution notice and no rate', () => {
    const { html } = render(DeckReportPanel, { props: { report, substitute: true } });
    expect(html).toContain('data-testid="deck-report"');
    expect(html).toContain('Not legal');
    expect(html.match(/data-testid="deck-report-problem"/g)).toHaveLength(1);
    expect(html).toContain('has 101 cards, wants exactly 100');
    expect(html.match(/data-testid="deck-report-unsupported"/g)).toHaveLength(2);
    expect(html).toContain('Leyline of the Hidden ×2 — needs trigger: a countered spell is missed');
    expect(html.match(/data-testid="deck-report-substituted"/g)).toHaveLength(2);
    expect(html).toContain('Leyline of the Hidden → Swamp ×1');
    expect(html).toContain('Vessel of Wildfire → Swamp ×1');
    expect(html).toContain('This deck will play with 2 basic lands in place of the cards listed above. Games with it are not games of the deck as written.');
    // The consent checkbox is offered only because the report has unsupported cards.
    expect(html).toContain('data-testid="deck-substitute"');
    expect(html).toContain('checked');
    expect(html).not.toContain('%');
  });

  it('renders nothing at all without a report', () => {
    const { html } = render(DeckReportPanel, { props: { report: null, substitute: false } });
    // Nothing but Svelte's hydration anchors: no panel, no strings.
    expect(html.replace(/<!--[^]*?-->/g, '')).toBe('');
  });

  it('hides the consent checkbox when nothing is unsupported', () => {
    // The notice is the server's substituted list's own business: an empty
    // unsupported list hides only the consent, not a swap already made.
    const { html } = render(DeckReportPanel, { props: { report: { ...report, unsupported: [] }, substitute: false } });
    expect(html).not.toContain('data-testid="deck-substitute"');
    expect(html).toContain('This deck will play with 2 basic lands');
  });

  it('shows the dirac verdict on one line when the server asked for it', () => {
    const { html } = render(DeckReportPanel, {
      props: { report: { ...report, dirac: { engine: 'dirac', set: 'a202511', playable: false, unsupported: [{ name: 'Leyline of the Hidden', count: 2, status: 'not lowered', reason: '' }], problems: [] } }, substitute: false },
    });
    expect(html).toContain('dirac: cannot play 1 cards: Leyline of the Hidden');
  });
});
