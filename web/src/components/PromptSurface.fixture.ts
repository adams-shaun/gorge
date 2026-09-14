import { mount } from 'svelte';
import '../app.css';
import PromptFixture from './PromptSurface.fixture.svelte';

const requested = new URLSearchParams(location.search).get('case');
const cases = ['initiative', 'arrange', 'scry', 'seqswap'] as const;
type FixtureCase = (typeof cases)[number];
const which: FixtureCase = cases.find((candidate) => candidate === requested) ?? 'initiative';
mount(PromptFixture, { target: document.querySelector('#fixture')!, props: { case: which } });
