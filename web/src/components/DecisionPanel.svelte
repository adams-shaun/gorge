<script lang="ts">
  import type { ReviewDecision, SeatInfo } from '../protocol';
  import { fetchDecisions } from '../lib/api';
  import { seatColour } from '../lib/colours';
  import { pointerRelease } from '../lib/pointer';
  import {
    duplicateLabels,
    entryIndexAt,
    hasExplain,
    isFallback,
    isOverride,
    isSearched,
    labelOf,
    nextEntry,
    note,
    signed,
    sourceBadge,
  } from '../lib/review';

  /**
   * DecisionPanel is the match reviewer's rail section (design T5), mounted
   * only for a review table (`gorged review`). It fetches the match's
   * decision index once and shows the decision at the DVR cursor: who was
   * asked, the kind and prompt, every option offered with the chosen one
   * marked, and — when the record carries explain notes — who decided (the
   * source badge), the bot's own answer and the search table. Between two
   * decisions it keeps the last one, dimmed, so the rail never flickers
   * empty while playing. The o/f/d keys jump to the next override,
   * fallback or searched decision (Shift: previous) through onScrub.
   */
  let {
    table,
    match,
    seats = [],
    cursor,
    onScrub,
    entries = $bindable<ReviewDecision[]>([]),
  }: {
    table: string;
    match: number;
    seats?: SeatInfo[];
    cursor: number;
    onScrub: (seq: number) => void;
    entries?: ReviewDecision[];
  } = $props();

  let error = $state<string | null>(null);
  let loaded = $state(false);

  $effect(() => {
    const t = table;
    const k = match;
    let live = true;
    loaded = false;
    error = null;
    fetchDecisions(t, k)
      .then((ds) => {
        if (!live) return;
        entries = ds;
        loaded = true;
      })
      .catch((e: unknown) => {
        if (!live) return;
        error = e instanceof Error ? e.message : String(e);
      });
    return () => {
      live = false;
    };
  });

  const at = $derived(entryIndexAt(entries, cursor));
  const e = $derived(at < 0 ? null : entries[at]);
  const exact = $derived(e !== null && e.ask_seq === cursor);
  const n = $derived(note(e));
  const explained = $derived(hasExplain(entries));
  const chosen = $derived(new Set(e?.chosen ?? []));
  const bot = $derived(new Set(n?.bot ?? []));
  const override = $derived(e !== null && isOverride(e));
  const badge = $derived(n ? sourceBadge(n) : null);
  const rows = $derived(n?.search?.rows ?? []);
  const maxVisits = $derived(Math.max(1, ...rows.map((r) => r.visits)));
  const seatName = (p: number) => seats[p]?.name ?? `Seat ${p + 1}`;
  // Two options can share a label (two copies of a card in hand): a search
  // row then names its option index too, as the option list does.
  const dupLabels = $derived(duplicateLabels(e));
  function candidate(choices: number[]): string {
    if (!e) return '';
    if (choices.length === 1) {
      const l = labelOf(e, choices);
      return dupLabels.has(l) ? `${l} (${choices[0]})` : l;
    }
    return labelOf(e, choices);
  }
  const kindLabel = (k: string) => k.replace(/_/g, ' ');
  const counts = $derived({
    override: entries.filter(isOverride).length,
    fallback: entries.filter(isFallback).length,
    searched: entries.filter(isSearched).length,
  });

  const JUMPS = {
    o: { pred: isOverride, name: 'override' },
    f: { pred: isFallback, name: 'fallback' },
    d: { pred: isSearched, name: 'searched decision' },
  } as const;
  type JumpKey = keyof typeof JUMPS;
  let missed = $state<string | null>(null);

  function jump(k: JumpKey, dir: 1 | -1) {
    const hit = nextEntry(entries, cursor, dir, JUMPS[k].pred);
    if (hit) {
      missed = null;
      onScrub(hit.ask_seq);
    } else {
      missed = `No ${dir > 0 ? 'later' : 'earlier'} ${JUMPS[k].name}`;
    }
  }

  // The review jump keys: o/O, f/F, d/D. Bare letters the table keymap
  // leaves unbound (its O and F bindings are Ctrl+Shift). Ignored while
  // typing, in a select, or with a command modifier; the transport keeps
  // Space/K and the arrows.
  function onWindowKey(ev: KeyboardEvent) {
    if (ev.ctrlKey || ev.metaKey || ev.altKey || ev.defaultPrevented || ev.repeat) return;
    const t = ev.target as HTMLElement | null;
    if (t && (t.isContentEditable || /^(INPUT|TEXTAREA|SELECT)$/.test(t.tagName))) return;
    const k = ev.key.toLowerCase();
    if (k !== 'o' && k !== 'f' && k !== 'd') return;
    if (!explained) return;
    ev.preventDefault();
    jump(k, ev.shiftKey ? -1 : 1);
  }
  $effect(() => {
    void cursor;
    missed = null;
  });
