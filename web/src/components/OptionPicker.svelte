<script lang="ts">
  import type { Option } from '../protocol';
  import type { TileOptions } from '../lib/cardoptions';
  import { postSingleAction, postTileOption, singleActionIcon, singleTapOptionOf } from '../lib/cardoptions';
  import {
    placeMenu,
    placeRadial,
    MENU_WIDTH,
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

  function toggle(): void {
    open = !open;
    if (open && badgeEl) {
      const r = badgeEl.getBoundingClientRect();
      anchor = { left: r.left, top: r.top, right: r.right, bottom: r.bottom };
    }
  }

  function close(): void {
    open = false;
  }

  function choose(option: Option): void {
    close();
    postTileOption(tileOptions, option);
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
      aria-label={action.label}
      title={action.label}
      onclick={(event) => {
        event.stopPropagation();
        postSingleAction(tileOptions);
      }}
    >
      <span aria-hidden="true">{icon === 'tap' ? '↻' : icon === 'cast' ? '✦' : '›'}</span>
    </button>
  {:else}
    <button
      class="badge badge--{tileOptions.tone}"
      class:selected={tileOptions.pickedOrder.length > 0}
      type="button"
      aria-haspopup="menu"
      aria-expanded={open}
      aria-label="{tileOptions.list.length} actions {subject}"
      title="Options {subject}"
      bind:this={badgeEl}
      onclick={(event) => {
        event.stopPropagation();
        toggle();
      }}
    >
      <span class="badge__n data">{tileOptions.list.length}</span>
    </button>
  {/if}

  {#if tileOptions.pickedOrder.length > 0}
    <span class="sel data" aria-label="picked {tileOptions.pickedOrder.join(', ')}">{tileOptions.pickedOrder.join(',')}</span>
  {/if}

  {#if open && tileOptions.list.length > 1 && tapAction === null}
    {#if radial}
      <div class="radial-pop" data-radial-picker role="menu" tabindex="-1" aria-label="Options {subject}" use:portal>
        {#each tileOptions.list as opt, i (opt.index)}
          {@const point = radialPlacement[i] ?? { x: 8, y: 8 }}
          {@const mana = manaSymbols[i]}
          <button
            class="wheel-button badge--{tileOptions.tone}"
            class:wheel-button--mana={isManaChoice}
            type="button"
            role="menuitem"
            data-wire-index={opt.index}
            data-mana-option={isManaChoice ? mana : undefined}
            aria-label={opt.label}
            title={opt.label}
            style:left="{point.x}px"
            style:top="{point.y}px"
            style:--pip={isManaChoice && mana ? `var(--mana-${mana.toLowerCase()})` : undefined}
            onclick={() => choose(opt)}
          >
            {#if !isManaChoice}<span>{compactLabel(opt.label)}</span>{/if}
          </button>
        {/each}
      </div>
    {:else}
      <!-- Keep the >6 list path structurally and visually unchanged. -->
      <div class="menu-pop" use:portal style:left="{menuPlacement.x}px" style:top="{menuPlacement.y}px" style:width="{MENU_WIDTH}px" style:max-height="{menuPlacement.maxHeight}px">
        <ul class="menu" role="menu" aria-label="Options {subject}">
          {#each tileOptions.list as opt (opt.index)}
            <li role="none">
              <button class="menu__item" type="button" role="menuitem" onclick={() => choose(opt)}>
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
    min-width: 1.1rem;
    height: 1.1rem;
    padding: 0 0.25rem;
    border-radius: 3px;
    border: var(--edge-w, 1px) solid var(--edge-inst);
    background: var(--instrument);
    color: var(--ink);
    font-family: var(--font-data);
    font-size: var(--t-10);
    font-weight: 600;
    cursor: pointer;
  }
  .badge--initiative { background: var(--initiative); border-color: var(--initiative); color: var(--felt-sunk); }
  .badge--offered { background: var(--offered); border-color: var(--offered); color: var(--felt-sunk); }
  .badge.selected,
  .action-icon.selected { outline: 2px solid var(--ink); outline-offset: 1px; }
  .action-icon { width: 1.35rem; padding: 0; font-size: var(--t-14); line-height: 1; }
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
  .wheel-button {
    position: fixed;
    box-sizing: border-box;
    width: 42px;
    height: 42px;
    padding: 4px;
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
  .wheel-button--mana {
    background: var(--pip);
    border-color: color-mix(in srgb, var(--pip) 72%, #000);
    box-shadow: var(--shadow-lift), inset 0 0 0 2px rgb(255 255 255 / 0.16);
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
