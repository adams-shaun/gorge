// Package cardcov is the per-card coverage inventory and use probe the
// card-fuzzing harnesses share: which of a card's own abilities (activated,
// mana, non-bookkeeping triggered, and each mode of a modal spell or ability)
// a finished game cast, played, offered or used.
//
// It moved here from cmd/cardfuzz so a second driver (mtgsim) can credit the
// same keys. Key forms: "cast", "land", "f<face>/a<i>" (activated, mana
// included), "f<face>/t<i>" (triggered), and the mode keys of ModeKeys,
// "f<face>/s/m<k>" for a modal spell and "<slotkey>/m<k>" for a modal slot.
//
// dirac/oracle.CoverageKeys is a hand port of Keys that must keep agreeing.
package cardcov
