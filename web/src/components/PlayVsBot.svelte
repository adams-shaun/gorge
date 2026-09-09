<script lang="ts">
  import { VS_BOT_FORMATS, startPlayVsBot, type VsBotFormat } from '../lib/playvsbot';

  // Task ui11: the landing-page entry point that sits the player down 1v1
  // against a bot. It picks a format, POSTs /api/games, and follows the join
  // path the server returns — a full navigation, so main.ts's
  // initSeatContext re-reads ?seat=&token= and arms the seat (client-side
  // navigation reuses the already-initialised null seat and would stay a
  // spectator).
  let format: VsBotFormat = $state('constructed');
  let busy = $state(false);
  let error = $state<string | null>(null);

  async function start() {
    busy = true;
    error = null;
    try {
      const join = await startPlayVsBot(format);
      window.location.href = join;
    } catch (e) {
      error = e instanceof Error ? e.message : String(e);
      busy = false;
    }
  }
</script>

<div class="playvsbot" data-testid="playvsbot">
  <h2>Play 1v1 vs a bot</h2>
  <p class="hint">You take a seat; a bot takes the other. Decks are assigned at random; the match replays from its seed.</p>
  <div class="formats" role="radiogroup" aria-label="format">
    {#each VS_BOT_FORMATS as f (f.value)}
      <label class="format">
        <input type="radio" name="vsbot-format" value={f.value} checked={format === f.value} onchange={() => (format = f.value)} />
        <span>{f.label}</span>
      </label>
    {/each}
  </div>
  <button type="button" onclick={start} disabled={busy}>
    {busy ? 'Starting…' : 'Start game'}
  </button>
  {#if error}<p class="error">{error}</p>{/if}
</div>

<style>
  .playvsbot {
    display: flex;
    flex-direction: column;
    gap: var(--sp-2);
    padding: var(--sp-4);
    border: 1px solid var(--edge-inst);
    border-radius: var(--radius-md);
    background: var(--instrument);
    color: var(--ink-inst);
    max-width: 22rem;
    flex: none;
  }
  h2 {
    margin: 0;
    font-size: var(--t-16);
    font-weight: 600;
  }
  .hint {
    margin: 0;
    font-size: var(--t-12);
    color: var(--ink-dim);
  }
  .formats {
    display: flex;
    gap: var(--sp-3);
  }
  .format {
    display: flex;
    align-items: center;
    gap: var(--sp-1);
    font-size: var(--t-13);
  }
  button {
    align-self: flex-start;
    padding: var(--sp-2) var(--sp-3);
    border: 1px solid var(--edge-inst);
    border-radius: var(--radius-sm);
    background: var(--felt);
    color: var(--ink);
    font-size: var(--t-13);
    cursor: pointer;
  }
  button:disabled {
    opacity: 0.6;
    cursor: default;
  }
  .error {
    margin: 0;
    font-size: var(--t-12);
    color: var(--danger);
  }
</style>
