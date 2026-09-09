<script lang="ts">
  import type { SeatInfo, View } from '../protocol';
  import type { Stops, TurnSide } from '../lib/autopilot';
  import type { CardOptions } from '../lib/cardoptions';
  import Board from './Board.svelte';
  import PhaseTrack from './PhaseTrack.svelte';

  /**
   * BoardStage owns the one piece of geometry Board and PhaseTrack share: the
   * clock's lane through the felt. During play Board reserves the lane and the
   * painted track is centred in it. Mulligan keeps ui26's separate top lane so
   * the opening-hand panel retains the board centre.
   */
  let {
    view,
    seats,
    options = null,
    seat = null,
    stops = null,
    onToggle = null,
    mulligan = false,
  }: {
    view: View;
    seats: SeatInfo[];
    options?: CardOptions | null;
    seat?: number | null;
    stops?: Stops | null;
    onToggle?: ((step: string, side: TurnSide) => void) | null;
    mulligan?: boolean;
  } = $props();
</script>

<div class="board-stage" data-board-stage>
  <Board {view} {seats} {options} reserveCentre={!mulligan} />
  <div class="phase-shard" class:mulligan data-phase-lane>
    <PhaseTrack {view} {seats} {seat} {stops} {onToggle} />
  </div>
</div>

<style>
  .board-stage {
    position: relative;
    width: 100%;
    height: 100%;
    min-width: 0;
  }
  .phase-shard {
    position: absolute;
    top: 50%;
    left: 0;
    right: 0;
    transform: translateY(-50%);
    z-index: 7;
    min-width: 0;
  }
  /* Mulligan keeps the centre for its hand; the clock retains ui26's compact
     top lane for that one decision instead of competing for the same pixels. */
  .phase-shard.mulligan {
    top: 0;
    transform: none;
  }
</style>
