import { mount } from 'svelte';
import '../app.css';
import PromptFixture from './PromptSurface.fixture.svelte';
import { botPick } from '../lib/botpick.svelte';
import type { BotPick } from '../lib/pairedbot';

const requested = new URLSearchParams(location.search).get('case');
const cases = ['initiative', 'arrange', 'scry', 'surveil', 'seqswap', 'discard1', 'discard2', 'charm', 'attackers'] as const;
type FixtureCase = (typeof cases)[number];
const which: FixtureCase = cases.find((candidate) => candidate === requested) ?? 'initiative';
// ?botpick=<json> stands in for the paired bot's pick, as PairedBotPanel sets it.
const mark = new URLSearchParams(location.search).get('botpick');
if (mark !== null) botPick.set(JSON.parse(mark) as BotPick);
mount(PromptFixture, { target: document.querySelector('#fixture')!, props: { case: which } });
