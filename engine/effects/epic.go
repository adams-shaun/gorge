package effects

// kw:Epic (CR 702.50a, Enduring Ideal / Eternal Dominion).
//
// The registration rides on three already-supported primitives composed by
// the engine/cards/kw_epic.go expansion, none of them new:
//
//   - the rest-of-game "you can't cast spells" clause is an Effect-delivered
//     CantBeCast static (Mode$ CantBeCast | ValidCard$ Card | Caster$ You,
//     Duration$ Permanent) -- the Bilbo's Gambit route this build already
//     consults: rules' castRestrictionSources merges registered CantBeCast
//     continuous effects beside the printed-S: route, gated by the shared
//     CantBeCastParamsReadable whitelist, and the legal walk + bot policy
//     honour cantCast statics the same way they honour every other one;
//   - the "at the beginning of each of your upkeeps, copy this spell except
//     for its epic ability" clause is a self-re-arming one-shot API$
//     DelayedTrigger Phase registration (NextTurn$ True + the upkeep body's
//     identical re-arm), whose Execute body is api:CopySpellAbility Defined$
//     Self: a resolved Epic card keeps its compiled Ability across the stack
//     exit, so the arm mints a spell SNAPSHOT copy onto the stack through the
//     StackCopy event's "epic spell copy" Text arm (engine/events/apply_copy.go,
//     the "copy card for play" precedent). The copy is put on the stack, not
//     cast, so the can't-cast static never consults it, and MayChooseTarget$
//     True gives it the CR 707.10c new-targets election.
//
// The copy-exclusion and the countered-spell gate live in the arming VECTOR,
// not in a matcher: the arm is the resolving spell's own tail instruction
// (engine/rules/epic.go's epicArmTailFor, the Cipher shape), so a countered,
// fizzled or effect-bounced Epic never reaches it, and the splice site's
// !o.IsCopy guard (engine/rules/stack.go) keeps the upkeep copy's own
// resolution from re-arming -- so an armed game always has exactly one live
// Epic engine per resolved Epic spell.
//
// Registration-honesty: this registration lands together with the behaviour
// tests (engine/rules/edh_keywords_epic_test.go, the keyword_registration
// ratchet row) and the cmd/forgec coverage test that reported these cards
// unsupported.
func init() { RegisterNonAPI("kw:Epic") }
