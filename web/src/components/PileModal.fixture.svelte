<script lang="ts">
  import type { CardView } from '../protocol';
  import PileModal from './PileModal.svelte';

  /**
   * PileModal.fixture.svelte: the pile-list cases the static SeatTable mount
   * in PileModal.fixture.ts cannot reach, because they need the `cards` prop
   * to CHANGE while the modal stays open.
   *
   * The wrapper mounts PileModal OPEN with a live $state pile. A test hovers
   * a card, waits out the 250 ms dwell, then takes that card out of the pile
   * through window.__pileRemoveLast — the modal stays open, the row
   * disappears without any pointer event (a removed element never fires
   * pointerleave), and the detail panel must close through supervise. The
   * removed card is the FINAL one deliberately: removing a non-final card
   * reflows its neighbour into the stationary pointer's spot, and the
   * browser's re-hover there legitimately arms a NEW card's dwell — a real
   * behaviour a removal test must not race against. The Escape layering
   * (detail first, modal second) is driven here too: onClose really closes,
   * so the second Escape ends with the modal gone.
   */
  const card = (id: number): CardView => ({
    id,
    name: `Fixture Card ${id}`,
    types: id % 2 === 0 ? 'Creature — Wizard' : 'Instant',
    printing: { name: `Fixture Card ${id}` },
    token: `#${id}`,
    tapped: false,
    power: 0,
    toughness: 0,
    damage: 0,
    attacking: false,
    controller: 0,
    owner: 0,
    summon_sick: false,
  });

  let cards = $state([card(1), card(2), card(3)]);
  let open = $state(true);

  (window as unknown as { __pileRemoveLast: () => void }).__pileRemoveLast = () => {
    cards = cards.slice(0, -1);
  };
</script>

<PileModal {open} title="Fixture graveyard" {cards} onClose={() => (open = false)} />
