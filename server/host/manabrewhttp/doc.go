// Package manabrewhttp is the
// wire transport that carries server/manabrew's translation over HTTP.
//
// It is stdlib-only SSE plus POST, at the host tier: it drives a
// *host.Registry exactly as server/host/httpapi does (ViewAtSeat, Pending,
// SubmitIntent, Undo), and never touches the engine directly. A connection
// is bound to one host/httpapi.SeatClaim through an injected resolver
// (Options.Seat), so a token can never read or act as a seat other than its
// own claim — the same fence claimForTable enforces in host/httpapi.
//
// Support is opt-in: nothing in this package is imported by any engine
// package, server/host/httpapi, or the rest of the host tier (internal/archtest
// pins the reverse rows), and cmd/gorged decides whether to mount it
// at all. An unmounted server serves no ManaBrew route.
//
// Routes, mounted by NewHandler under the caller's own prefix:
//
//   - GET  .../stream — one SSE connection per client: the current state,
//     the open prompt if one exists, and a live feed of engine→client
//     messages as the match progresses. It never replays history.
//   - POST .../send   — one ClientToServerMessage: a response is validated
//     against the pending decision and turned into a decision.Intent
//     through SubmitIntent; a directive queues or answers a concession;
//     restoreSnapshot asks for host.Registry.Undo. A rejection is returned
//     in the HTTP body and, when a stream is open, pushed on it too.
//   - GET  .../state  — one-shot state plus the open prompt, for poll-only
//     clients that never open a stream.
//
// Gorge extension: every
// state message's gameView carries the additive optional member
// `x_gorge_own_deck_v1` — the authenticated seat's own genesis deck manifest
// as name/count rows, the same data the native view's `own_deck` member
// holds. It is not part of the published ManaBrew protocol; clients that do
// not know it ignore it, and prompts never carry it.
package manabrewhttp
