<script lang="ts">
  import { onMount } from 'svelte';
  import { ApiError, checkDeck, fetchBotPolicies, fetchCapabilities, fetchDecks, type DeckInfo } from '../lib/api';
  import type { BotPolicyInfo, DeckReport, PairedBotChoice } from '../protocol';
  import DeckReportPanel from './DeckReportPanel.svelte';
  import ProtocolSetting from './ProtocolSetting.svelte';
  import { VS_BOT_FORMATS, VS_BOT_MULLIGANS, VS_BOT_SEATS, deckLabel, deckTextTooLong, startPlayVsBot, type VsBotFormat, type VsBotMulligans, type VsBotSeats } from '../lib/playvsbot';

  // The landing-page entry point that sits the player down against bots: a
  // duel, or (Commander only) a pod of three or four seats, one of them yours.
  // Deck ids come only from the server catalogue; empty values deliberately
  // stay the original random-assignment path. A pasted decklist replaces the
  // player's pool pick; the server alone judges it. The opponent picker lists
  // the server's offered policies for the chosen format; empty keeps its default.
  // A constant: eslint refuses a string literal inside the attribute's mustache.
  const DECK_TEXT_PLACEHOLDER = 'Commander\n1 Your Commander\n\n1 Sol Ring\n...';
  let format: VsBotFormat = $state('constructed');
  let humanDeck = $state('');
  let botDeck = $state('');
  let deckText = $state('');
  let botPolicy = $state('');
  let policies = $state<BotPolicyInfo[]>([]);
  let defaultPolicy = $state('');
  let pairedBot = $state('');
  let pairedBots = $state<PairedBotChoice[]>([]);
  let mulligans: VsBotMulligans = $state(6);
  let seats: VsBotSeats = $state(2);
  let decks = $state<DeckInfo[]>([]);
  let loadingDecks = $state(true);
  let busy = $state(false);
  let error = $state<string | null>(null);
  let report = $state<DeckReport | null>(null);
  let substitute = $state(false);
  let checking = $state(false);
  // Plain, not $state: the consent effect below must depend on substitute
  // alone, so a check landing (which moves these) never re-triggers it.
  let panelShown = false;
  let inflight = false;

  // runCheck posts the pasted list to the server's deck check and shows its
  // report verbatim. The server judges everything; this only sends the text,
  // the panel's format and the consent. A failed check shows the server's
  // message in the error line and clears the panel.
  async function runCheck() {
    const text = deckText.trim();
    if (text === '') return;
    inflight = true;
    checking = true;
    error = null;
    try {
      report = await checkDeck(text, format, substitute);
      panelShown = true;
    } catch (e) {
      error = e instanceof Error ? e.message : String(e);
      report = null;
      panelShown = false;
    } finally {
      inflight = false;
      checking = false;
    }
  }

  // Ticking or unticking the consent re-posts the check, so the panel shows
  // the server's new answer — either the swaps or the server's own refusal.
  $effect(() => {
    const consent = substitute;
    void consent;
    if (panelShown && !inflight) void runCheck();
  });
  const formatDecks = $derived(decks.filter((d) => d.format === format));
  const formatPolicies = $derived(policies.filter((p) => p.formats.includes(format)));
  const defaultLabel = $derived(policies.find((p) => p.name === defaultPolicy)?.label ?? defaultPolicy);
  const chosenPolicy = $derived(policies.find((p) => p.name === (botPolicy || defaultPolicy)));
  const formatPairedBots = $derived(pairedBots.filter((b) => !b.formats?.length || b.formats.includes(format)));

  onMount(() => {
    void loadDecks();
    void loadPolicies();
    void fetchCapabilities().then((c) => (pairedBots = c.paired_bots ?? [])).catch(() => {});
  });

  // A failed fetch leaves the picker at the server default: the game still
  // starts with the server's own policy.
  async function loadPolicies() {
    try {
      const list = await fetchBotPolicies();
      policies = list.policies;
      defaultPolicy = list.default;
    } catch {
      policies = [];
    }
  }

  function policyLabel(p: BotPolicyInfo): string {
    return p.tier && p.tier !== 'production' ? `${p.label} (${p.tier})` : p.label;
  }

  async function loadDecks() {
    try {
      decks = await fetchDecks();
    } catch (e) {
      error = e instanceof Error ? `Could not load decks: ${e.message}` : String(e);
    } finally {
      loadingDecks = false;
    }
  }

  function chooseFormat(next: VsBotFormat) {
    format = next;
    seats = next === 'commander' ? 4 : 2;
    humanDeck = '';
    botDeck = '';
    if (botPolicy && !formatPolicies.some((p) => p.name === botPolicy)) botPolicy = '';
    if (pairedBot !== 'off' && !formatPairedBots.some((b) => b.name === pairedBot)) pairedBot = '';
  }

  async function start() {
    if (deckTextTooLong(deckText)) {
      error = 'That list is too long to send.';
      return;
    }
    busy = true;
    error = null;
    try {
      const join = await startPlayVsBot(format, humanDeck, botDeck, mulligans, botPolicy, pairedBot, seats, deckText, substitute);
      window.location.href = join;
    } catch (e) {
      error = e instanceof Error ? e.message : String(e);
      // A refused deck carries the server's report: show it in the same
      // panel the check uses, keeping the server's message in the error line.
      if (e instanceof ApiError && e.deck) {
        report = e.deck;
        panelShown = true;
      }
      busy = false;
    }
  }
</script>

