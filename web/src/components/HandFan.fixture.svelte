<script lang="ts">
  import type { CardView, PlayerView } from '../protocol';
  import HandFan from './HandFan.svelte';

  /**
   * HandFan fixture: a real browser-mounted hand fan for the hover-inspector
   * lifetime regression on the HAND surface (fb-20260923T020655Z). The window
   * hooks below mutate the LIVE view the way an incoming stream frame does.
   *
   * The critical difference from Quadrant.fixture: a real live view refresh
   * REPLACES the rendered graph with a freshly deserialized one. So the "tick"
   * hook rebuilds the hand with FRESH-IDENTITY objects carrying the same ids
   * (`player.hand = player.hand.map(...)`) — `Object.assign` on the existing
   * objects would preserve identity and would NOT reproduce the defect. The
   * keyed each `{#each hand as c, i (c.id)}` keeps the DOM node but hands it a
   * fresh `CardView`, which is exactly what broke the old `hovered === c` gate.
   */

  const card = (id: number, name: string, over: Partial<CardView> = {}): CardView => ({
    id, name, types: 'Creature Bear', mana_cost: '1G',
    tapped: false, power: 2, toughness: 2, damage: 0, attacking: false,
    controller: 0, owner: 0, summon_sick: false, printing: { name }, token: '',
    ...over,
  });

  const player = $state<PlayerView>({
    seat: 0, name: 'P0', life: 20, lost: false, library_size: 60, hand_size: 3,
    graveyard_size: 0, hand: [card(7, 'Grizzly Bears'), card(8, 'Forest'), card(9, 'Hill Giant')],
    battlefield: [], graveyard: [], exile: [], pool: {}, command: [], commanders: [], commander_casts: [],
  });

  const w = window as unknown as {
    __tick: (over?: Partial<CardView>) => void;
    __replaceId: (from: number, to: number) => void;
    __remove: (id: number) => void;
  };
  // A live view refresh: the hand is rebuilt with FRESH objects, same ids. The
  // optional patch rides the rebuilt object so the test can prove the panel
  // renders refreshed data, not a stale snapshot.
  w.__tick = (over) => {
    player.hand = player.hand.map((c) => ({ ...c, ...over }));
  };
  // A card's id changes in place (the object was replaced, not updated).
  w.__replaceId = (from, to) => {
    player.hand = player.hand.map((c) => (c.id === from ? { ...c, id: to } : c));
  };
  // The described card leaves the hand entirely (played, discarded, exiled).
  w.__remove = (id) => {
    player.hand = player.hand.filter((c) => c.id !== id);
  };
</script>

<HandFan {player} />
