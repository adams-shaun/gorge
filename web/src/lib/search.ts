import type { CardView, Decision, Option } from '../protocol';
import { cardFromPick } from './arrange';

/**
 * search.ts is the library-search ask's pure half (fb-20260916T181754Z). A
 * hidden-library search (a fetchland, Diabolic Tutor, a Squadron Hawk pick —
 * 1166 corpus card files pose one) is a KChoose whose options are CARD PICKS
 * in LIBRARY scan order: Option.kind "search", Obj the card's id, Label the
 * card's bare name (effects/zone.go effSearchLibrary). Nothing on the wire
 * needs to change — the label and Obj are everything a client-side filter
 * and sort need — so this module is display-only by contract: it never
 * reorders `picked`, never touches the submit gate, and the answer posted is
 * still the picked wire indexes in click order (the engine reads the chosen
 * cards' order from the click sequence, not display position).
 *
 * The detection predicate and the sort/filter live here so the component
 * stays a renderer and the next consumer of the same vocabulary cannot fork
 * the shape — the same split discard.ts (fb-20260914T120705Z) established.
 */

/**
 * isSearchPick reports whether a decision is a library-search ask: it has
 * options, and EVERY non-concede option is a card pick — kind "search" with a
 * set Obj. A decision that mixes any other option kind (a mode list, a pass
 * option, an option with no object to show) stays on the generic text list.
 * The test is on the OPTION shapes, not on the decision's own kind (which is
 * KChoose, shared with every other list-pick), exactly as isDiscardPick is.
 * `decision` comes in as the panel's pending-or-null decision, so null is
 * false by construction. A search ask with NO options (a fail-to-find-only
 * Min 0 shape the engine resolves silently) is not this shape and never
 * reaches here.
 */
export function isSearchPick(d: Decision | null): boolean {
  if (d === null || d.options.length === 0) return false;
  const picks = d.options.filter((o) => o.kind !== 'concede');
  return picks.length > 0 && picks.every((o) => o.kind === 'search' && o.obj !== undefined);
}

/**
 * searchMatches says whether one option's label survives the filter: a
 * case-insensitive substring match on the card name. An empty (or
 * whitespace-only) filter matches everything — the unfiltered, sorted list.
 */
export function searchMatches(label: string, filter: string): boolean {
  const q = filter.trim().toLowerCase();
  if (q === '') return true;
  return label.toLowerCase().includes(q);
}

/**
 * searchOptions is the DISPLAY list: the decision's options that survive the
 * filter, sorted A→Z case-insensitively ("unsorted cards" is half the
 * complaint). Half the complaint's other half — the wall of cards in library
 * order — is safe to sort away because a search's answer is a SET of wire
 * indexes in click order: the engine reads the chosen cards' order from the
 * click sequence (SeatPanelState.picked), never from display position, and a
 * search decision carries no pass/resolve option, so primaryOf resolves
 * nothing whose position a sort could move. The sort is stable, and equal
 * labels (two copies of one card) keep their offered (library) order — the
 * explicit tie-break makes that hold regardless of the engine's sort
 * stability, and the input array is never mutated. The `picked` ordinals a
 * rendered row overlays are click-order facts read off SeatPanelState.picked,
 * so they follow the cards across any display order.
 */
export function searchOptions(d: Decision, filter: string): Option[] {
  const visible = d.options.filter((o) => searchMatches(o.label, filter));
  return visible.sort((a, b) => {
    const al = a.label.toLowerCase();
    const bl = b.label.toLowerCase();
    if (al !== bl) return al < bl ? -1 : 1;
    if (a.label !== b.label) return a.label < b.label ? -1 : 1;
    return 0;
  });
}

/**
 * searchCard synthesizes the CardView a search option renders as a face —
 * the shared cardFromPick under the option's own label, which for a search
 * option IS the card's bare name (no prefix to strip, unlike the discard
 * family's three "Discard "-prefixed shapes). The name is what both the art
 * proxy and the oracle resolver look the card up by, so the synthesized face
 * gets the real printed face and the hover inspector gets the real printed
 * oracle text for free. A library object the engine could not name
 * ("a card") renders as a blank face titled exactly that — the filter cannot
 * help with those either, and the label is still displayed verbatim.
 */
export function searchCard(d: Decision, o: Option): CardView {
  return cardFromPick(d, o, o.label);
}
