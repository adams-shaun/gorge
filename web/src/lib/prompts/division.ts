import type { CardView, Decision, Option, View } from '../../protocol';
import { findCardAnywhere } from '../board';

/**
 * division.ts reads a combat damage division's option words against the
 * board. The engine labels each split "0 to Grizzly Bears,2 to Grizzly Bears",
 * one share per recipient in the order the attacker's `blocked_by` lists them,
 * so two recipients with one name read alike. Each same-named recipient gets a
 * letter, drawn on its board tile and printed in the row.
 */

const SHARE = /(\d+) to (.+?)(?=,\d+ to |$)/g;

/** isDivision reports whether every option is a split of damage among named recipients. */
export function isDivision(d: Decision): boolean {
  return d.kind === 'choose' && d.options.length > 0 && d.options.every((o) => /^\d+ to /.test(o.label));
}

/**
 * recipients are the creatures sharing the source's damage, in label order: an
 * attacker's blockers, or the attackers a banded blocker blocks (in battlefield
 * order, as the engine walks them).
 */
function recipients(d: Decision, view: View): CardView[] {
  if (d.source === undefined) return [];
  const source = findCardAnywhere(view, d.source);
  if (!source) return [];
  const field = view.players.flatMap((p) => p.battlefield);
  if (source.attacking) return (source.blocked_by ?? []).flatMap((id) => field.find((c) => c.id === id) ?? []);
  return field.filter((c) => c.blocked_by?.includes(source.id));
}

/** divisionMarks maps each same-named recipient to its letter, in label order. */
export function divisionMarks(d: Decision, view: View): Map<number, string> {
  const marks = new Map<number, string>();
  const all = recipients(d, view);
  const repeated = all.filter((c) => all.some((other) => other.id !== c.id && other.name === c.name));
  repeated.forEach((c, i) => marks.set(c.id, String.fromCharCode(65 + i)));
  return marks;
}

/** divisionRowText is one split's words, "0 to Grizzly Bears A, 2 to Grizzly Bears B"; a letter never wraps away from its name. */
export function divisionRowText(d: Decision, view: View, o: Option): string {
  const shares = [...o.label.matchAll(SHARE)];
  const all = recipients(d, view);
  const marks = divisionMarks(d, view);
  return shares
    .map(([, amount, name], i) => {
      const mark = all[i]?.name === name ? marks.get(all[i].id) : undefined;
      return `${amount} to ${name}${mark ? `\u00a0${mark}` : ''}`;
    })
    .join(', ');
}
