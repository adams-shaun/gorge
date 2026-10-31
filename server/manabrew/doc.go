// Package manabrew is gorge's ManaBrew translator: a pure library that projects a seat-scoped view.View into the
// ManaBrew GameViewDto and maps a pending decision.Decision into the ManaBrew
// prompt of its decision.Kind.
//
// The package imports server/protocol/manabrew (wire types), view, decision and
// state (types only). It never imports rules, effects, events, host or
// bots/azmcts (archtest rows), never imports time, and never ranges
// over a map in a way whose order reaches the output: the maps the DTOs carry
// are encoded by encoding/json, which sorts map keys.
//
// The published ManaBrew protocol specification
// (https://docs.manabrew.app/protocol/) is licensed CC-BY-4.0; this package
// contains independently written Go code. See server/protocol/manabrew's
// ATTRIBUTION for the full attribution. Nothing here copies ManaBrew's AGPL
// reference implementation.
//
// Layout:
//
//	translator.go  the Translator: CardText seam, State, Prompt
//	ids.go         the id mint: players, cards, stack, hidden
//	state.go       view.View → GameViewDto, the step map,
//	               the x_gorge_own_deck_v1 own-deck extension
//	dispatch.go    the full switch over decision.Kinds
//	prompt_*.go    one file per decision kind
package manabrew
