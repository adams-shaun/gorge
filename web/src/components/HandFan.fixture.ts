import { mount } from 'svelte';
import '../app.css';
import HandFanFixture from './HandFan.fixture.svelte';

mount(HandFanFixture, { target: document.querySelector('#fixture')! });
