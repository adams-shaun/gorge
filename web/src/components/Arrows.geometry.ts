import { mount } from 'svelte';
import '../app.css';
// The REAL route, not a shell replica (findings r2): the first cut of this
// fixture mounted Board/Rail/Arrows itself, so removing Table.svelte's
// production mount left the live table with no arrows at all while the pin
// still passed — the fixture was quietly supplying its own overlay. This
// fixture mounts the production route and NOTHING else: the only `.arrows`
// element the page can ever contain is the one Table.svelte mounts. The
// state arrives the way the live page gets it — a snapshot frame on the SSE
// stream — which the test intercepts and fulfils (Arrows.geometry.test.ts);
// the route needs no backend beyond that.
import Table from '../routes/Table.svelte';

mount(Table, { target: document.querySelector('#app')!, props: { table: 'arrows-fixture' } });
