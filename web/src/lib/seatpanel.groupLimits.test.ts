import { describe, expect, it } from 'vitest';
import type { Decision, Option } from '../protocol';
import { pickOption } from './seatpanel.svelte';

// pickOption against Decision.groupLimits: a per-defender attack ceiling
// (AttackRestrict's MaxAttackers$ scoped by ValidDefender$) names each capped
// defender's Group and its own cap. The client stays rules-ignorant
// (R-E4-2): it derives the cap from the wire field alone, so a defender
// capped at two accepts a second attacker while a defender capped at one
// still replaces. The map overrides the decision-wide groupLimit for the
// groups it names.

const opt = (index: number, obj: number, player: number, group?: string): Option => ({
  index,
  kind: 'attacker',
  label: `attack ${obj} at player ${player}`,
  obj,
  player,
  ...(group ? { group } : {}),
});

// Two attackers offered at seat 0 (scoped, cap 2) and two at seat 2
// (scoped, cap 1): one decision, per-group limits that differ.
const scoped = (): Decision => ({
  seq: 21,
  player: 1,
  kind: 'attackers',
  prompt: 'declare attackers',
  min: 0,
  max: 4,
  groupLimits: { 'attack-restrict:0': 2, 'attack-restrict:2': 1 },
  options: [
    opt(0, 1, 0, 'attack-restrict:0'),
    opt(1, 2, 0, 'attack-restrict:0'),
    opt(2, 1, 2, 'attack-restrict:2'),
    opt(3, 2, 2, 'attack-restrict:2'),
  ],
});

describe('pickOption — per-Group cap from groupLimits', () => {
  it('appends a second member of a group capped at two', () => {
    const d = scoped();
    expect(pickOption(d, 0, [])).toEqual([0]);
    expect(pickOption(d, 1, [0])).toEqual([0, 1]);
  });
  it('replaces at a group whose own cap is one even though another group allows two', () => {
    const d = scoped();
    let picked = pickOption(d, 2, []); // [2] -- group "attack-restrict:2" cap 1
    picked = pickOption(d, 3, picked); // cap full: oldest member replaced
    expect(picked).toEqual([3]);
  });
  it('keeps groupLimit when groupLimits does not name the group', () => {
    const d: Decision = {
      seq: 22,
      player: 1,
      kind: 'choose',
      prompt: 'choose',
      min: 0,
      max: 4,
      groupLimit: 2,
      groupLimits: { other: 3 },
      options: [
        opt(0, 1, 0, 'unnamed'),
        opt(1, 2, 0, 'unnamed'),
        opt(2, 3, 0, 'unnamed'),
      ],
    };
    let picked = pickOption(d, 0, []);
    picked = pickOption(d, 1, picked);
    expect(picked).toEqual([0, 1]);
  });
});
