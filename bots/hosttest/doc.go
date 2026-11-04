// Package hosttest holds the host-level gates of the hosted bot policies
// cmd/gorged links on top of bots/all (search, sb-search-lite-atk,
// sb-heuristic, sb-uniform, sb-first, cast-profile, az-c3, az-c4, az-c5), run
// through the REAL host.
//
// It has no code of its own. Each property that needs host internals is
// server/host's exported contract harness (contract*.go: RunContractTable,
// RunEnvLeakContract, RunClosedPrivacyContract, RunShadowParityContract,
// RunCaretakerLifePlanContract), the same functions server/host runs on the
// policies bots/all links; the rest go through the host's public API
// (host.New, Registry.AddTable/Start/Wait/Matches/SnapshotMatch,
// host.NormalizeBotPolicy and the NewBotPolicySeat factories). The policies
// live here rather than in bots/all so server/host's own registry-wide gates
// stay the size of the built-in set.
package hosttest
