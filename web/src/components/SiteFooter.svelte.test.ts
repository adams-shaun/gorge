import { describe, expect, it } from 'vitest';
import { render } from 'svelte/server';
import SiteFooter from './SiteFooter.svelte';

describe('SiteFooter', () => {
  it('carries the fan-content notice', () => {
    const { body } = render(SiteFooter);
    expect(body).toContain('data-testid="site-footer"');
    expect(body).toContain('Fan Content Policy');
    expect(body).toContain('Scryfall');
  });
});
