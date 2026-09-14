import { describe, expect, it } from 'vitest';
import { render } from 'svelte/server';
import OracleText from './OracleText.svelte';

// The repo's component-test convention is deterministic SSR via svelte/server
// (see CardTile.svelte.test.ts's header note): this environment has no DOM,
// so only the markup the component renders can be asserted. These renders pin
// the popup's oracle paragraph contract: mana tokens draw ManaSymbols' own
// pip classes at the inline size, the tap icon draws its outlined disc, an
// unknown token stays literal braced text, and the paragraph's pre-wrap line
// breaks survive — they live in the tokenizer's verbatim text runs.
describe('OracleText — the printed card\u2019s rules text renders its symbols', () => {
  it('{B} draws the same b pip the transcript draws, at the inline size', () => {
    const { html } = render(OracleText, { props: { text: 'Sacrifice a {B} permanent.' } });
    expect(html).toContain('mana-symbols--inline');
    expect(html).toContain('pip p-b');
    expect(html).toContain('B</span>');
  });

  it('{2/B} draws the split twobrid pip', () => {
    const { html } = render(OracleText, { props: { text: 'Pay {2/B}.' } });
    expect(html).toContain('pip p-split p-x-b');
    expect(html).toContain('2/B');
  });

  it('{W/P} draws the phyrexian pip — one classifier with the transcript', () => {
    const { html } = render(OracleText, { props: { text: 'Pay {W/P}.' } });
    expect(html).toContain('pip p-split p-w-x');
  });

  it('{T} draws the outlined tap icon, not a mana pip', () => {
    const { html } = render(OracleText, { props: { text: '{T}: Add {C}.' } });
    expect(html).toContain('title="tap"');
    expect(html).toContain('title="tap">T</span>');
  });

  it('{TK} stays literal braced text — never dropped, never guessed at', () => {
    const { html } = render(OracleText, { props: { text: 'Roll {TK}.' } });
    expect(html).toContain('{TK}');
    expect(html).not.toContain('mana-symbols');
  });

  it('a multi-line text keeps its newline for the paragraph\u2019s pre-wrap', () => {
    const { html } = render(OracleText, {
      props: { text: 'Flying, vigilance\nEach other Angel enters with an additional +1/+1 counter for each {W} spent to cast it.' },
    });
    expect(html).toContain('Flying, vigilance\n');
    expect(html).toContain('pip p-w');
  });
});
