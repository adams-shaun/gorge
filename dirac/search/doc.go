// Package search is gorge bots/azmcts's AlphaZero search ported onto dirac
// state, for the production config az:sims=100:autopay:oppnodes:oppfull:
// morekinds:closedlist:topk=6 (DefaultOptions).
//
// Same algorithm as azmcts (tree.go RunTree/simulate/selectEdge/commit):
// one PUCT tree per searched decision over the searching seat's searched
// points (and the opponent's, OpponentNodes), single perspective (every
// value is the seat's win probability; an opponent's point selects on
// 1 - Q), ISMCTS availability (N_avail counts the simulations a child was
// offered in, this one included), first-play urgency parent Q - FPU,
// strict > so the bot's candidate 0 wins ties, one expansion per
// simulation, the leaf evaluated once (no rollouts), the value backed up
// the path undiscounted; the move is the most-visited root child. Each
// simulation copies the root State (memcpy), re-deals what the seat cannot
// see from the per-decision seed (azmcts DecisionSeed / RedealSeed: the
// seat's own library order, the opponent's hand and library -- from a
// belief list under ClosedList), re-seeds the world's chance, then walks:
// the tree's edges are played as packed dirac actions, every other
// decision is answered by the Bot (azmcts: the auto-pay heuristic bot)
// until the next searched point, at most MaxSteps bot submits.
//
// Candidates (azmcts candidates.go / searchprobe enumerators), the bot's
// answer always first: priority = pass, casts, activations (the auto-pay
// vocabulary; land plays never, and a bot land play leaves the decision
// unsearched); attackers = whole declarations (bot's, none, all-in, each
// option toggled); blockers = whole declarations (bot's, none, each pair
// added, each bot pair removed; menace-invalid ones dropped); single
// target / modes / optional / small choose = one option each. Limit 6 cuts
// in enumeration order; PriorTopK keeps the bot's and the 5 of highest
// prior (only with an Evaluator that is a PriorSource, as azmcts needs a
// net). Keys: an object the seat saw at the root by its object index, any
// other by its card; the opponent's keys name every object by card and
// carry the opponent prefix (same-named candidates collapse).
//
// Preallocated: NewSearcher sizes the node/edge arrays (index-linked, no
// pointers), the walk's worlds, points and scratch; Search allocates
// nothing (TestSearchZeroAllocs).
//
// Options.CachedWorlds K > 0 runs K trees of Sims/K simulations over fixed
// deals with their roots merged (azmcts runCachedWorlds); Options.NodeCache
// > 0 then caches world states at tree nodes so a simulation resumes from
// its deepest stored node (azmcts nodecache.go; exact, TestNodeCacheIsExact).
// Both default off.
//
// Options.LazyPriors (default on) skips a replayed point's network ranking
// when the point belongs to an existing node, no candidate cut or
// truncation can fire and every candidate key already has an edge: nothing
// then reads the point's prior, so the call is wasted. The search is
// unchanged (visits, Q, priors, choices; TestLazyPriorsIsExact); each skip
// is counted in Stats.PriorSkips. Off, every point is ranked as before.
//
// The self-play options (azmcts's names; every one 0 = off,
// the search exactly as without it): PriorRankMaxOptions (points.go: skip
// the network ranking above that many options), ForcedK (zforced.go: forced
// root playouts, Result.Target pruned), and the seat's SimsByKind,
// playout-cap randomisation (FullProb, CheapSims, FullProbByKind) and
// decided-game searches (DecidedHigh, DecidedN, DecidedSims,
// DecidedWeight), which SeatSearch (zseat.go) applies around Search, read
// once per decision. PCRFull and DecisionSeed are gorge's bit for bit; the
// kinds are dirac's searched kinds (KindNames), so azmcts's
// "multitarget" is dirac's "target".
// Options.EarlyCutGap / EarlyCutShare stop a decision's search before Sims
// (earlycut.go): the gap rule when the runner-up can no longer catch the
// leader (the pick is the full search's), the share rule when the leader
// holds more than a share of the visits after a minimum. Searcher.Trace
// (CutTrace) records the root after every simulation so rules can be
// scored offline against one full search. Off by default.
//
// # Where this does not match azmcts exactly
//
//   - The bot: Heuristic is a small deterministic stand-in (land, then
//     the costliest castable spell; safe attacks; surviving or trading
//     blocks; harmful spells at the opponent's best creature). gorge's own
//     auto-pay heuristic bot is ported natively in dirac/search/hbot
//     (held to gorge's decisions by khbot); a host passes either as the
//     Bot to NewSearcher (hbot needs the table's Facts).
//   - Dirac poses gorge's multi-selects as single picks, so the fuller
//     kinds' set candidates (modes / choose / multitarget sets) are one
//     option per candidate at each pick; trigger order, mulligan and the
//     dirac's arrange picks are bot-answered.
//   - Priority candidate order: gorge sorts non-payment plays by their
//     actions' JSON; dirac keeps option order (casts in hand order,
//     then activations). Dirac's autopay offers every affordable cast,
//     so its vocabulary is gorge's payment casts plus X spells.
//   - Keys of objects the seat did not see at the root use the card, not
//     gorge's observer-local introduction-order reference (NameKeys
//     semantics for the seat too).
//   - Redeal: the seat's knowledge is its hand plus public zones; gorge's
//     KnownCardTracker facts (cards seen going into a library, scry tops,
//     revealed cards) and the deal's probe verification are not modelled.
//     The closed-list belief colours approximate Forge's colour identity
//     with the card's colours, cost pips and mana production.
//   - No backup discount, Dirichlet noise, move sampling or deadline (the
//     c6 eval config uses none of them).
//   - Leaf and prior: an Evaluator -- Uniform (0.5, the zero-cost stub),
//     HeuristicLeaf (azmcts's generation-0 searchprobe.LeafValue formula) or
//     dirac/neteval (the c6 network: value from the searching seat's view,
//     option logits from the deciding seat's with the bot's picks marked;
//     candidate logits as azmcts priorsWith composes them, softmaxed). The
//     options the network scores are dirac decision's (in canonical
//     order), where gorge under autopay scores its decision's options plus
//     synthetic plain-cast options appended for payment casts; dirac's
//     Done of a declaration is no gorge option and is left out of the
//     declaration's subset score.
package search