<div class="playvsbot" data-testid="playvsbot">
  <h2>Play vs bots</h2>
  <p class="hint">
    {seats === 2 ? 'Two seats: you and one bot.' : `${seats} seats: you and ${seats - 1} bots.`} Choose the matchup, or leave a seat random. The match replays
    from its seed.
  </p>
  <div class="formats" role="radiogroup" aria-label="format">
    {#each VS_BOT_FORMATS as f (f.value)}
      <label class="format">
        <input type="radio" name="vsbot-format" value={f.value} checked={format === f.value} onchange={() => chooseFormat(f.value)} />
        <span>{f.label}</span>
      </label>
    {/each}
  </div>
  {#if format === 'commander'}
    <label class="mulligans">
      <span>Seats</span>
      <select aria-label="Seats" bind:value={seats} data-testid="seats">
        {#each VS_BOT_SEATS as n (n)}
          <option value={n}>{n}</option>
        {/each}
      </select>
    </label>
  {/if}
  <label class="mulligans">
    <span>Mulligans</span>
    <select aria-label="Mulligans" bind:value={mulligans} data-testid="mulligans">
      {#each VS_BOT_MULLIGANS as n (n)}
        <option value={n}>{n}</option>
      {/each}
    </select>
    <span class="mulligan-hint">each redraw adds one card you put on the bottom</span>
  </label>
  <div class="pickers" aria-busy={loadingDecks}>
    {#if seats === 2}
      <label class="picker">
        <span>Your deck</span>
        <select aria-label="Your deck" bind:value={humanDeck} disabled={deckText.trim() !== ''} data-testid="human-deck">
          <option value="">Random</option>
          {#each formatDecks as d (d.id)}
            <option value={d.id} disabled={!!d.refused}>{deckLabel(d)}</option>
          {/each}
        </select>
      </label>
      <label class="picker">
        <span>Bot deck</span>
        <select aria-label="Bot deck" bind:value={botDeck} data-testid="bot-deck">
          <option value="">Random</option>
          {#each formatDecks as d (d.id)}
            <option value={d.id} disabled={!!d.refused}>{deckLabel(d)}</option>
          {/each}
        </select>
      </label>
    {:else}
      <p class="hint">Pod decks are dealt from the server's Commander pool.</p>
    {/if}
    <label class="picker">
      <span>Paste your deck</span>
      <textarea
        aria-label="Paste your deck"
        rows="12"
        placeholder={DECK_TEXT_PLACEHOLDER}
        bind:value={deckText}
        data-testid="deck-text"
      ></textarea>
    </label>
    <button type="button" class="check" onclick={() => void runCheck()} disabled={deckText.trim() === '' || checking} data-testid="deck-check">
      {checking ? 'Checking…' : 'Check deck'}
    </button>
    <DeckReportPanel report={report} bind:substitute />
    <label class="picker">
      <span>Bot</span>
      <select aria-label="Bot" bind:value={botPolicy} data-testid="bot-policy">
        <option value="">{defaultLabel ? `Server default (${defaultLabel})` : 'Server default'}</option>
        {#each formatPolicies as p (p.name)}
          <option value={p.name}>{policyLabel(p)}</option>
        {/each}
      </select>
      {#if chosenPolicy?.description}<span class="policy-hint">{chosenPolicy.description}</span>{/if}
    </label>
    {#if formatPairedBots.length > 0}
      <label class="picker">
        <span>Paired bot</span>
        <select aria-label="Paired bot" bind:value={pairedBot} data-testid="paired-bot">
          <option value="">Server default: {pairedBots[0].label}</option>
          {#each formatPairedBots.filter((b) => b.name !== pairedBots[0].name) as b (b.name)}
            <option value={b.name}>{b.label}</option>
          {/each}
          <option value="off">None</option>
        </select>
        <span class="policy-hint">Shows you what its search makes of your decisions, if you turn it on in the game.</span>
      </label>
    {/if}
  </div>
  <ProtocolSetting />
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
    border-radius: var(--radius);
    background: var(--instrument);
    color: var(--ink-inst);
    max-width: 32rem;
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
  .mulligans {
    display: flex;
    align-items: center;
    gap: var(--sp-2);
    font-size: var(--t-12);
    color: var(--ink-dim);
  }
  .mulligans select {
    width: auto;
    padding: var(--sp-1) var(--sp-2);
    border: 1px solid var(--edge-inst);
    border-radius: var(--radius);
    background: var(--instrument-raised);
    color: var(--ink-inst);
    font: var(--t-14) var(--font-ui);
  }
  .mulligan-hint {
    color: var(--ink-dim);
  }
  .format {
    display: flex;
    align-items: center;
    gap: var(--sp-1);
    font-size: var(--t-14);
  }
  .pickers {
    display: grid;
    gap: var(--sp-2);
  }
  .picker {
    display: grid;
    gap: var(--sp-1);
    font-size: var(--t-12);
    color: var(--ink-dim);
  }
  .policy-hint {
    color: var(--ink-dim);
  }
  select,
  textarea {
    width: 100%;
    padding: var(--sp-2);
    border: 1px solid var(--edge-inst);
    border-radius: var(--radius);
    background: var(--instrument-raised);
    color: var(--ink-inst);
    font: var(--t-14) var(--font-ui);
  }
  button {
    align-self: flex-start;
    padding: var(--sp-2) var(--sp-3);
    border: 1px solid var(--edge-inst);
    border-radius: var(--radius);
    background: var(--felt);
    color: var(--ink);
    font-size: var(--t-14);
    cursor: pointer;
  }
  textarea {
    box-sizing: border-box;
    resize: vertical;
  }
  button:disabled {
    opacity: 0.6;
    cursor: default;
  }
  .error {
    margin: 0;
    font-size: var(--t-12);
    color: var(--danger);
    white-space: pre-wrap;
  }
</style>
