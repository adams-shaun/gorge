import type { Decision, Option } from '../protocol';

/** Display-only name-a-card picker shape: one exact choice and nameless wire options. */
export function isNamePick(d: Decision | null): boolean {
  if (d === null || d.min !== 1 || d.max !== 1 || d.options.length === 0) return false;
  const picks = d.options.filter((o) => o.kind !== 'concede');
  return picks.length > 0 && picks.every((o) => o.kind === 'name' && o.obj === undefined);
}

export function nameOptions(d: Decision, filter: string): Option[] {
  const q = filter.trim().toLowerCase();
  return d.options.filter((o) => o.kind === 'name' && o.obj === undefined && (q === '' || o.label.toLowerCase().includes(q)))
    .sort((a, b) => a.label.toLowerCase().localeCompare(b.label.toLowerCase()) || a.label.localeCompare(b.label));
}

export const NAME_PICK_RENDER_LIMIT = 200;
