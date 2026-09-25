import { mount } from 'svelte';
import '../app.css';
import SeatPanelFollowUpFixture from './SeatPanelFollowUp.fixture.svelte';

// The whole fixture is the Svelte component: it creates the real
// SeatPanelState, renders the panel's plain option buttons, and runs
// Table.svelte's decode effect reactively so a test can drive the
// SSE-before-ARM race. See SeatPanelFollowUp.fixture.svelte for the body.
mount(SeatPanelFollowUpFixture, { target: document.getElementById('fixture')! });
