package oracle

import (
	"os"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
)

const nivixGuildmageRepro = "nivix_guildmage_gap_search_budget_exhausted"

// CR 601.2g: Nivix Guildmage's cast costs {1}{U}{R}. With five Islands each
// watched by two of the payer's own Scalds, every Island tap queues two
// triggers, which gorge's Taps-deferred partial walk prices (one trigger per
// tap) as no script, so the autopay surface withholds the cast; the kernel's
// mirror taps the Island and offers it (tail-49 pair 88, decision 677,
// "options options"). The card is refused by name and the recorded game still
// shows the divergence with it implicated.
func TestMBCNivixGuildmageReprosLockstep(t *testing.T) {
	entries, err := cardset.ReadRefused(os.DirFS(moduleRootFromOracle))
	if err != nil {
		t.Fatal(err)
	}
	for i := range entries {
		e := &entries[i]
		if e.Card != "Nivix Guildmage" {
			continue
		}
		if e.Repro.Pool != "custom" || e.Repro.File != nivixGuildmageRepro {
			t.Fatalf("Nivix Guildmage's entry replays %s/%s, want custom/%s", e.Repro.Pool, e.Repro.File, nivixGuildmageRepro)
		}
		checkRefusedRepro(t, e)
		return
	}
	t.Fatal("Nivix Guildmage has no dirac/support/refused entry: gorge's Taps-deferred walk prices no script beside two Scalds")
}
