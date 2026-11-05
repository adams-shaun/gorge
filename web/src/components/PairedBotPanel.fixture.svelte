<script lang="ts">
  import { fetchPairedBotStatus } from '../lib/api';
  import type { PlayerAnswer } from '../lib/pairedbot';
  import type { Decision, PairedBotStatus } from '../protocol';
  import PairedBotPanel from './PairedBotPanel.svelte';

  const decisionAt = (seq: number): Decision =>
    ({
      seq, player: 0, kind: 'priority', prompt: '', min: 1, max: 1,
      options: [
        { index: 0, label: 'Pass priority', kind: 'pass' },
        { index: 1, label: `Cast Spell ${seq}`, kind: 'cast' },
      ],
    }) as Decision;
  let seq = $state(1);
  let answered = $state<PlayerAnswer | null>(null);
  const w = window as unknown as { __setSeq: (s: number) => void; __answer: (a: PlayerAnswer) => void; __differed: string[] };
  w.__setSeq = (s) => (seq = s);
  w.__answer = (a) => (answered = a);
  w.__differed = [];
  const ctx = { seat: 0, token: 'tok' };
  // The table route fetches the offer and hands it to the panel; so does this fixture.
  let status = $state<PairedBotStatus | null>(null);
  fetchPairedBotStatus('t1', ctx).then((s) => (status = s), () => (status = null));
</script>

<PairedBotPanel table="t1" match={1} {ctx} {status} decision={decisionAt(seq)} {answered} onDiffered={(t) => w.__differed.push(t)} />
