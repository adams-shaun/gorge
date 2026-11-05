import { describe, expect, it } from 'vitest';
import { partnerLayout } from './partnerrail';

describe('partnerLayout', () => {
  it('gives an offered paired bot its own column opposite the main rail by default', () => {
    expect(partnerLayout('opposite', 'right', true, false)).toEqual({ where: 'column', side: 'left' });
    expect(partnerLayout('opposite', 'left', true, false)).toEqual({ where: 'column', side: 'right' });
    // A hidden main rail is a drawer on the right edge.
    expect(partnerLayout('opposite', 'hidden', true, false)).toEqual({ where: 'column', side: 'left' });
  });

  it('keeps the panel in the main rail when the profile says so', () => {
    expect(partnerLayout('same', 'right', true, false)).toEqual({ where: 'main' });
    expect(partnerLayout('same', 'left', true, false)).toEqual({ where: 'main' });
  });

  it('tucks a hidden paired bot into an edge drawer opposite the main rail, on any width', () => {
    expect(partnerLayout('hidden', 'right', true, false)).toEqual({ where: 'drawer', side: 'left' });
    expect(partnerLayout('hidden', 'left', true, true)).toEqual({ where: 'drawer', side: 'right' });
  });

  it('never takes a column on a narrow screen: the panel joins the main rail', () => {
    expect(partnerLayout('opposite', 'right', true, true)).toEqual({ where: 'main' });
    expect(partnerLayout('opposite', 'left', true, true)).toEqual({ where: 'main' });
  });

  it('without an offer there is no partner rail, whatever the setting', () => {
    for (const placement of ['opposite', 'same', 'hidden'] as const) {
      expect(partnerLayout(placement, 'right', false, false), placement).toEqual({ where: 'main' });
    }
  });
});
