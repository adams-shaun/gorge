<script lang="ts">
  import type { BotPick } from '../../lib/pairedbot';
  import { pickTag } from '../../lib/pairedbot';

  /**
   * BotPickTag is the paired bot's mark on a control it would press: its
   * words and its estimate, quiet beside the control's own label. The
   * control still answers only when the player presses it.
   */
  let { pick }: { pick: BotPick } = $props();
</script>

<span class="bot-pick" data-bot-pick-tag title="The paired bot would choose this. Advice only: you still answer.">{pickTag(pick)}</span>

<style>
  .bot-pick {
    flex: none;
    align-self: center;
    font: 500 var(--t-11)/1.2 var(--font-ui);
    color: var(--gilt, #d4ad62);
    white-space: nowrap;
  }
</style>
