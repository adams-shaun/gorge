package host

import (
	"testing"
)

// TestMatchJournalSyncFollowsHumans pins the per-match journal durability
// rule: the registry-wide Options.Sync still gates everything, and within it
// only a match with a human seat pays the per-burst journal fsyncs. Each
// match is built through the real newMatchWith, so the table config is the
// same one the live path derives the rule from.
func TestMatchJournalSyncFollowsHumans(t *testing.T) {
	build := func(t *testing.T, o Options, cfg TableConfig) (*Registry, *match) {
		t.Helper()
		r, err := New(o)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { r.Close() })
		m, err := r.newMatchWith(newTable(r, cfg), 1, nil)
		if err != nil {
			t.Fatal(err)
		}
		if m.files == nil {
			t.Fatalf("precondition: Dir=%q left the match without journal files", o.Dir)
		}
		t.Cleanup(m.files.close)
		return r, m
	}

	// Registry durability on: shape alone decides between all-bot and human.
	botOpts := diskOptions(t, t.TempDir())
	botOpts.Sync = true
	botReg, bot := build(t, botOpts, fourSeatTable("t2", false))

	humanCfg := fourSeatTable("t3", false)
	humanCfg.Humans = []int{0}
	humanOpts := diskOptions(t, t.TempDir())
	humanOpts.Sync = true
	humanReg, human := build(t, humanOpts, humanCfg)

	// Registry durability off: the global opt still gates the human path.
	optOutCfg := fourSeatTable("t4", false)
	optOutCfg.Humans = []int{0}
	optOutOpts := diskOptions(t, t.TempDir())
	optOutOpts.Sync = false
	optOutReg, optOut := build(t, optOutOpts, optOutCfg)

	if !botReg.opts.Sync || !humanReg.opts.Sync {
		t.Fatalf("precondition: registry Sync is off (bot=%v human=%v), the shape comparison would be vacuous", botReg.opts.Sync, humanReg.opts.Sync)
	}
	if bot.files.sync {
		t.Fatalf("all-bot match journal sync = true, want false")
	}
	if !human.files.sync {
		t.Fatalf("human-seat match journal sync = false, want true (Humans=%v, Options.Sync=%v)", humanCfg.Humans, humanReg.opts.Sync)
	}
	if bot.files.sync == human.files.sync {
		t.Fatalf("precondition: both table shapes produced journal sync=%v", bot.files.sync)
	}
	if optOut.files.sync {
		t.Fatalf("human-seat match journal sync = true with Options.Sync=false, want false")
	}
	if optOutReg.opts.Sync {
		t.Fatalf("precondition: Options.Sync is on, the registry-gate case would be vacuous")
	}

	// The deferred-sync journal must still leave byte-identical, replayable
	// files: close the all-bot match cleanly and replay its genesis burst.
	bot.files.close()
	want := bot.g.Log().Events[:int(bot.bounds[0])]
	if len(want) == 0 {
		t.Fatal("precondition: the all-bot match has no genesis burst to replay")
	}
	l, err := readLog(botOpts.Dir, "t2", 1)
	if err != nil {
		t.Fatalf("readLog after deferred-sync close: %v", err)
	}
	if len(l.Events) != len(want) {
		t.Fatalf("replayed %d genesis events, want %d", len(l.Events), len(want))
	}
	for i := range want {
		if l.Events[i].Seq != want[i].Seq || l.Events[i].Kind != want[i].Kind {
			t.Fatalf("replayed event %d = seq %d kind %d, want seq %d kind %d", i, l.Events[i].Seq, l.Events[i].Kind, want[i].Seq, want[i].Kind)
		}
	}
	if got, wantHead := l.Head(), bot.g.Log().HeadAt(len(want)); got != wantHead {
		t.Fatalf("replayed genesis head = %s, want %s", got, wantHead)
	}
}
