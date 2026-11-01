package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// exwStatScen is a table with a source creature and exiled cards of several
// printed types, plus the exile filter Card.ExiledWithSource (FExiledWith).
func exwStatScen(t *testing.T) (sc *scen, f kop.FilterID, cardOf map[string]CardID) {
	t.Helper()
	deck := []cardset.Entry{{Name: "Plains", Count: 20}}
	fdnSet := cardsettest.Set(t, "fdn")
	tbl, _, _, err := fdnSet.Matchup(deck, deck)
	if err != nil {
		t.Fatal(err)
	}
	n := CardID(len(tbl.Cards))
	tbl.Cards = append(tbl.Cards,
		kop.CardDef{Name: "T Host", ExiledWith: true, Types: kop.TCreature, Power: 3, Toughness: 3},
		kop.CardDef{Name: "T Land", Types: kop.TLand},
		kop.CardDef{Name: "T Golem", Types: kop.TArtifact | kop.TCreature, Power: 1, Toughness: 1},
		kop.CardDef{Name: "T Aura", Types: kop.TEnchantment},
		kop.CardDef{Name: "T Bear", Types: kop.TCreature, Power: 2, Toughness: 2})
	cardOf = map[string]CardID{"host": n, "land": n + 1, "golem": n + 2, "aura": n + 3, "bear": n + 4}
	tbl.Filters = append(tbl.Filters, kop.Filter{Zone: kop.ZoneExile, Flags: kop.FExiledWith})
	f = kop.FilterID(len(tbl.Filters) - 1)
	return newScenTable(t, tbl, Config{NoElide: true}), f, cardOf
}

// exwExile exiles card c (owned by owner) as a forward-linked card of src.
func exwExile(sc *scen, c CardID, owner uint8, src Obj) Obj {
	o := sc.obj(c, owner)
	sc.s.place(o, kop.ZoneExile, false)
	if src != NoObj {
		sc.s.cold.exw[sc.s.g.nexw] = exwRec{o: o, src: src, sver: sc.s.objs[src].ver, fl: exwFwd}
		sc.s.g.nexw++
	}
	return o
}

// TestExileTypesAgg: ExCountExile N 1 is the number of distinct card types
// (CR 205.2a) among the exiled cards linked to the static's source, both
// seats' exile zones; a card exiled by nothing adds none, and N 0 still counts
// cards (Keen-Eyed Curator).
func TestExileTypesAgg(t *testing.T) {
	sc, f, c := exwStatScen(t)
	host := sc.perm(c["host"], 0)
	ctx := evalCtx{src: host, srcSl: sc.slot(host), ctrl: 0}
	types := kop.Expr{Kind: kop.ExCountExile, N: 1, F: f}
	count := kop.Expr{Kind: kop.ExCountExile, F: f}
	read := func(x *kop.Expr) int { return sc.s.exprCold(sc.k, x, ctx) }
	if read(&types) != 0 || read(&count) != 0 {
		t.Fatal("nothing exiled must read 0")
	}
	exwExile(sc, c["land"], 0, host)
	exwExile(sc, c["bear"], 1, host) // the other seat's exile zone is read too
	exwExile(sc, c["bear"], 0, host) // a second creature adds no type
	if got := read(&types); got != 2 {
		t.Errorf("land + two creatures: %d card types, want 2", got)
	}
	exwExile(sc, c["golem"], 0, host) // artifact creature: artifact is new
	exwExile(sc, c["aura"], 1, NoObj) // exiled by nothing: reads as not linked
	if got := read(&types); got != 3 {
		t.Errorf("+ an artifact creature and an unlinked enchantment: %d card types, want 3", got)
	}
	exwExile(sc, c["aura"], 0, host)
	if got := read(&types); got != 4 {
		t.Errorf("+ a linked enchantment: %d card types, want 4", got)
	}
	if got := read(&count); got != 5 {
		t.Errorf("N 0 counts the five linked cards, got %d", got)
	}
	other := sc.perm(c["host"], 0)
	oc := evalCtx{src: other, srcSl: sc.slot(other), ctrl: 0}
	if got := sc.s.exprCold(sc.k, &types, oc); got != 0 {
		t.Errorf("another source's link: %d card types, want 0", got)
	}
}

