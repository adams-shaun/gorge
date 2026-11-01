package host

// The hosted-policy contract harness: the gates every hosted bot policy owes
// the host (docs/design/2026-09-28-hosted-bot-packages.md §5.3 and §8, and the
// closed-list privacy proof), exported so a module that registers its own
// policies runs EXACTLY the harness
// gorge runs on the policies it links, through the real host internals the
// properties are about. gorge's own rows (server/host/*_test.go) call these
// same functions.
//
// It is test support, never called by the server: nothing in gorged reaches
// it, so the linker drops it from every binary. Its exported surface is kept
// to entry points and their configs; no signature carries an engine or a
// game (internal/archtest TestNoExportLeaksAnEngineGame), and the harness
// takes a ContractT rather than importing package testing.
//
//   - RunContractTable (contract.go): one table to completion, its log
//     replayed to the recorded head. The determinism rows are two of these.
//   - RunEnvLeakContract (contract_envleak.go): the hidden-card swap row.
//   - RunClosedPrivacyContract (contract_privacy.go): the closed-list
//     privacy proof against a human seat.
//   - RunShadowParityContract (contract_shadow.go): the shadow-scan parity
//     row.

import (
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/game/gorgegame"
	"github.com/adams-shaun/gorge/engine/replay"
	"github.com/adams-shaun/gorge/server/protocol"
)

// ContractT is the part of testing.TB the contract harness uses; a
// *testing.T satisfies it.
type ContractT interface {
	Helper()
	Logf(format string, args ...any)
	Errorf(format string, args ...any)
	Fatalf(format string, args ...any)
	Cleanup(func())
}

// ContractRun is one table's finished match as RunContractTable saw it.
type ContractRun struct {
	Info protocol.MatchInfo
	// Log is a copy of the match's event log (seed, events, intents).
	Log *events.Log
	// Reason is the match's end or crash reason.
	Reason string
	// RootRefusals counts the honest roots the host refused (BP-08).
	RootRefusals int
	// Feeds reports whether the match kept observation feeds (an Env seat).
	Feeds bool
}

// RunContractTable builds a registry from o, adds cfg, plays its first match
// to completion, and replays the match's log under the match's own rules
// configuration: the replay must reach the recorded head (a failure is
// fatal). The registry is closed before it returns.
func RunContractTable(t ContractT, o Options, cfg TableConfig) ContractRun {
	t.Helper()
	r, err := New(o)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer r.Close()
	if err := r.AddTable(cfg); err != nil {
		t.Fatalf("%q: AddTable: %v", cfg.BotPolicy, err)
	}
	if err := r.Start(cfg.ID); err != nil {
		t.Fatalf("%q: Start: %v", cfg.BotPolicy, err)
	}
	r.Wait(cfg.ID)
	r.mu.RLock()
	tbl := r.tables[cfg.ID]
	r.mu.RUnlock()
	tbl.mu.RLock()
	if len(tbl.history) != 1 {
		n := len(tbl.history)
		tbl.mu.RUnlock()
		t.Fatalf("%q: table kept %d matches, want 1", cfg.BotPolicy, n)
	}
	m := tbl.history[0]
	tbl.mu.RUnlock()
	m.mu.RLock()
	run := ContractRun{Info: m.info(), Log: m.e.L.Clone(), Reason: m.reason, RootRefusals: m.rootRefusals, Feeds: m.feeds != nil}
	rulesCfg := gorgegame.RulesConfig(m.gc)
	m.mu.RUnlock()
	t.Logf("%s: %s turn %d intents %d events %d rootRefusals %d feeds %v reason %q", cfg.Name, run.Info.State, run.Info.Turns,
		len(run.Log.Intents), len(run.Log.Events), run.RootRefusals, run.Feeds, run.Reason)
	replayed, err := replay.Replay(run.Log, rulesCfg)
	if err != nil {
		t.Fatalf("%q replay: %v", cfg.BotPolicy, err)
	}
	if got := replayed.L.Head(); got != run.Info.Head {
		t.Fatalf("%q replay head %s, want %s", cfg.BotPolicy, got, run.Info.Head)
	}
	return run
}
