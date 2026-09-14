import { describe, expect, it } from 'vitest';
import { ClientBreadcrumbs, RingBuffer } from './breadcrumbs';
import { defaultSettings } from './playsettings';

describe('RingBuffer', () => {
  it('keeps its cap in insertion order and snapshots as JSON data', () => {
    const ring = new RingBuffer<{ n: number }>(3);
    for (let n = 1; n <= 5; n++) ring.push({ n });
    expect(ring.values()).toEqual([{ n: 3 }, { n: 4 }, { n: 5 }]);
    expect(JSON.stringify(ring.values())).toBe('[{"n":3},{"n":4},{"n":5}]');
  });
});

describe('ClientBreadcrumbs', () => {
  it('emits the feedback client.json shape with settings, yields and actions', () => {
    const crumbs = new ClientBreadcrumbs();
    crumbs.setView(23, 24);
    crumbs.setPlay(defaultSettings(), ['7:Blood Artist:drain']);
    crumbs.record('intent_sent', { decision_kind: 'target', choices: [{ index: 2, kind: 'permanent' }] });
    crumbs.consoleError([new Error('render failed')]);

    const snapshot = crumbs.snapshot(null);
    expect(snapshot).toMatchObject({
      version: 1,
      view: { sequence: 23, intent_index: 24 },
      play: { settings: { preset: 'casual' }, active_yields: ['7:Blood Artist:drain'] },
      actions: [{ type: 'intent_sent', detail: { decision_kind: 'target' } }],
      bundle_build_id: null,
      viewport: { width: null, height: null },
      console_errors: [{ type: 'console_error' }],
    });
    expect(() => JSON.parse(JSON.stringify(snapshot))).not.toThrow();
  });
});
