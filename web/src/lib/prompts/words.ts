/**
 * promptWords reads an engine prompt or option label as a player would write
 * it. Other tools read the engine's strings off the wire, so the tidying happens
 * where they are shown: "turn 5 — discard 2 card(s)" becomes
 * "Discard 2 cards", and "Choose 1 to 1 mode(s)" becomes "Choose 1 mode".
 */
export function promptWords(text: string): string {
  const undated = text.replace(/^turn \d+ — (.)/, (_, first: string) => first.toUpperCase());
  return undated
    .replace(/\b(\d+) to \1\b/g, '$1')
    .replace(/(?<![\w/+-])1((?: (?!to\b|or\b)[a-z-]+){0,2}) (\w+)\(s\)/g, '1$1 $2')
    .replace(/(\w+)\(s\)/g, '$1s');
}
