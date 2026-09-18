<script lang="ts">
  import type { Option } from '../protocol';
  import type { TileOptions } from '../lib/cardoptions';
  import { ACTION_GLYPHS, actionAccessibleLabel, postSingleAction, postTileOption, singleActionIcon, singleTapOptionOf, tileScenario } from '../lib/cardoptions';
  import {
    placeMenu,
    placeRadial,
    MENU_WIDTH,
    RADIAL_BUTTON,
    type MenuAnchor,
  } from '../lib/menuplacement';

  /**
   * The single card/commander/identity options affordance. One option acts
   * directly, two through six open the compact radial picker, and larger
   * sets retain the existing rectangular list menu.
   */
  let {
    tileOptions,
    subject,
    open0 = false,
    collapseTapActions = false,
  }: {
    tileOptions: TileOptions;
    subject: string;
    open0?: boolean;
    collapseTapActions?: boolean;
  } = $props();

  // svelte-ignore state_referenced_locally
  let open = $state(open0);
  let anchor = $state<MenuAnchor | null>(null);
  let badgeEl = $state<HTMLButtonElement | null>(null);
  const tapAction = $derived(collapseTapActions ? singleTapOptionOf(tileOptions) : null);
  const radial = $derived(tileOptions.list.length <= 6);

  function captureAnchor(): void {
    if (!badgeEl) return;
    const r = badgeEl.getBoundingClientRect();
    anchor = { left: r.left, top: r.top, right: r.right, bottom: r.bottom };
  }

  function toggle(): void {
    open = !open;
    if (open) captureAnchor();
  }

  // A direct card action can produce a second, object-bound decision (for
  // example Underground Sea's source-level activation followed by Add U / Add
  // B). The first picker is destroyed while that intent is in flight, so the
  // parent marks the one immediate continuation that should arrive open.
  $effect(() => {
    if (!tileOptions.autoOpen) return;
    open = true;
    captureAnchor();
  });

  function close(): void {
    open = false;
  }

  /** choose posts one option; Ctrl held (holdPriority) skips pass-after-acting for this one cast/ability (prio3). */
  function choose(option: Option, holdPriority = false): void {
    close();
    postTileOption(tileOptions, option, holdPriority);
  }

  function onWindowKeydown(event: KeyboardEvent): void {
    if (event.key === 'Escape') close();
  }

  const viewportWidth = () => typeof window === 'undefined' ? 0 : window.innerWidth;
  const viewportHeight = () => typeof window === 'undefined' ? 0 : window.innerHeight;
  const menuPlacement = $derived(
    anchor
      ? placeMenu(anchor, viewportWidth(), viewportHeight())
      : { x: 8, y: 8, maxHeight: 400, up: false },
  );
  const radialPlacement = $derived(
    anchor
      ? placeRadial(anchor, tileOptions.list.length, viewportWidth(), viewportHeight())
      : [],
  );

  // The count badge's scenario: the icon+noun every option in the list agrees
  // on, or null for a mixed list (the bare neutral count — the rule never
  // claims a scenario the list does not have).
  const scenario = $derived(tileScenario(tileOptions));

  const MANA_LABEL = /^Add ([WUBRGC])$/i;
  const manaSymbols = $derived(tileOptions.list.map((option) => option.label.match(MANA_LABEL)?.[1]?.toUpperCase() ?? null));
  const isManaChoice = $derived(manaSymbols.length > 0 && manaSymbols.every((symbol) => symbol !== null));

  function compactLabel(label: string): string {
    const word = label.trim().split(/\s+/)[0] ?? label;
    return word.length > 8 ? `${word.slice(0, 7)}…` : word;
  }

  /** Portal follows the existing list menu mechanism: fixed coordinates in
   * the viewport, outside every scroll container that could clip it. */
  function portal(node: HTMLElement) {
    document.body.appendChild(node);
    return { destroy: () => node.remove() };
  }

  /** Immediate help-bubble placement (fb-20260917T232800Z). The bubble sits
   * in the same portaled radial layer as its button — a tooltip child of the
   * button would be clipped by .wheel-button's overflow: hidden — anchored to
   * the button's own radial coordinates: centred above it when there is room,
   * flipped below against the viewport's top edge. The horizontal anchor is
   * clamped half a max bubble width from each edge so a wide label on an
   * edge button never runs off screen (it may sit slightly off-centre then,
   * the cheap price of not measuring text). */
  const TIP_GAP = 8;
  const TIP_ROOM = 40; // bubble line height + gap: point.y below this flips the bubble below the button
  const TIP_HALF = 88; // half of the bubble's 176px max-width
  function tipAnchorX(x: number): number {
    return Math.max(TIP_HALF, Math.min(x + RADIAL_BUTTON / 2, viewportWidth() - TIP_HALF));
  }
