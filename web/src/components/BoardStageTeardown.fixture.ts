import { mount } from 'svelte';
import '../app.css';
import BoardStageTeardownFixture from './BoardStageTeardown.fixture.svelte';

mount(BoardStageTeardownFixture, { target: document.getElementById('fixture')! });
