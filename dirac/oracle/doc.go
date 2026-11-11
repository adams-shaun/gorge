// Package oracle runs gorge (github.com/adams-shaun/gorge) and dirac
// (package dirac) in lockstep: the same decks and seed, the
// same canonicalised option list at every decision, the same canonical
// state projection after every action gorge commits. It is the kernel's
// oracle: a divergence is reported with both sides' options or projections
// diffed, a replayable JSON repro and a bounded shrink of its action prefix.
//
// # Pieces
//
//   - Setup (setup.go): Corpus opens the gorge corpus (cards.SharedCorpus)
//     and resolves pool decks (engine/deck.Load); Matchup.Config is the
//     rules.Config gorge self-play builds (Names p0/p1, Tokens,
//     NameUniverse, Mulligans, bench.PlayGame's default object cap), and
//     NewGorgeSide starts it with rules.New + Advance exactly as play does.
//     Mulligan keep/mulligan answers are the Policy's, not setup.
//   - GorgeSide (gorge.go): gorge behind the canonical surface (Side).
//   - KernelSide (kapi.go): a kernel behind the same surface. kapi.go is the
//     only file naming kernel types; GorgeKernel / GorgeKernelAPI are the
//     self-check kernels (a second gorge engine, directly or through the
//     contract's Kernel surface).
//   - Lockstep (gorge-led) and LockstepLed (either side leads, the policy
//     answers the leader's decision) in lockstep.go, Repro/Shrink (repro.go), Replay of a kernel
//     game through gorge with forced chance outcomes (replay.go), Fuzz and
//     the pairing rule (fuzz.go), cmd/kfuzz.
//
// # Card identity
//
// Every card object is a canonical instance (a Ref, 0..MaxInst-1): deck s's
// i-th card is instance offset(s)+i (offset(0) = 0, offset(1) = len(deck
// 0)), exactly gorge's genesis order (AddObject per seat in decklist
// order); tokens and copies take the next instances in creation order.
// gorge keeps an ObjID across zone changes, so the binding never changes.
// Ability objects on the stack are no card: a reference to one is
// RefStack+position (bottom = 0). Players are RefPlayer+seat; RefNone is
// none. A deck card's table index (Matchup.Table, sorted distinct names) is
// the kernel's CardID.
//
// # Canonical option grammar
//
// A canonical option is a Canon, packed kind<<24 | src<<16 | tgt<<8 | mode
// (the contract's Action layout) over the oracle's own kinds. A decision's
// options are sorted ascending. Per gorge decision kind:
//
//	priority     pass                         (Pass, -, -, 0)
//	             land play                    (PlayLand, land, -, face played: modal_land 1)
//	             cast                         (Cast, card, -, castMode | altCostIndex<<4)
//	             cast a face                  (CastFace, card, -, face<<4 | route), route faces.go's
//	                                          faceCastModes index (adventure_alt 2, adventure_recast 3, split_alt 4, ...)
//	             activate (non-mana)          (Activate, permanent, -, Face.Abilities index)
//	             special action               (Special, obj, -, specialKinds index)
//	attackers    per (creature, defender)     (Attack, creature, P<seat> | battle, 0)
//	blockers     per (blocker, attacker)      (Block, blocker, attacker, 0)
//	target       per target                   (Target, -, target, 0)
//	modes        per mode                     (Mode, -, -, option index = Choices$ order)
//	mulligan     keep / mulligan / bottom     (Keep|Mulligan, -, -, 0) / (Bottom, card, -, 0)
//	trigger_order per trigger                 (Order, trigger source, -, occurrence)
//	trigger_optional yes / no                 (Yes|No, trigger source, -, 0)
//	arrange      per card (pile A)            (Arrange, card, -, 0)
//	replacement  per replacement              (Replacement, source, -, occurrence); apply/decline = Yes/No;
//	             a mana-colour pick           (Choose+mana, -, -, colour)
//	starting_player per seat                  (StartPlayer, -, P<seat>, 0)
//	commander_zone                            (CommanderZone, commander, -, option index)
//	choose       per option                   (Choose+k, obj, -, value), k the option kind's index in
//	             chooseKinds; value = Amount (x, number), colour WUBRGC (mana, color), option index
//	             (division: the lexicographic composition order over the attacker's blockers;
//	             altaddcost, pile-a/b, dungeon, room), pip announcement pay_W..pay_C/generic/life =
//	             0..7, convoke announcement convoke_W..convoke_G/generic = 0..5 (CR 702.51a), improvise_generic = 6 (CR 702.126a), harmonize = 7 (CR 702.180a);
//	             a label-valued pick (type, name) carries a 16-bit value in (tgt, mode): the
//	             label's index in gorge's creature-type vocabulary (effects.CreatureTypeWordList),
//	             else 0x8000 | its 15-bit FNV-1a; a player-valued pick has tgt P<seat>.
//
// castMode is castModes' index of Option.Mode / the planned cast's origin
// ("" 0, kicked 1, flashback 2, mayplay 3, surged 4, miracle 5, bestowed 6,
// ...). "occurrence" is the k-th repeat, in gorge's option order, of an
// otherwise identical tuple (two triggers of one source); it is the one
// place the canonical form depends on gorge's option order rather than on
// the game state alone (Stats.Occurrence counts it).
//
// # Multi-pick
//
// A decision that is not Min == Max == 1 is posed as a sequence of single
// picks of the same kind: each step offers every option not yet chosen
// (unless Repeatable) that the published whole-answer constraints admit
// (group caps, budgets, set properties, same controller, one declaration
// per attacking/blocking creature) AND after which the answer can still be
// completed into a valid one (an exact search over the remaining picks:
// a lone blocker on a menace attacker is not offered when no second blocker
// can join it), plus (Done, -, -, 0) once the answer so far is valid
// (Decision.Validate, Required attackers covered). An attackers or
// blockers declaration is submitted at Done only (the kernel's AkDone);
// any other answer at Done or as soon as Max picks are made. The answer's order is the pick
// order (trigger order: first pick goes on the stack first; arrange: pile A
// top first; bottoming: bottom order).
//
// Config.ArrangeRestOrder mirrors the kernel's second scry/surveil phase:
// closing pile A with Done opens a pile-B order sequence (the same picks,
// no Done, all but the last card -- the kernel implies it), and the answer
// is submitted with Intent.Rest in that order. Off, an arrange's pile B
// keeps gorge's offered order (the legacy answer).
//
// # Autopay priority vocabulary
//
// The kernel has no manual mana. gorge's mana abilities ("activate") and
// concede are never options; the plays are, each paid the way gorge's own
// auto-pay surface pays it:
//
//   - a cast gorge's planner pays (Decision.PaymentActions with a plan):
//     Intent.Payment with Plans[0] (the planner's first witness);
//   - a potential play (Engine.PotentialPaymentPlans) priced with a witness:
//     the witness is lowered (bots/spellbench/payexec: activate each source
//     at priority, answer the colour asks, then pick the play's option);
//   - a potential play priced after a scripted prefix (a source the census
//     cannot price, e.g. Giada's Angel-only mana): the script's answers,
//     then the play paid as priced at the next priority decision -- offered
//     only when that completes on a clone of the engine;
//   - an offered play (free, or the floating pool pays it): as offered.
//
// Payment windows and colour asks those routes pose are answered beneath
// the surface (Stats.Internal); a planned cast that falls back to the
// manual window is answered by its auto-fill, else a source making an owed
// colour (Stats.Fallbacks).
//
// A fallback payment window can list a hand-zone source (Elvish Spirit Guide
// or Simian Spirit Guide) that the kernel cannot pay with, so kgen refuses
// those cards rather than lower an inert ability (TestSpiritGuideGorgeWindow
// measures the window, TestSpiritGuideRefusal the refusal).
//
// Plays the planner cannot price (Reason
// unsupported/ambiguous, or a dead-end script) are not options: gorge's
// auto-pay surface cannot make them (Stats.Unpriced lists them: Luminous
// Rebuke's target-dependent cost, Eaten Alive's optional cost, the bait
// counter abilities).
//
// A resolution-time payment's "_manual" route (pay by choosing sources) is
// hidden like a mana tap; its auto route is the option.
//
// # Elision
//
// Config.PassElision mirrors the kernel's elision: a canonical decision or
// multi-pick step with exactly one option is answered in place and never
// posed. Off, every decision is posed (dirac.Config.NoElide).
//
// # Projection
//
// Projection (project.go) is compared with == and diffed field by field:
// per instance its front-face name hash, deck table index, zone (ZoneCeased
// for a token that left), owner, controller, library position from the top,
// flags (tapped, summoning sick, attacking, blocking, face down, token,
// copy, phased out, monstrous), damage, counters (+1/+1, -1/-1, loyalty,
// other), layer-applied power/toughness of battlefield creatures,
// attached-to and the attacker it blocks; per seat life, poison, lands
// played (the active player's only), mana pool WUBRGC, hand and library sizes; the stack bottom first
// (kind spell/activated/triggered, source, ability index -- Face.Abilities
// i or 0x80|Face.Triggers i --, controller, up to four targets); turn, step
// (gorge's state.Step), active player, priority holder (not compared: the lockstep compares the
// decision's player instead), RNG draw count
// (Engine.RNGDraws), game over and winner. It is read from state.Game and
// the engine's derived characteristics (the source engine/view itself
// reads).
//
// # Not 1:1
//
// Stats records where canonicalisation is not a pure function of the state:
// Occurrence (repeated tuples), Duplicate (tuples that still collided and
// were dropped), Unknown (option kinds outside the tables, canonicalised as
// choose:other), Unpriced (legal plays the autopay vocabulary leaves out)
// and Overflow (decisions with more than MaxOptions options).
package oracle