</script>

<svelte:window onkeydown={onWindowKey} />

<section class="review" aria-label="Decision" data-review-panel data-intent={e?.intent ?? ''} data-exact={exact}>
  {#if error}
    <p class="quiet">Decisions did not load: {error}</p>
  {:else if !loaded}
    <p class="quiet">Loading decisions…</p>
  {:else if entries.length === 0}
    <p class="quiet">This record has no decisions.</p>
  {:else if e === null}
    <p class="quiet">Setup. The first decision is at event {entries[0].ask_seq}.</p>
  {:else}
    <div class="entry" class:past={!exact}>
      <header>
        <h3 title="The intent index: ?intent={e.intent} and repro -at {e.intent} open this decision">
          Decision {e.intent}
          <span class="of">of {entries.length - 1}</span>
        </h3>
        {#if badge}
          <span class="badge tone-{badge.tone}" title={badge.tip} data-source={n?.source}>{badge.text}</span>
        {/if}
      </header>
      <p class="who" style:--seat={seatColour(e.seat, seats)}>
        <span class="name">{seatName(e.seat)}</span>
        <span class="kind">{kindLabel(e.kind)}</span>
        {#if override}<span class="flag" title="The seat chose differently from its own heuristic bot">overrode the bot</span>{/if}
      </p>
      {#if e.prompt}<p class="prompt">{e.prompt}</p>{/if}

      <ol class="options" aria-label="Options offered">
        {#each e.options as o (o.index)}
          <li class:chosen={chosen.has(o.index)} class:bot={bot.has(o.index)} data-option={o.index}>
            <span class="ix" title="Option index">{o.index}</span>
            <span class="label">{o.label}</span>
            {#if chosen.has(o.index)}<span class="tag tag-chosen">chosen</span>{/if}
            {#if bot.has(o.index) && override}<span class="tag tag-bot">bot</span>{/if}
          </li>
        {/each}
      </ol>
      {#if e.options_total > e.options.length}
        <p class="more">{e.options.length} of {e.options_total} options shown</p>
      {/if}
      {#if e.chosen.length !== 1 || e.answer?.startsWith('cast with') || e.answer?.startsWith('announce')}
        <p class="answer"><span class="k">Answer</span> {e.answer}</p>
      {/if}
      {#if n?.reason}
        <p class="answer"><span class="k">Reason</span> {n.reason}</p>
      {/if}

      {#if n?.search}
        {@const s = n.search}
        <table class="search" aria-label="Search">
          <thead>
            <tr><th class="c-ans">Candidate</th><th class="c-vis">Visits</th><th class="num c-prior">Prior</th><th class="num">Q</th></tr>
          </thead>
          <tbody>
            {#each rows as r, i (i)}
              <tr class:chosen={r.chosen} class:bot={r.bot}>
                <td class="c-ans" title={candidate(r.choices)}>
                  <span class="ans"><span class="cand">{candidate(r.choices)}</span>{#if r.chosen}<span class="tag tag-chosen">chosen</span>{/if}{#if r.bot}<span class="tag tag-bot">bot</span>{/if}</span>
                </td>
                <td class="c-vis">
                  <span class="vis"><span class="track"><span class="bar" style:width={`${(r.visits / maxVisits) * 100}%`}></span></span><span class="vn">{r.visits}</span></span>
                </td>
                <td class="num c-prior">{r.prior.toFixed(2)}</td>
                <td class="num" class:neg={r.q < 0}>{signed(r.q)}</td>
              </tr>
            {/each}
          </tbody>
        </table>
        <p class="foot">
          <span>Root <b class:neg={s.root_value < 0}>{signed(s.root_value)}</b></span>
          {#if s.sims}<span>{s.sims} sims</span>{/if}
          {#if s.ms}<span>{Math.round(s.ms)} ms</span>{/if}
          <span>{s.engine}{s.cheap ? ', cheap' : ''}</span>
          {#if s.dropped}<span>{s.dropped} more not listed</span>{/if}
        </p>
      {:else if n?.net}
        <p class="foot"><span>Net value <b class:neg={n.net.value < 0}>{signed(n.net.value)}</b></span></p>
      {/if}
    </div>
  {/if}

  {#if loaded && entries.length > 0}
    {#if explained}
      <div class="jumps" role="group" aria-label="Jump to">
        <button type="button" onclick={() => jump('o', 1)} title="Next override (o); previous (Shift+O)" use:pointerRelease disabled={counts.override === 0}>
          <kbd>o</kbd> override <span class="n">{counts.override}</span>
        </button>
        <button type="button" onclick={() => jump('f', 1)} title="Next fallback (f); previous (Shift+F)" use:pointerRelease disabled={counts.fallback === 0}>
          <kbd>f</kbd> fallback <span class="n">{counts.fallback}</span>
        </button>
        <button type="button" onclick={() => jump('d', 1)} title="Next searched decision (d); previous (Shift+D)" use:pointerRelease disabled={counts.searched === 0}>
          <kbd>d</kbd> searched <span class="n">{counts.searched}</span>
        </button>
      </div>
      {#if missed}<p class="quiet missed" role="status">{missed}</p>{/if}
    {:else}
      <p class="quiet">No explain recorded</p>
    {/if}
  {/if}
</section>

<style>
  /* Instrument register, like the rail around it: hairlines, no boxes. The
     one loud thing is the chosen row's gilt rule; the bot's alternative is
     verdigris, a fallback ember. */
  .review {
    container-type: inline-size;
    flex: 0 1 auto;
    max-height: 55%;
    overflow-y: auto;
    padding: var(--sp-2) var(--sp-3) var(--sp-3);
    border-bottom: 1px solid var(--edge-inst);
    background: var(--instrument);
    color: var(--ink-inst);
    font-family: var(--font-ui);
    font-size: var(--t-12);
  }
  .entry { transition: opacity 0.12s ease-out; }
  .entry.past { opacity: 0.72; }
  @media (prefers-reduced-motion: reduce) { .entry { transition: none; } }

  header { display: flex; align-items: baseline; justify-content: space-between; gap: var(--sp-2); }
  h3 {
    margin: 0;
    font-family: var(--font-serif);
    font-size: var(--t-20);
    font-weight: 600;
    color: var(--ink);
    line-height: 1.15;
  }
  .of { font-family: var(--font-ui); font-size: var(--t-12); font-weight: 400; color: var(--ink-faint); margin-left: var(--sp-1); }

  .badge {
    flex: none;
    padding: 1px var(--sp-2);
    border: 1px solid var(--edge-inst);
    border-radius: 999px;
    font-size: var(--t-11);
    cursor: help;
    white-space: nowrap;
  }
  .tone-policy { color: var(--gilt); border-color: color-mix(in srgb, var(--gilt) 55%, transparent); }
  .tone-plain { color: var(--ink-dim); }
  .tone-fallback { color: var(--ember); border-color: color-mix(in srgb, var(--ember) 60%, transparent); }

  .who {
    display: flex;
    flex-wrap: wrap;
    align-items: baseline;
    gap: var(--sp-1) var(--sp-2);
    margin: var(--sp-1) 0 0;
    padding-left: var(--sp-2);
    border-left: 3px solid var(--seat);
  }
  .name { color: var(--ink); font-weight: 600; }
  .kind { color: var(--ink-dim); }
  .flag { color: var(--verdigris); }
  .prompt { margin: 2px 0 0; color: var(--ink-faint); overflow-wrap: anywhere; }

  .options { list-style: none; margin: var(--sp-2) 0 0; padding: 0; }
  .options li {
    display: flex;
    flex-wrap: wrap;
    align-items: baseline;
    gap: var(--sp-2);
    padding: 3px var(--sp-1) 3px var(--sp-2);
    border-top: 1px solid color-mix(in srgb, var(--edge-inst) 55%, transparent);
  }
  .options li:first-child { border-top: 0; }
  .options li.chosen { box-shadow: inset 3px 0 0 var(--gilt); }
  .options li.bot:not(.chosen) { box-shadow: inset 3px 0 0 var(--verdigris); }
  .options li.chosen .label { color: var(--ink); }
  .label { flex: 1 1 7rem; min-width: 0; overflow-wrap: break-word; }
  .ix { flex: none; min-width: 1.2em; text-align: right; color: var(--ink-faint); font-variant-numeric: tabular-nums; }
  .tag { flex: none; font-size: var(--t-11); }
  .tag-chosen { color: var(--gilt); }
  .tag-bot { color: var(--verdigris); }
  .more, .answer { margin: var(--sp-1) 0 0; color: var(--ink-faint); overflow-wrap: anywhere; }
  .answer .k { color: var(--ink-dim); margin-right: var(--sp-1); }

  .search { width: 100%; margin-top: var(--sp-3); border-collapse: collapse; table-layout: fixed; font-variant-numeric: tabular-nums; }
  .search th {
    text-align: left;
    font-weight: 400;
    color: var(--ink-faint);
    padding: 0 var(--sp-1) 2px 0;
    border-bottom: 1px solid var(--edge-inst);
  }
  .search td { padding: 3px var(--sp-1) 3px 0; vertical-align: baseline; border-bottom: 1px solid color-mix(in srgb, var(--edge-inst) 45%, transparent); }
  .search .c-ans { width: 46%; }
  .search .c-vis { width: 28%; }
  .search .num { width: 13%; text-align: right; }
  .ans { display: flex; align-items: baseline; gap: var(--sp-1); min-width: 0; }
  .cand { flex: 0 1 auto; min-width: 0; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
  .search tr.chosen .cand { color: var(--ink); }
  .vis { display: flex; align-items: center; gap: var(--sp-1); }
  .track { flex: 1; min-width: 0; height: 0.5rem; }
  .bar {
    display: block;
    height: 100%;
    min-width: 1px;
    border-radius: 1px;
    background: color-mix(in srgb, var(--ink-inst) 30%, transparent);
  }
  .search tr.chosen .bar { background: color-mix(in srgb, var(--gilt) 70%, transparent); }
  .search tr.bot:not(.chosen) .bar { background: color-mix(in srgb, var(--verdigris) 60%, transparent); }
  .vn { flex: none; min-width: 2.2em; text-align: right; color: var(--ink-inst); }
  .neg { color: var(--ink-dim); }
  .foot { display: flex; flex-wrap: wrap; gap: var(--sp-1) var(--sp-3); margin: var(--sp-2) 0 0; color: var(--ink-faint); }
  .foot b { font-weight: 600; color: var(--ink); }
  .foot b.neg { color: var(--ink-dim); }

  .jumps { display: flex; flex-wrap: wrap; gap: var(--sp-1); margin-top: var(--sp-3); }
  .jumps button {
    font: inherit;
    font-size: var(--t-11);
    color: var(--ink-inst);
    background: transparent;
    border: 1px solid var(--edge-inst);
    border-radius: var(--radius);
    padding: 2px var(--sp-2);
    cursor: pointer;
  }
  .jumps button:hover:not(:disabled) { background: var(--instrument-raised); }
  .jumps button:disabled { opacity: 0.45; cursor: default; }
  .jumps button:focus-visible { outline: 2px solid var(--gilt); outline-offset: 2px; }
  kbd {
    font-family: var(--font-ui);
    font-size: var(--t-11);
    padding: 0 4px;
    border: 1px solid var(--edge-inst);
    border-radius: 2px;
    color: var(--ink);
  }
  .n { color: var(--ink-faint); margin-left: 2px; }
  .quiet { margin: var(--sp-1) 0 0; color: var(--ink-faint); }
  /* A narrow rail keeps the candidate, its visits and Q; the prior goes. */
  @container (max-width: 17rem) {
    .search .c-prior { display: none; }
    .search .c-ans { width: 52%; }
    .search .c-vis { width: 30%; }
    .search .num { width: 18%; }
  }
  /* A sliver of a rail: the candidate and its Q only. */
  @container (max-width: 11rem) {
    .search .c-vis, .search .c-prior { display: none; }
    .search .c-ans { width: 65%; }
    .search .num { width: 35%; }
    .options .tag { flex-basis: 100%; }
    .search .tag { display: none; }
    .search tr.chosen td:first-child { box-shadow: inset 2px 0 0 var(--gilt); padding-left: var(--sp-1); }
    .search tr.bot:not(.chosen) td:first-child { box-shadow: inset 2px 0 0 var(--verdigris); padding-left: var(--sp-1); }
  }
  .missed { color: var(--ink-dim); }
</style>
