import { describe, expect, it } from 'vitest';
import { render } from 'svelte/server';
import PlayVsBot from './PlayVsBot.svelte';

// The repo's component-test convention is deterministic SSR via svelte/server:
// no DOM, so the click handler and the navigation side-effect never run here
// — this test pins the rendered entry point (format choices, two labelled
// random-default deck pickers and the start control), and its flow is covered by the api and
// playvsbot lib tests plus the Go "starts a real match" suite.
describe('PlayVsBot entry point', () => {
  it('renders a start control with both formats offered, constructed preselected', () => {
    const { html } = render(PlayVsBot, {});
    expect(html).toContain('Play 1v1 vs a bot');
    expect(html).toContain('Start game');
    expect(html).toContain('Commander');
    expect(html).toContain('Constructed');
    expect(html).toContain('value="constructed"');
    expect(html).toContain('value="commander"');
    expect(html).toContain('Your deck');
    expect(html).toContain('Bot deck');
    expect(html.match(/>Random<\/option>/g)).toHaveLength(2);
  });
});
