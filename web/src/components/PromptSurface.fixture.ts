import { mount } from 'svelte';
import '../app.css';
import PromptFixture from './PromptSurface.fixture.svelte';

const which = new URLSearchParams(location.search).get('case') ?? 'initiative';
mount(PromptFixture, { target: document.querySelector('#fixture')!, props: { case: which } });
