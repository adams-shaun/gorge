import { mount } from 'svelte';
import '../app.css';
import PendingTrayFixture from './PendingTray.fixture.svelte';

const requested = new URLSearchParams(location.search).get('case');
const cases = ['empty', 'stuck', 'stuck-unanswerable'] as const;
type FixtureCase = (typeof cases)[number];
const which: FixtureCase = cases.find((candidate) => candidate === requested) ?? 'empty';
mount(PendingTrayFixture, { target: document.querySelector('#fixture')!, props: { case: which } });
