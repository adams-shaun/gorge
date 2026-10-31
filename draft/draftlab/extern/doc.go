// Package extern connects gorge to out-of-process Magic agents over the
// gorge-agent/1 wire: one dialed session (a child's stdio or a Unix socket)
// serves any number of concurrent games, batched by the process-wide hub.
// The package holds the transports, the handshake and the batching hub, and
// turns a dialed Agent into the three draftlab value types a training
// workflow composes.
//
// # Roles
//
// A dialed agent becomes one of the three draftlab roles with
//
//	a.Drafter(l)  // draftlab.NamedDrafter  — answers a draft seat's picks
//	a.Builder(l)  // draftlab.NamedBuilder  — turns a pool into a deck
//	a.Player(l)   // draftlab.NamedPlayer   — plays engine games as a seat
//
// exactly where a built-in agent would: Lab.Draft, Lab.Build,
// Lab.PlayGame/Games/Play/Gauntlet/RunEvent all accept the returned values.
// An agent whose hello does not declare the role's kind is refused with
// ErrRole.
//
// # The agent contract
//
// An answer is a pure function of its item and the model version: batch
// composition, batch order and stream ids are scheduling-dependent and an
// agent must never let them change an answer. (The gorge-side results are
// then independent of Options.Workers, Flush and MaxBatch; the reference
// agent satisfies the contract.) A model that changes weights mid-session
// breaks the contract: model_version must change whenever the weights do.
//
// # Hidden information
//
// An agent receives only the deciding seat's own Seat-visibility view.View
// (one seat's PickView at a pick, one seat's pool at a build) and nothing
// else — never the engine, a botpolicy Board, a feed, a redeal or another
// seat's view. One process serving several seats (both seats of one game,
// or several seats of a pod) sees every such redacted view on its one
// connection and could combine them, exactly as one program running two
// built-in bots could: isolate seats with separate endpoints (one process
// per seat). The protocol never joins two seats' streams for the agent.
//
// # Batching and workers
//
// Set Options.Workers on the lab's draftlab.Options to the batch size you
// want (e.g. 64), not to the core count: one Agent serves every concurrent
// game, and a game's answers advance in lock-step rounds, so the batch size
// approaches the number of games blocked on the model.
//
// # Proving a run without the model
//
// Set draftlab.Options.RecordTape and use Lab.ReplayTape /
// Lab.VerifyEventTapes to re-drive the recorded games' accepted intents
// with no player policy at all — the model-free proof that a run's games
// replay byte for byte.
package extern
