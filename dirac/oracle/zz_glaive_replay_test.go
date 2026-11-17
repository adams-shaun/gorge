package oracle

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"unsafe"

	"github.com/adams-shaun/gorge/engine/decision"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/rules"
)

// Scratch diagnosis of the r13 lockstep repro (Lashwrithe equip offer,
// decision 943): replays the recorded game with gorge's observer-only window
// diagnostics on, then dumps the divergent decision's raw options, window
// reasons and boards. Not committed: deleted before the fix's commit.
// GORGE_R13_REPLAY=1 arms it.

func setWindowDiag(e *rules.Engine) {
	f := reflect.ValueOf(e).Elem().FieldByName("windowDiagnostics")
	if !f.IsValid() {
		panic("no windowDiagnostics field")
	}
	*(*bool)(unsafe.Pointer(f.UnsafeAddr())) = true
}

func TestZZR13Replay(t *testing.T) {
	if os.Getenv("GORGE_R13_REPLAY") == "" {
		t.Skip("replay env not set")
	}
	w, _ := os.Getwd()
	w = filepath.Dir(filepath.Dir(w))
	r, err := ReadRepro(w + "/.ds4/scratch/r13/repro.json")
	if err != nil {
		t.Fatal(err)
	}
	reg := synthReg(t)
	set := cardset.Raw(globSet(t, r.Pool, r))
	m, err := r.Matchup(reg)
	if err != nil {
		t.Fatal(err)
	}
	entries := func(names []string) []cardset.Entry {
		es := make([]cardset.Entry, len(names))
		for i, n := range names {
			es[i] = cardset.Entry{Name: n, Count: 1}
		}
		return es
	}
	tbl, _, _, err := set.Matchup(entries(r.Cards[0]), entries(r.Cards[1]))
	if err != nil {
		t.Fatal(err)
	}
	rc := r.RunConfig()
	rc.Config.GorgeAutopaySurface = r.Surface != "full"
	rc.MaxDecisions = len(r.Actions) + 200
	lead, err := NewGorgeSide(m, r.Seed, rc.Config, nil)
	if err != nil {
		t.Fatal(err)
	}
	setWindowDiag(lead.E)
	k, err := SetSide(set, tbl, rc.Config)(m, r.Seed)
	if err != nil {
		t.Fatal(err)
	}
	defer CloseSide(k)
	calls := 0
	rc.Check = func(ref *GorgeSide, kk Side) *Divergence {
		calls++
		if calls != 943 {
			return nil
		}
		rs := kk.(*realSide)
		for _, ci := range []int{18, 27} {
			t.Logf("kernel card %d = %q", ci, rs.gname[ci])
			for j := range rs.tbl.Cards[ci].Abilities {
				a := &rs.tbl.Cards[ci].Abilities[j]
				mn := a.Cost.Mana
				t.Logf("  ability %d kind=%v cost g=%d pips=%v tb=%d hyb=%v flags=%v manaColors=%d manaN=%d",
					j, a.Kind, mn.Generic, mn.Pips, mn.Tb, mn.Hyb, a.Cost.Flags, a.ManaColors, a.ManaN)
			}
		}
		return nil
	}
	res := LockstepLed(m, r.Seed, lead, k, &ScriptPolicy{Actions: r.Actions}, rc)
	if res.Div == nil {
		t.Fatalf("expected the recorded divergence, got clean (decisions %d, stall %q)", res.Decisions, res.Stall)
	}
	t.Logf("divergence at %d: %s %s %v", res.Div.At, res.Div.What, res.Div.Field, res.Div.Diff)
	if res.Div.What == "kernel" {
		t.Logf("kernel detail: %s", res.Div.Detail)
	}
	t.Logf("implicates: %v", res.Div.Implicates(m))
	if res.Div.At != 943 {
		t.Fatalf("diverged at %d, expected 943", res.Div.At)
	}
	d := lead.E.Pending()
	if d == nil {
		t.Fatal("no pending decision on the gorge side")
	}
	t.Logf("pending decision kind=%v player=%d options=%d", d.Kind, d.Player, len(d.Options))
	// Does gorge's own script search fund the equip (obj 14, ability 0)?
	script, verdict := lead.E.PotentialPlayScript(0, decision.PotentialAction{Kind: "ability", Obj: 14, Ability: 0}, 160)
	t.Logf("PotentialPlayScript(0, ability#14/0): verdict=%q script steps=%d", verdict, len(script))
	for i, st := range script {
		picks := ""
		for _, pk := range st.Picks {
			picks += fmt.Sprintf(" [%s obj=%d ab=%d sym=%q]", pk.Kind, pk.Obj, pk.Ability, pk.ManaSymbol)
		}
		t.Logf("  step %d kind=%v picks=%s", i, st.Kind, picks)
	}
	var pr, pk Projection
	lead.Project(&pr)
	if rs, ok := k.(*realSide); ok {
		rs.Project(&pk)
		t.Logf("proj equal=%v pr.Pool=%v pk.Pool=%v pr.Life=%v pk.Life=%v", pr == pk, pr.Pool, pk.Pool, pr.Life, pk.Life)
	}
	t.Logf("gorge raw pool p0=%v p1=%v", lead.E.G.Players[0].Pool, lead.E.G.Players[1].Pool)
	for _, wr := range d.WindowReasons {
		t.Logf("window: obj=%d kind=%s reason=%s", wr.Obj, wr.Kind, wr.Reason)
	}
	optionsDebug(lead)
}
