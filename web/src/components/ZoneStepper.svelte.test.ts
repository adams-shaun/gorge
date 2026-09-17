import { describe, expect, it } from 'vitest';
import { render } from 'svelte/server';
import ZoneStepper from './ZoneStepper.svelte';
import { layoutStore } from '../lib/layoutsettings.svelte';

/**
 * ZoneStepper is the on-board resize affordance (fb-20260916T182801Z): a
 * − / + pair with the zone's size readout. SSR pins what it SHOWS for a
 * given store state (the store's own file covers the write path; this
 * environment has no DOM, so a click cannot be driven here — the panel
 * fixture's pattern is where real clicks are exercised, and the panel
 * section shares the store's methods directly).
 */
describe('ZoneStepper', () => {
  it('renders the − / + pair, the zone key and the 100% readout at the defaults', () => {
    const { html } = render(ZoneStepper, { props: { zone: 'creatures', label: 'creature' } });
    expect(html).toContain('data-zone-stepper="creatures"');
    expect(html).toContain('data-zone-step="smaller"');
    expect(html).toContain('data-zone-step="larger"');
    expect(html).toContain('data-zone-scale-readout="creatures"');
    expect(html).toContain('100%');
    expect(html).toContain('aria-label="Smaller creature cards"');
    expect(html).toContain('aria-label="Larger creature cards"');
  });

  it('the readout follows the store: a bump is visible and reset restores 100%', () => {
    layoutStore.bump('hand', 0.2);
    const { html } = render(ZoneStepper, { props: { zone: 'hand', label: 'hand' } });
    expect(html).toContain('120%');
    layoutStore.reset();
    layoutStore.dispose();
    const calm = render(ZoneStepper, { props: { zone: 'hand', label: 'hand' } }).html;
    expect(calm).toContain('100%');
  });
});