</script>

<svelte:window onkeydown={onWindowKeydown} onclick={close} />

<div class="tile-actions">
  {#if tileOptions.list.length === 1 || tapAction !== null}
    {@const action = tapAction ?? tileOptions.list[0]}
    {@const icon = singleActionIcon(action)}
    <button
      class="action-icon badge--{tileOptions.tone}"
      class:selected={tileOptions.pickedOrder.length > 0}
      type="button"
      data-single-action
      data-action-icon={icon}
      aria-label={actionAccessibleLabel(action)}
      title={actionAccessibleLabel(action)}
      onclick={(event) => {
        event.stopPropagation();
        postSingleAction(tileOptions, true, event.ctrlKey);
      }}
    >
      <span aria-hidden="true">{ACTION_GLYPHS[icon]}</span>
    </button>
  {:else}
    <button
      class="badge badge--{tileOptions.tone}"
      class:selected={tileOptions.pickedOrder.length > 0}
      type="button"
      aria-haspopup="menu"
      aria-expanded={open}
      aria-label={scenario
        ? `${tileOptions.list.length} ${scenario.noun} ${subject}`
        : `${tileOptions.list.length} actions ${subject}`}
      title="Options {subject}"
      data-action-icon={scenario?.icon}
      bind:this={badgeEl}
      onclick={(event) => {
        event.stopPropagation();
        toggle();
      }}
    >
      {#if scenario}
        <span class="badge__icon" aria-hidden="true">{ACTION_GLYPHS[scenario.icon]}</span>
      {/if}
      <span class="badge__n data">{tileOptions.list.length}</span>
    </button>
  {/if}

  {#if tileOptions.pickedOrder.length > 0}
    <span class="sel data" aria-label="picked {tileOptions.pickedOrder.join(', ')}">{tileOptions.pickedOrder.join(',')}</span>
  {/if}

  {#if open && tileOptions.list.length > 1 && tapAction === null}
    {#if radial}
      <div class="radial-pop" data-option-picker data-radial-picker role="menu" tabindex="-1" aria-label="Options {subject}" use:portal>
        {#each tileOptions.list as opt, i (opt.index)}
          {@const point = radialPlacement[i] ?? { x: 8, y: 8 }}
          {@const mana = manaSymbols[i]}
          {@const tipAbove = point.y >= TIP_ROOM}
          <div class="wheel-slot">
            <button
              class="wheel-button badge--{tileOptions.tone}"
              class:wheel-button--mana={isManaChoice}
              type="button"
              role="menuitem"
              data-wire-index={opt.index}
              data-mana-option={isManaChoice ? mana : undefined}
              aria-label={opt.label}
              style:left="{point.x}px"
              style:top="{point.y}px"
              style:--pip={isManaChoice && mana ? `var(--mana-${mana.toLowerCase()})` : undefined}
              onclick={(event) => choose(opt, event.ctrlKey)}
            >
              {#if isManaChoice}<span aria-hidden="true">{mana}</span>{:else}<span>{compactLabel(opt.label)}</span>{/if}
            </button>
            <!-- Immediate help bubble (fb-20260917T232800Z): the option's own
                 label, revealed the moment the pointer enters or the button
                 takes keyboard focus — no dwell delay, no native title (the
                 ~1s browser tooltip this feedback replaced). Sibling of the
                 button inside the portaled radial layer, so overflow: hidden
                 cannot clip it; pointer-events: none so it never blocks a
                 click on the wheel. -->
            <span
              class="wheel-tip"
              class:wheel-tip--below={!tipAbove}
              role="tooltip"
              style:left="{tipAnchorX(point.x)}px"
              style:top="{tipAbove ? point.y : point.y + RADIAL_BUTTON + TIP_GAP}px"
              style:transform={tipAbove ? 'translate(-50%, calc(-100% - 8px))' : 'translate(-50%, 0)'}
            >{opt.label}</span>
          </div>
        {/each}
      </div>
    {:else}
      <!-- The list and wheel share data-option-picker: both are portaled,
           decision-blocking surfaces, so document hotkeys stay behind either
           shape. data-radial-picker remains the wheel-specific e2e hook. -->
      <div class="menu-pop" data-option-picker use:portal style:left="{menuPlacement.x}px" style:top="{menuPlacement.y}px" style:width="{MENU_WIDTH}px" style:max-height="{menuPlacement.maxHeight}px">
        <ul class="menu" role="menu" aria-label="Options {subject}">
          {#each tileOptions.list as opt (opt.index)}
            <li role="none">
              <button class="menu__item" type="button" role="menuitem" onclick={(event) => choose(opt, event.ctrlKey)}>
                {opt.label}
              </button>
            </li>
          {/each}
        </ul>
      </div>
    {/if}
  {/if}
</div>

<style>
  .tile-actions {
    position: absolute;
    top: 1px;
    right: 1px;
    z-index: 3;
    display: flex;
    flex-direction: column;
    align-items: flex-end;
    gap: 2px;
    line-height: 1;
  }
  .badge,
  .action-icon {
    display: inline-flex;
    align-items: center;
    justify-content: center;
    gap: 2px;
    /* 1.1rem + 25% (task fb-9946410e): the badge box grew, the font went up
       one token step (t-10 → t-12, the scale's nearest step to +25%). */
    min-width: 1.375rem;
    height: 1.375rem;
    padding: 0 0.25rem;
    border-radius: 3px;
    border: var(--edge-w, 1px) solid var(--edge-inst);
    background: var(--instrument);
    color: var(--ink);
    font-family: var(--font-data);
    font-size: var(--t-12);
    font-weight: 600;
    cursor: pointer;
  }
  .badge--initiative { background: var(--initiative); border-color: var(--initiative); color: var(--felt-sunk); }
  .badge--offered { background: var(--offered); border-color: var(--offered); color: var(--felt-sunk); }
  .badge.selected,
  .action-icon.selected { outline: 2px solid var(--ink); outline-offset: 1px; }
  /* 1.35rem + 25%; the icon font has no single token at +25% of t-14, so the
     token itself is scaled. */
  .action-icon { width: 1.6875rem; padding: 0; font-size: calc(var(--t-14) * 1.25); line-height: 1; }
  .badge__n { font-size: inherit; }
  .badge:hover,
  .badge[aria-expanded='true'],
  .action-icon:hover { border-color: var(--ink-dim); color: var(--ink); }
  .sel {
    display: inline-flex;
    align-items: center;
    justify-content: center;
    min-width: 1rem;
    height: 1rem;
    padding: 0 0.2rem;
    border-radius: 2px;
    background: var(--ink);
    color: var(--felt-sunk);
    font-size: var(--t-10);
    font-weight: 600;
  }

  /* The portal itself has no box: each 42px control is positioned around the
     badge centre by placeRadial, making the empty middle read as a wheel. */
  .radial-pop { position: fixed; inset: 0; z-index: 20; pointer-events: none; }
  /* One slot per option: the button plus its help bubble. The slot is an
     unpositioned, zero-size grouping so the button's own fixed coordinates
     are untouched; :hover and :focus-within on the slot are what reveal the
     bubble (an ancestor of the hovered/focused button is in the hover chain
     even at zero size, so no JS is involved and the reveal is immediate —
     zero dwell, the point of fb-20260917T232800Z). */
  .wheel-slot { pointer-events: none; }
  .wheel-tip {
    position: fixed;
    z-index: 21;
    box-sizing: border-box;
    max-width: 176px;
    padding: 2px 6px;
    border: var(--edge-w, 1px) solid var(--edge-inst);
    border-radius: 4px;
    background: var(--instrument);
    color: var(--ink-inst);
    box-shadow: var(--shadow-lift);
    font-family: var(--font-data);
    font-size: var(--t-10);
    font-weight: 600;
    line-height: 1.3;
    text-align: center;
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
    opacity: 0;
    visibility: hidden;
    pointer-events: none;
  }
  .wheel-slot:hover .wheel-tip,
  .wheel-slot:focus-within .wheel-tip {
    opacity: 1;
    visibility: visible;
  }
  .wheel-button {
    position: fixed;
    box-sizing: border-box;
    width: 30px;
    height: 30px;
    padding: 2px;
    border: 2px solid var(--edge-inst);
    border-radius: 999px;
    background: var(--instrument);
    color: var(--ink-inst);
    box-shadow: var(--shadow-lift);
    font-family: var(--font-ui);
    font-size: var(--t-10);
    font-weight: 600;
    line-height: 1;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
    cursor: pointer;
    pointer-events: auto;
    transition: transform 80ms ease-out, filter 80ms ease-out, box-shadow 80ms ease-out;
  }
  .wheel-button.badge--initiative { background: var(--instrument); border-color: var(--initiative); color: var(--ink-inst); }
  .wheel-button.badge--offered { background: var(--instrument); border-color: var(--offered); color: var(--ink-inst); }
  .wheel-button:hover,
  .wheel-button:focus-visible {
    transform: scale(1.1);
    filter: brightness(1.12);
    box-shadow: var(--shadow-lift), 0 0 0 2px var(--ink);
    outline: none;
  }
  /* Two classes, matching .wheel-button.badge--*'s specificity and coming
     after it in source order: a colour choice's tint must win over the tone
     tint, or the pip silently falls back to the plain instrument grey (the
     reported "where are my mana symbols" bug — the CSS was there, the more
     specific tone rule was simply painting over it every time). */
  .wheel-button.wheel-button--mana {
    background: var(--pip);
    border-color: color-mix(in srgb, var(--pip) 72%, #000);
    box-shadow: var(--shadow-lift), inset 0 0 0 2px rgb(255 255 255 / 0.16);
    color: #fff;
    font-size: var(--t-12);
    font-weight: 700;
    text-shadow: 0 1px 2px rgb(0 0 0 / 55%);
  }

  .menu-pop {
    position: fixed;
    z-index: 20;
    box-sizing: border-box;
    overflow-y: auto;
    background: var(--instrument);
    border: var(--edge-w, 1px) solid var(--edge-inst);
    border-radius: var(--radius);
    box-shadow: var(--shadow-lift);
    padding: 2px;
  }
  .menu { margin: 0; padding: 0; list-style: none; }
  .menu__item {
    display: block;
    width: 100%;
    text-align: left;
    background: none;
    border: 0;
    border-left: 2px solid transparent;
    border-radius: 0;
    color: var(--ink-inst);
    font-family: var(--font-ui);
    font-size: var(--t-12);
    line-height: 1.35;
    padding: var(--sp-1) var(--sp-2);
    cursor: pointer;
  }
  .menu__item:hover,
  .menu__item:focus-visible {
    background: color-mix(in srgb, var(--ink) 7%, var(--instrument));
    border-left-color: var(--ink-dim);
    color: var(--ink);
  }
</style>
