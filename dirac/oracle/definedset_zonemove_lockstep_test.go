package oracle

import (
	"slices"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kgen"
	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/engine/cards"
)

// A qualified Defined$ object set under a ChangeZone move (kgen
// build_setexpr.go kopRunsSetObject). Under Exile, Bounce and ToLibrary the
// kernel's set read lost the move's origin and identity guards (CR 400.7)
// and played divergent, so the builder refuses those shapes; under Return
// (graveyard to hand, graveyard to battlefield) the set still builds and
// plays as gorge does.

const (
	zmUpkeep  = "T:Mode$ Phase | Phase$ Upkeep | ValidPlayer$ You | TriggerZones$ Battlefield | Execute$ ZmMove\n"
	zmWatcher = "T:Mode$ ChangesZone | Origin$ Battlefield | Destination$ Graveyard | ValidCard$ Creature.YouCtrl | TriggerZones$ Battlefield | Execute$ ZmMove\n"
)

func zmCarrier(name, trig, body string) string {
	return "Name:" + name + "\nManaCost:1\nTypes:Creature Bear\nPT:2/2\n" + trig + "SVar:ZmMove:" + body + "\n"
}

// zmRefused are the shapes that diverged (2026-11-28 probe), each with the
// kop the builder names in its refusal.
var zmRefused = []struct{ name, src, op string }{
	{"Zm Self Hand", zmCarrier("Zm Self Hand", zmUpkeep, "DB$ ChangeZone | Origin$ Battlefield | Destination$ Hand | Defined$ Self.Creature"), "Bounce"},
	{"Zm Self Top", zmCarrier("Zm Self Top", zmUpkeep, "DB$ ChangeZone | Origin$ Battlefield | Destination$ Library | LibraryPosition$ 0 | Defined$ Self.Creature"), "ToLibrary"},
	{"Zm Self Bottom", zmCarrier("Zm Self Bottom", zmUpkeep, "DB$ ChangeZone | Origin$ Battlefield | Destination$ Library | LibraryPosition$ -1 | Defined$ Self.Creature"), "ToLibrary"},
	{"Zm Self Exile", zmCarrier("Zm Self Exile", zmUpkeep, "DB$ ChangeZone | Origin$ Battlefield | Destination$ Exile | Defined$ Self.Creature"), "Exile"},
	{"Zm Attacker Top", zmCarrier("Zm Attacker Top", zmWatcher, "DB$ ChangeZone | Origin$ Graveyard | Destination$ Library | LibraryPosition$ 0 | Defined$ TriggeredAttacker.Creature"), "ToLibrary"},
	{"Zm Attacker Bottom", zmCarrier("Zm Attacker Bottom", zmWatcher, "DB$ ChangeZone | Origin$ Graveyard | Destination$ Library | LibraryPosition$ -1 | Defined$ TriggeredAttacker.Creature"), "ToLibrary"},
	{"Zm Attacker Exile", zmCarrier("Zm Attacker Exile", zmWatcher, "DB$ ChangeZone | Origin$ Graveyard | Destination$ Exile | Defined$ TriggeredAttacker.Creature"), "Exile"},
	{"Zm Cards Exile", zmCarrier("Zm Cards Exile", zmWatcher, "DB$ ChangeZone | Origin$ Graveyard | Destination$ Exile | Defined$ TriggeredCards.Creature"), "Exile"},
}

// zmBuilt are the set shapes that still build (each reads a set), beside the
// unqualified controls and an exiling watcher that races them.
var zmBuilt = []struct {
	name, src string
	set       bool
}{
	{"Zm Attacker Hand", zmCarrier("Zm Attacker Hand", zmWatcher, "DB$ ChangeZone | Origin$ Graveyard | Destination$ Hand | Defined$ TriggeredAttacker.Creature"), true},
	{"Zm Cards Hand", zmCarrier("Zm Cards Hand", zmWatcher, "DB$ ChangeZone | Origin$ Graveyard | Destination$ Hand | Defined$ TriggeredCards.Creature"), true},
	{"Zm Own Return", zmCarrier("Zm Own Return", "T:Mode$ ChangesZone | Origin$ Battlefield | Destination$ Graveyard | ValidCard$ Card.Self | Execute$ ZmMove\n",
		"DB$ ChangeZone | Origin$ Graveyard | Destination$ Battlefield | Defined$ Self.YouOwn"), true},
	{"Zm Plain Hand", zmCarrier("Zm Plain Hand", zmUpkeep, "DB$ ChangeZone | Origin$ Battlefield | Destination$ Hand | Defined$ Self"), false},
	{"Zm Plain Exile", zmCarrier("Zm Plain Exile", zmWatcher, "DB$ ChangeZone | Origin$ Graveyard | Destination$ Exile | Defined$ TriggeredCards"), false},
}

