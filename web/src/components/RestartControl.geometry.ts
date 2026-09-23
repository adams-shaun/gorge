import { mount } from 'svelte';
import '../app.css';
import RestartControlFixture from './RestartControlFixture.svelte';

// The browser interaction fixture for the Restart control's arm/confirm
// two-step. It mounts the REAL component (through a thin state-owning
// wrapper, exactly as Table.svelte hosts it) so a test can click both
// buttons and read the callback.
mount(RestartControlFixture, { target: document.querySelector('#restart')! });
