/**
 * The distinct mana symbols observed in the pinned Forge card corpus
 * (`.cards/cardsfolder`, 34,613 `ManaCost:` lines), extracted with:
 *
 *   grep -rhoE "^ManaCost:.*" .cards/cardsfolder \
 *     | sed 's/ManaCost://' | tr ' ' '\n' | sort -u
 *
 * The pipeline's own artifacts — the "no cost" whole-string marker for cards
 * with no mana cost (2,089 lines), which splits into the tokens "no" and
 * "cost" — are excluded; they are not pips. Every other token MUST parse to a
 * non-empty symbol; manaSymbols' corpus test ratchets against this list so a
 * new symbol nobody thought of fails that test instead of silently blanking
 * every affected card. If the corpus pin moves, regenerate this list and rerun
 * the test.
 */
export const CORPUS_MANA_SYMBOLS: readonly string[] = [
  '0', '1', '10', '11', '12', '13', '14', '15', '16', '2',
  '2/B', '2/G', '2/R', '2B', '2G', '2R', '2U', '2W',
  '3', '4', '5', '6', '7', '8', '9',
  'B', 'BG', 'BP', 'BR',
  'C', 'CB', 'CG', 'CR', 'CU', 'CW',
  'G', 'GP', 'GU', 'GUP', 'GW', 'GWP',
  'PRG',
  'R', 'RG', 'RP', 'RW', 'RWP',
  'S', 'U', 'UB', 'UP', 'UR',
  'W', 'WB', 'WP', 'WU',
  'X',
];
