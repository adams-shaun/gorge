// Package feat encodes a kernel position into the network inputs gorge's
// bots/policynet produces for the same position under FeaturesEntity (the
// entity network's feature set): the state (92 dense scalars, the sorted hashed
// sparse bag, one entity card per visible card) and one option encoding per
// decision option -- byte-for-byte what policynet.EncodeStateWith and
// EncodeOptionsWith produce from gorge's redacted view.View of the deciding
// seat. dirac/oracle's lockstep asserts it at every decision (FeatCheck).
//
// The kernel has no strings, so every string-derived fact policynet reads
// (card names, type lines, mana costs, spell APIs, keyword spellings, the
// mana a source advertises as available, off-battlefield P/T) is
// precomputed per CardID into a Pool at pool load (dirac/oracle.FeatPool
// builds one from gorge); the hot path streams FNV-1a over those
// precomputed rows and the kernel's own numbers. feat imports nothing from
// gorge: its output types (Feature, Card, State, Option) are
// layout-compatible with policynet's (Row uint16, Value float32), with
// fixed-capacity storage, and an encode allocates nothing.
//
// # Use from a search
//
//	enc := feat.NewEncoder(pool)                 // one per goroutine
//	st := enc.EncodeState(&s, &d)                // the state at decision d (d.Player's view)
//	opts = enc.EncodeOptions(&d, opts)           // d's options, in d's option order
//	opts = enc.EncodeOptionsBot(&d, opts, bot)   // the same, naming the bot's answer (index in d)
//
// EncodeStateInc is the incremental encoder (inc.go): the same output,
// regenerating only the cards whose inputs changed. It is asserted equal to
// EncodeState at every lockstep decision, but on today's kernel it is not
// faster (see inc.go and the measurements in the package history): the
// per-encode view build is the floor.
//
// # What the network was trained on, and what a kernel search must not ask
//
// The c6 net was trained on gorge's decisions, whose option lists differ
// from the kernel's in three ways a search has to respect:
//
//   - Option order and count. The option encoding includes the option's
//     index over the list length (policynet odIndex = idx/(n-1)), and
//     gorge's list is not the kernel's: each decision kind lists its options
//     in gorge's own enumeration order, and a priority list also carries
//     one manual mana-tap "activate" option per source p can tap, "concede"
//     after "pass", and -- appended after concede -- one synthetic plain
//     cast per planner-paid cast that has no legacy cast option (azmcts
//     priorsWith's payCasts form, the PriorTopK search's prior: the bot's
//     first, then engine order). EncodeOptions encodes every option at its
//     place in THAT list (listpos.go), synthesised from kernel state, so the
//     encoding equals gorge's for every option gorge's search scores; the
//     lockstep asserts the position, the list length and the bytes. Name the
//     bot's answer with EncodeOptionsBot when it is a planner-paid cast (it
//     changes the synthetic casts' order). An option gorge's list has no
//     entry for -- Done, a kicked cast or one gorge's planner does not plan
//     ({X}, an either-or additional cost, a target-conditional reduction)
//     that the floating pool cannot pay -- is encoded as the list's last
//     entry; gorge's search never scores such an option.
//     gorge itself is not consistent here: its training record
//     (azmcts record.go) appends synthetic casts only for the candidates
//     the search kept, in the kept order (PriorTopK reorders them by the
//     prior), while inference (priorsWith) appends one for every payment
//     candidate, bot first then engine order; and without PriorTopK the
//     plain prior appends none and falls back to uniform. EncodeOptions
//     follows inference.
//   - Multi-pick decisions. gorge poses attackers, blockers, discards,
//     bottoming, arrange and target sets as ONE decision on ONE state and
//     scores a candidate SET (CandidateScore over the chosen options); the
//     kernel asks one pick at a time and applies earlier picks to its state.
//     Encode a declaration at its FIRST pick (before any pick is applied;
//     EncodeState masks in-progress attack/block flags, but a discard or a
//     cost pick has already moved its card), score the whole set from that
//     encoding as gorge does, and replay the picks without re-asking the
//     net.
//   - Done. The kernel closes a declaration with an AkDone option; gorge has
//     no such option (an answer simply ends), so the net was never trained
//     on an option of kind "done". EncodeOptions encodes it (kind "done",
//     the option-kind "other" slot) only so the list is complete: never use
//     the net's score for a Done option. Score the declaration as a set at
//     its first pick instead.
package feat