// TestExwStaticSource: the FExiledWith flag takes its source from the context
// a static passes; another source or no context matches nothing, and the
// switch the lockstep mutation flips drops the source.
func TestExwStaticSource(t *testing.T) {
	sc, f, c := exwStatScen(t)
	a := sc.perm(c["host"], 0)
	b := sc.perm(c["host"], 0)
	o := exwExile(sc, c["bear"], 0, a)
	own := evalCtx{src: a, srcSl: sc.slot(a), ctrl: 0}
	oth := evalCtx{src: b, srcSl: sc.slot(b), ctrl: 0}
	match := func(cx *evalCtx) bool { return sc.s.matchCardGenAt(sc.k, f, c["bear"], 0, 0, o, cx) }
	if !match(&own) {
		t.Error("the static's own link must match")
	}
	if match(&oth) {
		t.Error("another source's context must not match")
	}
	if match(nil) {
		t.Error("no context and no resolution must match nothing")
	}
	if sc.s.matchCardGenAt(sc.k, f, c["bear"], 0, 0, NoObj, &own) {
		t.Error("an unknown object must not match")
	}
	DebugExwStaticNoSrc = true
	if match(&own) {
		t.Error("DebugExwStaticNoSrc must drop the source")
	}
	DebugExwStaticNoSrc = false
	// a card that leaves exile is a new object for the association (gorge
	// prunes every source's forward list): back in exile it is not a's
	sc.s.exwLeaveExile(o)
	if match(&own) {
		t.Error("a card that left exile must not stay linked through the forward list")
	}
}

// TestExwStaticsCarriers pins the generated carriers: Dawnhand Dissident's
// permission removes three counters and Quilled Greatwurm's six (Static.A, the
// count the five hardcoded sixes used to be), and Keen-Eyed Curator and
// Maralen read the link through the filter flag (a missed regeneration fails).
func TestExwStaticsCarriers(t *testing.T) {
	deck := []cardset.Entry{{Name: "Dawnhand Dissident", Count: 10}, {Name: "Maralen, Fae Ascendant", Count: 10}, {Name: "Keen-Eyed Curator", Count: 10}, {Name: "Forest", Count: 10}}
	kexwSet := cardsettest.Set(t, "exw")
	tbl, _, _, err := kexwSet.Matchup(deck, deck)
	if err != nil {
		t.Fatal(err)
	}
	fdnSet := cardsettest.Set(t, "fdn")
	fdn, _, _, err := fdnSet.Matchup([]cardset.Entry{{Name: "Quilled Greatwurm", Count: 40}}, []cardset.Entry{{Name: "Quilled Greatwurm", Count: 40}})
	if err != nil {
		t.Fatal(err)
	}
	may := func(tb *kop.Table, name string) (kop.Static, bool) {
		for i := range tb.Cards {
			if tb.Cards[i].Name != name {
				continue
			}
			for _, a := range tb.Cards[i].Abilities {
				if a.Kind == kop.AbStatic && a.Static.Kind == kop.StatMayPlayFrom {
					return a.Static, true
				}
			}
		}
		return kop.Static{}, false
	}
	st, ok := may(tbl, "Dawnhand Dissident")
	if !ok || st.A != 3 || st.StFlags&kop.StRemove6Counters == 0 {
		t.Errorf("Dawnhand Dissident: static %+v, want A 3 with the counter flag", st)
	}
	if fl := tbl.Filters[st.Filter].Flags; fl&kop.FExiledWith == 0 {
		t.Errorf("Dawnhand Dissident: filter flags %#x lack FExiledWith", fl)
	}
	st, ok = may(fdn, "Quilled Greatwurm")
	if !ok || st.A != 6 || st.StFlags&kop.StRemove6Counters == 0 {
		t.Errorf("Quilled Greatwurm: static %+v, want A 6 with the counter flag", st)
	}
	st, ok = may(tbl, "Maralen, Fae Ascendant")
	if !ok || st.StFlags&kop.StOncePerTurn == 0 || st.StFlags&kop.StWithoutManaCost == 0 {
		t.Errorf("Maralen: static %+v, want OncePerTurn + WithoutManaCost", st)
	}
	if fl := tbl.Filters[st.Filter].Flags; fl&kop.FExiledWith == 0 || tbl.Filters[st.Filter].Pred != kop.PredCMCLEV {
		t.Errorf("Maralen: filter %+v, want FExiledWith with a PredCMCLEV bound", tbl.Filters[st.Filter])
	}
}
