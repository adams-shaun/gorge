package mapping_test

// mixedSelect must carry the
// Section 7.1 hidden order, hidden_pick's Destination must choose the select
// purpose, the generalized newDig's presentational remainder op must set
// Inside, and a yes/no ask about a hidden card must open the look.

import (
	"cmp"
	"slices"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/rules"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/server/spellbench/catalog"
	"github.com/adams-shaun/gorge/server/spellbench/gamecfg"
	"github.com/adams-shaun/gorge/server/spellbench/internal/testcorpus"
	"github.com/adams-shaun/gorge/server/spellbench/internal/testgame"
	"github.com/adams-shaun/gorge/server/spellbench/mapping"
	"github.com/adams-shaun/gorge/server/spellbench/observe"
	"github.com/adams-shaun/gorge/server/spellbench/protocol"
	"github.com/adams-shaun/gorge/server/spellbench/secrets"
	"github.com/adams-shaun/gorge/server/spellbench/wire"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

func repoGame(t *testing.T, reg *cards.Registry, deck []*cards.Card, secret byte) *gamecfg.Game {
	t.Helper()
	s := make([]byte, 32)
	s[0] = secret
	g, err := gamecfg.New(reg, secrets.NewGame(s), [2][]*cards.Card{deck, deck},
		gamecfg.Rules{Mulligan: "none", StartingSeat: state.PlayerID(0)})
	if err != nil {
		t.Fatal(err)
	}
	return g
}

// untilRepoDeck plays a legacy repo deck with bots until pred holds.
func untilRepoDeck(t *testing.T, deck string, pred func(*decision.Decision, *rules.Engine) bool) *gamecfg.Game {
	t.Helper()
	reg := testcorpus.Registry(t)
	cs := testutil.RepoDeck(t, reg, deck)
	for s := byte(1); s < 25; s++ {
		g := repoGame(t, reg, cs, s)
		if testgame.RunUntil(t, g, testgame.Bots(uint64(s)), func(e *rules.Engine) bool {
			d := e.Pending()
			return d != nil && pred(d, e)
		}, 30000) {
			return g
		}
	}
	t.Fatalf("no %s game reached the wanted decision", deck)
	return nil
}

func cardName(g *gamecfg.Game, id state.ObjID) string { return g.E.G.Obj(id).Face().Name }

// TestMixedSelectOrdersHiddenCandidates takes a real Thoughtseize
// modes/discard decision, reverses its options, and checks the pose is still
// in (card_name, object_id) order, each hidden candidate carrying the key V5
// reads. Before the fix the candidates had Hidden=false and empty keys, so
// Finalize left them in engine order.
func TestMixedSelectOrdersHiddenCandidates(t *testing.T) {
	g := untilRepoDeck(t, "dimir-tempo", func(d *decision.Decision, _ *rules.Engine) bool {
		return d.Kind == decision.KModes && mapping.Route(d) == "modes/discard"
	})
	env := envFor(t, g)
	orig := *g.E.Pending()
	d := orig
	d.Options = slices.Clone(orig.Options)
	names := map[state.ObjID]string{}
	distinct := map[string]bool{}
	for _, o := range d.Options {
		names[o.Obj] = cardName(g, o.Obj)
		distinct[names[o.Obj]] = true
	}
	if len(distinct) < 2 {
		t.Fatalf("precondition: opponent hand %v has fewer than two names", names)
	}
	slices.SortFunc(d.Options, func(a, b decision.Option) int {
		if c := cmp.Compare(names[a.Obj], names[b.Obj]); c != 0 {
			return -c // reverse name order, the leak Finalize must hide
		}
		return -cmp.Compare(a.Obj, b.Obj)
	})
	tx, err := mapping.Begin(env, &d)
	if err != nil {
		t.Fatal(err)
	}
	p, err := tx.Pose()
	if err != nil {
		t.Fatal(err)
	}
	if !p.Look {
		t.Fatal("pose does not open the look over a hidden hand")
	}
	var prev string
	for i, c := range p.Candidates {
		if c.Sem.Kind != "select_object" {
			t.Fatalf("candidate %d is %s, want select_object", i, c.Sem.Kind)
		}
		if !c.Hidden {
			t.Fatalf("candidate %d does not carry the hidden flag", i)
		}
		sel, ok := c.Sem.Fields["choice"].(protocol.TargetRef)
		if !ok || sel.Object == nil {
			t.Fatalf("candidate %d has no object choice", i)
		}
		r := sel.Object
		if r.CardName == nil || c.SortName != *r.CardName || c.SortID != r.ObjectID {
			t.Fatalf("candidate %d key (%q,%q), want (%q,%q)", i, c.SortName, c.SortID, cardNameOf(r), r.ObjectID)
		}
		key := c.SortName + "\x00" + c.SortID
		if key < prev {
			t.Fatalf("candidate %d breaks (card_name, object_id) order", i)
		}
		prev = key
	}
}

func cardNameOf(r *protocol.ObjectRef) string {
	if r.CardName == nil {
		return ""
	}
	return *r.CardName
}

// TestHiddenPickDestinationSelectsPurpose pins the Destination -> purpose
// mapping the generalized hidden_pick route uses. The real decision supplies
// the pending answer the transaction probes; each copy only changes the
// resume param the purpose is read from.
func TestHiddenPickDestinationSelectsPurpose(t *testing.T) {
	g := untilRepoDeck(t, "tron", func(d *decision.Decision, _ *rules.Engine) bool {
		return d.Kind == decision.KChoose && len(d.Options) > 0 && d.Options[0].Kind == "hidden_pick" &&
			d.ResumeSA != nil && d.ResumeSA.Params["Destination"] != ""
	})
	env := envFor(t, g)
	orig := *g.E.Pending()
	cases := []struct{ dest, want string }{
		{"", "other"},
		{"Hand", "put_into_hand"},
		{"Battlefield", "put_onto_battlefield"},
		{"Graveyard", "put_into_graveyard"},
		{"Exile", "exile"},
		{"Library", "search"},
	}
	for _, c := range cases {
		d := orig
		sa := *orig.ResumeSA
		sa.Params = map[string]string{}
		for k, v := range orig.ResumeSA.Params {
			sa.Params[k] = v
		}
		sa.Params["Destination"] = c.dest
		d.ResumeSA = &sa
		tx, err := mapping.Begin(env, &d)
		if err != nil {
			t.Fatalf("%q: %v", c.dest, err)
		}
		p, err := tx.Pose()
		if err != nil {
			t.Fatalf("%q: %v", c.dest, err)
		}
		if p.Context.Purpose == nil || *p.Context.Purpose != c.want {
			t.Fatalf("Destination %q purpose %v, want %q", c.dest, p.Context.Purpose, c.want)
		}
	}
}

// TestDigNonHandPrimaryAndNonBottomRemainder drives the generalized newDig
// shape (battlefield primary, hand remainder) from a real Matter Reshaper and
// checks the partition names both destinations and every presentational
// remainder ordering op states Inside=false.
func TestDigNonHandPrimaryAndNonBottomRemainder(t *testing.T) {
	g := untilRepoDeck(t, "eldrazi-stompy", func(d *decision.Decision, _ *rules.Engine) bool {
		return d.Kind == decision.KChoose && len(d.Options) > 0 && d.Options[0].Kind == "dig" &&
			d.ResumeSA != nil && strings.EqualFold(strings.TrimSpace(d.ResumeSA.Params["DestinationZone"]), "Battlefield")
	})
	env := envFor(t, g)
	tx, err := mapping.Begin(env, g.E.Pending())
	if err != nil {
		t.Fatal(err)
	}
	sawPrimary, sawHand := false, false
	orderOps := 0
	answerAll(t, tx, func(p *mapping.Pose) int {
		for i, c := range p.Candidates {
			if c.Sem.Kind == "arrange_card" {
				dst, _ := c.Sem.Fields["destination"].(string)
				sawPrimary = sawPrimary || dst == "battlefield"
				sawHand = sawHand || dst == "hand"
				if dst == "hand" {
					return i // leave the card to the hand remainder
				}
				continue
			}
			if c.Op.Op == "dest" {
				if c.Op.List != "hand" || c.Op.Inside == nil || *c.Op.Inside {
					t.Fatalf("remainder order op %+v, want Inside=false on hand", c.Op)
				}
				orderOps++
			}
		}
		return 0
	})
	if !sawPrimary || !sawHand {
		t.Fatalf("partition destinations battlefield=%v hand=%v", sawPrimary, sawHand)
	}
	_ = orderOps
}

// TestChooseYesnoOpensLookForAHiddenNamedCard pins openLookIfHidden: a yes/no
// option naming a library card must start the seat's look so the view rekeys
// it. Delver of Secrets reveals its top card before the transform ask.
func TestChooseYesnoOpensLookForAHiddenNamedCard(t *testing.T) {
	g := untilRepoDeck(t, "ur-delver", func(d *decision.Decision, e *rules.Engine) bool {
		if d.Kind != decision.KChoose || mapping.Route(d) != "choose/yesno" {
			return false
		}
		for _, o := range d.Options {
			if obj := e.G.Obj(o.Obj); o.Obj != 0 && obj != nil && !observe.Visible(d.Player, obj) {
				return true
			}
		}
		return false
	})
	env := envFor(t, g)
	tx, err := mapping.Begin(env, g.E.Pending())
	if err != nil {
		t.Fatal(err)
	}
	p, err := tx.Pose()
	if err != nil {
		t.Fatal(err)
	}
	d := g.E.Pending()
	if !env.Looking[d.Player] {
		t.Fatal("the look over the hidden card was not opened")
	}
	if len(p.Known) != 0 {
		t.Fatalf("known %d, want the yes/no pose to carry no known entries", len(p.Known))
	}
}

// TestDigRemainderOrderOpSetsInside uses Make Your Own Luck's dig (exile
// primary, three-card hand remainder) to reach the presentational remainder
// ordering op. The hand is outside the native Choices here, so the op must
// say Inside=false rather than leave the legacy top/hand convention to guess
// wrong and only answer by the single-candidate "forced" path.
func TestDigRemainderOrderOpSetsInside(t *testing.T) {
	reg := testcorpus.Registry(t)
	deck := catalog.Deck{CatalogID: "review", Rows: []wire.DeckRow{
		{Name: "Make Your Own Luck", Count: 8},
		{Name: "Island", Count: 26}, {Name: "Forest", Count: 26}}}
	cs, err := catalog.Resolve(reg, deck)
	if err != nil {
		t.Fatal(err)
	}
	var g *gamecfg.Game
	for s := byte(1); s < 10 && g == nil; s++ {
		try := repoGame(t, reg, cs, s)
		if testgame.RunUntil(t, try, testgame.Bots(uint64(s)), func(e *rules.Engine) bool {
			d := e.Pending()
			return d != nil && d.Kind == decision.KChoose && len(d.Options) > 0 && d.Options[0].Kind == "dig" &&
				d.ResumeSA != nil && strings.EqualFold(strings.TrimSpace(d.ResumeSA.Params["DestinationZone"]), "Exile")
		}, 30000) {
			g = try
		}
	}
	if g == nil {
		t.Fatal("no game reached the exile/hand dig")
	}
	env := envFor(t, g)
	tx, err := mapping.Begin(env, g.E.Pending())
	if err != nil {
		t.Fatal(err)
	}
	orderOps := 0
	commit := answerAll(t, tx, func(p *mapping.Pose) int {
		for i, c := range p.Candidates {
			if c.Sem.Kind == "arrange_card" {
				if c.Sem.Fields["destination"] == "hand" {
					return i // leave the rest to the hand remainder
				}
				continue
			}
			if c.Op.Op == "dest" {
				if c.Op.List != "hand" || c.Op.Inside == nil || *c.Op.Inside {
					t.Fatalf("remainder order op %+v, want Inside=false on hand", c.Op)
				}
				orderOps++
			}
		}
		return 0
	})
	if orderOps == 0 {
		t.Fatal("no presentational remainder order op was posed")
	}
	if _, err := g.Probe(commit...); err != nil {
		t.Fatalf("dig commit rejected: %v", err)
	}
}
