<script lang="ts">
  /**
   * The 17Lands credit: the bot drafters' pick ratings and
   * the builder's suggestion come from 17Lands public data, CC BY 4.0, which
   * requires attribution wherever that data shapes what the player sees.
   */
  let { what = 'Pick ratings' }: { what?: string } = $props();
</script>

<p class="ev-foot" data-testid="attribution">
  {what}: data from <a href="https://www.17lands.com/public_datasets" target="_blank" rel="noopener noreferrer">17Lands</a>,
  <a href="https://creativecommons.org/licenses/by/4.0/" target="_blank" rel="noopener noreferrer">CC BY 4.0</a>.
</p>
