import { describe, expect, it } from 'vitest';
import { render } from 'svelte/server';
import type { CardView, PlayerView } from '../protocol';
import { LETHAL_CMD_DAMAGE } from '../lib/commander';
import IdentityBar from './IdentityBar.svelte';

const card = (id: number, name: string): CardView => ({
  id, name, types: 'Legendary Creature',
  tapped: false, power: 0, toughness: 0, damage: 0, attacking: false,
  controller: 0, owner: 0, summon_sick: false, printing: { name }, token: `#${id}`,
});

const player = (over: Partial<PlayerView> = {}): PlayerView => ({
  seat: 0, name: 'Ari', life: 38, lost: false, library_size: 30, hand_size: 7, graveyard_size: 0,
  hand: [], battlefield: [], graveyard: [], exile: [], pool: {},
  command: [], commanders: [], commander_casts: [], ...over,
});

const bar = (p: PlayerView, players: PlayerView[] = [p]) =>
  render(IdentityBar, { props: { player: p, players, colour: '#e5484d', active: false, priority: false, corner: 'bl' } }).html;

// the dealers' rosters the taker's clock resolves names from
const dealers = [
  player({ seat: 1, name: 'Bo', commanders: [card(10, 'Isamaru')] }),
  player({ seat: 2, name: 'Ci', commanders: [card(11, 'Zur')] }),
];

describe('IdentityBar — the commander-damage clock (CR 903.10)', () => {
  it('no commander damage on the wire renders no clock at all', () => {
    const html = bar(player({ life: 38 }));
    expect(html).not.toContain('data-commander-damage');
    expect(html).toContain('>38<'); // life is still there, alone
  });

  it('a low total renders quietly, with the commander named and the amount exact', () => {
    const html = bar(player({ life: 38, cmd_damage: { '10': 5 } }), dealers);
    expect(html).toContain('data-commander-damage');
    expect(html).toContain('data-cmd-damage="10"');
    expect(html).toContain('Isamaru');
    expect(html).toContain('>5<');
    expect(html).toContain('data-cmd-state="low"');
    expect(html).not.toContain('critical');
    expect(html).not.toContain('lethal');
  });

  it('19 from one commander is surfaced distinctly from a low total — the danger line', () => {
    const at19 = bar(player({ life: 38, cmd_damage: { '10': 19 } }), dealers);
    expect(at19).toContain('data-cmd-state="critical"');
    // the danger class is on the row (Svelte's scope class sits between)
    expect(at19).toMatch(/class="cmd-row[^"]*\bcrit\b/);
    const at20 = bar(player({ life: 38, cmd_damage: { '10': 20 } }), dealers);
    expect(at20).toContain('data-cmd-state="critical"');
    const at18 = bar(player({ life: 38, cmd_damage: { '10': 18 } }), dealers);
    expect(at18).toContain('data-cmd-state="low"');
    expect(at18).not.toContain('"critical"');
  });

  it(`${LETHAL_CMD_DAMAGE} from one commander is lethal no matter the life total`, () => {
    const html = bar(player({ life: 38, cmd_damage: { '10': LETHAL_CMD_DAMAGE } }), dealers);
    expect(html).toContain('data-cmd-state="lethal"');
    expect(html).toMatch(/class="cmd-row[^"]*\blethal\b/);
    // the second clock reads beside the life it overrides
    expect(html.indexOf('>38<')).toBeLessThan(html.indexOf('data-commander-damage'));
  });

  it('commander damage is per commander, never summed: 11 + 11 is two rows of 11, not one 22', () => {
    const html = bar(player({ life: 38, cmd_damage: { '10': 11, '11': 11 } }), dealers);
    const rows = [...html.matchAll(/data-cmd-damage="(\d+)" data-cmd-amount="(\d+)" data-cmd-state="([^"]+)"/g)].map((m) => ({ id: m[1], amount: m[2], state: m[3] }));
    expect(rows).toEqual([
      { id: '10', amount: '11', state: 'low' },
      { id: '11', amount: '11', state: 'low' },
    ]);
    // the classic wrong implementation reads 22; the two rows keep 11 each
    expect(html).not.toContain('>22<');
  });

  it('21 from ONE commander is a single lethal row even beside another commander\'s damage', () => {
    const html = bar(player({ life: 38, cmd_damage: { '10': LETHAL_CMD_DAMAGE, '11': 11 } }), dealers);
    expect(html).toContain('data-cmd-state="lethal"');
    expect(html).toContain('data-cmd-state="low"');
    expect(html.match(/data-cmd-damage=/g)).toHaveLength(2);
  });

  it('name and seat come from the dealer rosters, not the taker\'s own view', () => {
    const html = bar(player({ seat: 0, cmd_damage: { '10': 7, '11': 3 } }), dealers);
    expect(html).toContain('Isamaru');
    expect(html).toContain('Zur');
    // dealer order is their seat order, deterministic
    const names = [...html.matchAll(/class="cmd-name[^"]*">([^<]+)</g)].map((m) => m[1]);
    expect(names).toEqual(['Isamaru', 'Zur']);
  });
});
