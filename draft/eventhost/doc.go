// Package eventhost is the draft-event runtime that lives inside gorged:
// the event store (<dir>/events/<id>/event.json + log.jsonl), create and
// join, the DRAFT phase (humans pick over the API, bots pick
// synchronously and deterministically between their picks), BUILD, the
// rounds on host tables, per-event SSE frames, and restart resume for every
// phase.
//
// # Series
//
// An event is best-of-one (the default) or best-of-three (create's
// best_of: 3). A Bo3 event's series run on one host series table each:
// advanceSeriesBo3 logs the series' entries (SeriesStarted, each game's
// GameStarted with its play/draw chooser — game 1 the table's toss winner,
// later games MTR 2.2's loser-chooses rule, bots' Sideboarded between
// games, the series' SeriesEnded), and the table's hook — Host.Series,
// reached through gorged's Relay — plans each match from the event's own
// state: the in-flight game's decks and chooser, a stop once the series
// has ended or moved to a new table, or a park on the run (answered by
// wakeWaiters when the log moves) until the next GameStarted is logged.
// A crashed series match aborts its attempt and restarts the game on a
// fresh table; past MaxAttempts the game logs as a draw and the series
// goes on. A Bo3 series with a human side waits between games for that
// seat's sideboarding submission, with the bot side already locked. The
// lock order extends the host's rule: Host.Series
// takes Host.mu alone to find the run, then run.mu alone; a parked call
// waits holding nothing, and advance answers the waiters (in sorted
// table-id order) under run.mu.
//
// The package never imports net/http: handlers read an Error's HTTP status
// and code through the Status/Code methods.
//
// # Concurrency
//
// One mutex guards the whole Host (map of runs, closing flag) and one mutex
// guards each run. Host.mu is a leaf: no code path holding a run.mu may
// take Host.mu, so the exported methods take Host.mu only to find the run,
// release it, and then take run.mu for the whole call. run.mu also orders
// against the host.Registry (Session, Publish, AddTable, Start):
// take run.mu first, call the Registry beneath it, never the reverse.
//
// # Clock
//
// eventhost is the only package of the event stack that may read the
// clock. The abandonment sweep reads it through Options.Now.
// The pick timers read and arm through Options.Clock (Now and
// AfterFunc; nil = the real clock): in a pod with two or more humans every
// human pick has a deadline — XMage's table by the cards left in the pack,
// ×1.5 relaxed, replaced by a positive Options.PickTimeout — and an
// expired pick is taken by the seat's own drafter and logged as
// Picked{Auto: true}. Single-human pods have no timer at all. Nothing
// about a deadline is persisted: a restart re-arms every timer with its
// full limit, and replay never reads the clock.
package eventhost
