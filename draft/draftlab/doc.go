// Package draftlab is the Go composition API a training program imports
// (draft events): open a card corpus,
// resolve built-in drafters, builders and players by name@version, open
// packs (pod, flat, sealed pool), run a draft of 4 or 8 seats into pools
// plus a replayable event log, and turn pools or lists into
// engine-loadable decks.
//
// Every call is a pure function of its inputs and seed: the same (Lab,
// arguments, seed) triple yields byte-identical results, and the
// Options.Workers count never reaches a result. The corpus registry is
// read-only after Open (cards.SharedCorpus); nothing here mutates game
// state — playing games is E5-c2's seam, gauntlet/events/manifests are
// E5-c3's, CSV artifacts the demo is E5-c4's, and the bench Env driver
// that lifts the Env refusal is E5-c5's.
//
// Agents are named "name" or "name@version". A ref without a version
// resolves to the version this build's table lists for the agent (or
// "unversioned" for a name the table does not list); an explicit version
// must equal the table's, so a training run's manifest names exactly the
// behaviour it ran. Built-in agents are fingerprinted
// (testdata/agent_fingerprints.json): a behaviour change must bump the
// agent's version in the table and regenerate the golden.
//
// Env rule: a Player policy whose bots.Info.Env is true reads every seat's
// REAL decklist, so it is refused unless Options.AllowDecklistBots was set
// (a bot-only opt-in, by design). With the
// opt-in such a game runs through the bench Env driver
// (bench.Hooks{EnvSeats: true, HonestRoot: true}), so the hosted wrapper is
// driven with its bots.Env — the honest root, never the live engine — and
// its result is marked for the manifest (E5-c3's AllowDecklistBots).
//
// The E5-d seam (out-of-process agents): an external drafter/builder plugs
// in as a pickbot.Drafter / pickbot.Builder adapter and may implement
// FallibleDrafter / FallibleBuilder to report a failure; adapters describe
// cards through Lab.Card; a game seat whose answers are batched across
// concurrent games implements GameScoped and is told each game's begin and
// end; and Options.RecordTape keeps each game's accepted intents in
// GameResult.Tape, so ReplayTape / VerifyEventTapes re-drive a game with no
// player policy at all.
//
// The package imports the engine tier freely (bots, seat, event, deck,
// limited) but never the wall clock: no `time`, no legacy math/rand —
// every stream is a seeded math/rand/v2 PCG, and no map iteration order
// reaches a result.
package draftlab
