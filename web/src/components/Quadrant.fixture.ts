import { mount } from 'svelte';
import '../app.css';
import QuadrantFixture from './Quadrant.fixture.svelte';

mount(QuadrantFixture, { target: document.querySelector('#fixture')! });