var zmPool = []string{"Grizzly Bears", "Glory Seeker", "Savannah Lions", "Shock", "Mountain", "Plains"}

var zmDecks = [2][]cardset.Entry{
	{
		{Name: "Zm Attacker Hand", Count: 3}, {Name: "Zm Own Return", Count: 3}, {Name: "Zm Plain Hand", Count: 3}, {Name: "Zm Plain Exile", Count: 3},
		{Name: "Grizzly Bears", Count: 4}, {Name: "Glory Seeker", Count: 4}, {Name: "Savannah Lions", Count: 2}, {Name: "Shock", Count: 4},
		{Name: "Mountain", Count: 7}, {Name: "Plains", Count: 7},
	},
	{
		{Name: "Zm Cards Hand", Count: 3}, {Name: "Zm Own Return", Count: 3}, {Name: "Zm Plain Exile", Count: 3}, {Name: "Zm Attacker Hand", Count: 3},
		{Name: "Grizzly Bears", Count: 4}, {Name: "Glory Seeker", Count: 4}, {Name: "Savannah Lions", Count: 2}, {Name: "Shock", Count: 4},
		{Name: "Mountain", Count: 7}, {Name: "Plains", Count: 7},
	},
}

// TestDefinedSetZoneMoveLockstep: every diverging shape is refused by the
// builder on its kop, every set shape that builds reads a set, and over seeds
// 1-6 the built carriers play with no divergence while a set runs.
func TestDefinedSetZoneMoveLockstep(t *testing.T) {
	if isolate(t) {
		return
	}
	reg, err := cards.SharedCorpus(corpusDirOrSkip(t))
	if err != nil {
		t.Fatal(err)
	}
	byName := map[string]*cards.Card{}
	var all []*cards.Card
	for _, n := range zmPool {
		c, ok := reg.Lookup(n)
		if !ok {
			t.Fatalf("%s not in the corpus", n)
		}
		byName[n] = c
		all = append(all, c)
	}
	var srcs []string
	for _, r := range zmRefused {
		srcs = append(srcs, r.src)
	}
	for _, b := range zmBuilt {
		srcs = append(srcs, b.src)
	}
	for _, src := range srcs {
		c, diags := cards.ParseBytes("zonemove_probe.txt", []byte(src))
		if c == nil || len(diags) != 0 {
			t.Fatalf("parse: %v", diags)
		}
		if d := c.Link(); len(d) != 0 {
			t.Fatalf("link: %v", d)
		}
		byName[c.Faces[0].Name] = c
		all = append(all, c)
	}
	pd, err := kgen.TableFromCards(reg, all, "definedset-zonemove")
	if err != nil {
		t.Fatal(err)
	}
	entry := func(name string) int {
		i := slices.Index(pd.Names, name)
		if i < 0 {
			t.Fatalf("%s not in the table", name)
		}
		return i
	}
	for _, r := range zmRefused {
		i := entry(r.name)
		want := "Defined$ set holding an object on kop " + r.op
		if pd.Status[i] != cardset.StatusNotLowered || !strings.Contains(pd.Reason[i], want) {
			t.Errorf("%s: status %v reason %q, want refused with %q", r.name, pd.Status[i], pd.Reason[i], want)
		}
	}
	for _, b := range zmBuilt {
		i := entry(b.name)
		if pd.Status[i] == cardset.StatusNotLowered {
			t.Fatalf("%s refused: %s", b.name, pd.Reason[i])
		}
		reads := slices.ContainsFunc(pd.Defs[i].Abilities, func(a kop.Ability) bool { return len(a.SetRefs) > 0 })
		if reads != b.set {
			t.Fatalf("%s: reads a set %v, want %v", b.name, reads, b.set)
		}
	}
	var gd [2][]*cards.Card
	for s, d := range zmDecks {
		for _, e := range d {
			c := byName[e.Name]
			if c == nil {
				t.Fatalf("%s is neither in the pool nor a carrier", e.Name)
			}
			for i := 0; i < e.Count; i++ {
				gd[s] = append(gd[s], c)
			}
		}
		if len(gd[s]) != 40 {
			t.Fatalf("deck %d: %d cards", s, len(gd[s]))
		}
	}
	st := &dirac.SetStats{}
	dirac.DebugSetStats = st
	defer func() { dirac.DebugSetStats = nil }()
	clean, _, divs := lockstepSeedsMode(t, reg, cardset.Raw(cardset.NewDataSet(pd)), "definedset-zonemove", zmDecks, gd, 1, 6, lockstepFailEach, nil, nil, castFirstPolicy)
	if len(divs) > 0 {
		t.Errorf("%d divergent games", len(divs))
	}
	if st.Runs == 0 || st.Members == 0 {
		t.Errorf("no game ran a RefSet instruction over a member: %+v", *st)
	}
	t.Logf("%d clean games; %+v", clean, *st)
}
