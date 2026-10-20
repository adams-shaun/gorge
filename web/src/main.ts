import { mount } from 'svelte';
import './app.css';
import App from './App.svelte';
import { initSeatContext } from './lib/seat';

// The seat identity (?seat=N&token=…) is read once, before the app mounts,
// and kept for the session (M2e-4, R-E4-5). The pathname lets a bare match
// replay link restore the table's remembered seat after a reload or new tab.
initSeatContext(location.search, location.pathname);

const app = mount(App, { target: document.getElementById('app')! });

export default app;
