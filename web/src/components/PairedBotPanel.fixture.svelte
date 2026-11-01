<script lang="ts">
  import type { Decision } from '../protocol';
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
  (window as unknown as { __setSeq: (s: number) => void }).__setSeq = (s) => (seq = s);
</script>

<PairedBotPanel table="t1" match={1} ctx={{ seat: 0, token: 'tok' }} decision={decisionAt(seq)} />
