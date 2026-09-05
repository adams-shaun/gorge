/** manaSymbols splits Forge's cost notation ("1 W", "X G G", "W/U") into renderable symbols; anything unrecognised yields []. */
export function manaSymbols(cost: string): string[] {
  if (!cost) return [];
  const parts = cost.trim().split(/\s+/);
  const ok = parts.every((p) => /^(\d+|X|Y|Z|[WUBRGC]|[WUBRGC]\/[WUBRGCP]|S|2\/[WUBRG])$/.test(p));
  return ok ? parts : [];
}
