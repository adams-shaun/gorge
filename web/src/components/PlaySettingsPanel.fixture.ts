import { mount } from 'svelte';
import '../app.css';
import PlaySettingsPanel from './PlaySettingsPanel.svelte';
import { SeatPanelState } from '../lib/seatpanel.svelte';

/**
 * The OPTIONS editor mounted against a REAL SeatPanelState — the same
 * binding the live seat panel uses. Storage is null so no localStorage key
 * leaks between page loads (every page starts at casual). The state itself
 * is published on window so the playwright test can reach the machine-side
 * brake (suspendAuto): a deliberate internal, never rendered as a control,
 * and exactly the thing the paused-to-preset regression needs to trip.
 */
const state = new SeatPanelState('t1', 1, { seat: 0, token: 'tok' }, null);
(window as unknown as { playSettingsState: SeatPanelState }).playSettingsState = state;

// The game log switch (fb-20260917T231628Z) is wired to window-owned props so
// the playwright test can prove a real click reaches the write path without
// re-mounting: showLog is a static false (the seated default) and every call
// of onToggleLog increments the counter the test reads back.
(window as unknown as { gameLogToggles: number }).gameLogToggles = 0;

mount(PlaySettingsPanel, {
  target: document.querySelector('#fixture')!,
  props: {
    state,
    showLog: false,
    onToggleLog: () => {
      (window as unknown as { gameLogToggles: number }).gameLogToggles += 1;
    },
  },
});
