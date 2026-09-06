import { mount } from 'svelte';
import './app.css';
import App from './App.svelte';
import { initSeatContext } from './lib/seat';

// The seat identity (?seat=N&token=…) is read once, before the app mounts,
// and kept for the session (M2e-4, R-E4-5).
initSeatContext(location.search);

const app = mount(App, { target: document.getElementById('app')! });

export default app;
