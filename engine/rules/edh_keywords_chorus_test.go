package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/effects"
	"github.com/adams-shaun/gorge/engine/state"
)

// Chorus of the Conclave's pay-any-amount additional cost (CR 601.2b) played
// through a real game on compiled corpus cards: Chorus enters via a logged
// zone move, Grizzly Bears ({1}{G}) is cast from hand paying from the mana
// pool. No Forge script text is committed.

func chorusEngine(t *testing.T, withChorus bool) (*Engine, Config) {
	t.Helper()
	reg := searchTestRegistry(t)
	chorus := searchCorpusCard(t, reg, "Chorus of the Conclave")
	bear := searchCorpusCard(t, reg, "Grizzly Bears")
	bolt := searchCorpusCard(t, reg, "Lightning Bolt")
	mountain := searchCorpusCard(t, reg, "Mountain")
	halberdier := searchCorpusCard(t, reg, "Keldon Halberdier")
	deck := []*cards.Card{chorus, bolt, halberdier}
	for len(deck) < 40 {
		deck = append(deck, bear)
	}
	opp := make([]*cards.Card, 40)
	for i := range opp {
		opp[i] = mountain
	}
	cfg := seatZeroStart(Config{Seed: 7702, Names: []string{"chorus", "opp"},
		Decks: [][]*cards.Card{deck, opp}, Tokens: reg.Tokens})
	e := New(cfg)
	e.Advance()
	toMain1(t, e)
	if withChorus {
		searchMoveByName(t, e, "Chorus of the Conclave", state.ZBattlefield)
	}
	return e, cfg
}

func chorusAmounts(t *testing.T, e *Engine) []int {
	t.Helper()
	d := e.Pending()
	if d == nil || d.Kind != decision.KChoose {
		t.Fatalf("expected the KChoose chorus amount decision, got %+v", d)
	}
	var out []int
	for _, o := range d.Options {
		if o.Kind != chorusAnswerKind {
			t.Fatalf("option %+v is not a chorus amount", o)
		}
		out = append(out, o.Amount)
	}
	return out
}

func chorusChoose(t *testing.T, e *Engine, want int) {
	t.Helper()
	for _, o := range e.Pending().Options {
		if o.Kind == chorusAnswerKind && o.Amount == want {
			submitChoices(t, e, o.Index)
			return
		}
	}
	t.Fatalf("no chorus option for %d: %+v", want, e.Pending().Options)
}

func chorusPool(e *Engine) int32 {
	var n int32
	for _, v := range e.G.Players[0].Pool {
		n += v
	}
	return n
}

// TestEDHKeywordsChorusOffersZeroToPoolAndPaidAmountBecomesCounters: with
// five mana floating and a two-mana creature, the offer is 0..3, paying 2
// charges exactly 2 extra and the Bears enter with exactly 2 counters.
func TestEDHKeywordsChorusOffersZeroToPoolAndPaidAmountBecomesCounters(t *testing.T) {
	t.Parallel()
	e, cfg := chorusEngine(t, true)
	bear := searchMoveByName(t, e, "Grizzly Bears", state.ZHand)
	addMana(t, e, 0, "GGGGG")

	submitChoices(t, e, replicateOption(t, e, bear, "").Index)
	got := chorusAmounts(t, e)
	if len(got) != 4 || got[0] != 0 || got[3] != 3 {
		t.Fatalf("chorus amounts %v, want 0..3 (five mana, {1}{G} base)", got)
	}
	chorusChoose(t, e, 2)
	passUntilStackEmpty(t, e, 40)

	o := e.G.Obj(bear)
	if o.Zone != state.ZBattlefield {
		t.Fatalf("Bears in %s, want the battlefield", o.Zone)
	}
	if n := o.Counter("P1P1"); n != 2 {
		t.Fatalf("Bears entered with %d +1/+1 counters, want 2", n)
	}
	if left := chorusPool(e); left != 1 {
		t.Fatalf("pool holds %d after paying {1}{G}+2 of 5, want 1", left)
	}
	replayCheck(t, e, cfg)
}

// TestEDHKeywordsChorusPayingZeroAddsNothing: answering 0 is the plain cast.
func TestEDHKeywordsChorusPayingZeroAddsNothing(t *testing.T) {
	t.Parallel()
	e, cfg := chorusEngine(t, true)
	bear := searchMoveByName(t, e, "Grizzly Bears", state.ZHand)
	addMana(t, e, 0, "GGGGG")

	submitChoices(t, e, replicateOption(t, e, bear, "").Index)
	chorusAmounts(t, e)
	chorusChoose(t, e, 0)
	passUntilStackEmpty(t, e, 40)

	o := e.G.Obj(bear)
	if o.Zone != state.ZBattlefield {
		t.Fatalf("Bears in %s, want the battlefield", o.Zone)
	}
	if n := o.Counter("P1P1"); n != 0 {
		t.Fatalf("Bears entered with %d counters after paying 0, want 0", n)
	}
	if left := chorusPool(e); left != 3 {
		t.Fatalf("pool holds %d after a plain {1}{G} from 5, want 3", left)
	}
	replayCheck(t, e, cfg)
}

// TestEDHKeywordsChorusOfferTracksTheLivePool: the ceiling is whatever the
// pool can still pay -- three mana for a two-mana creature offers 0..1, and
// paying the maximum drains the pool.
func TestEDHKeywordsChorusOfferTracksTheLivePool(t *testing.T) {
	t.Parallel()
	e, _ := chorusEngine(t, true)
	bear := searchMoveByName(t, e, "Grizzly Bears", state.ZHand)
	addMana(t, e, 0, "GGG")

	submitChoices(t, e, replicateOption(t, e, bear, "").Index)
	got := chorusAmounts(t, e)
	if len(got) != 2 || got[1] != 1 {
		t.Fatalf("chorus amounts %v, want 0..1 for three mana", got)
	}
	chorusChoose(t, e, 1)
	passUntilStackEmpty(t, e, 40)
	if n := e.G.Obj(bear).Counter("P1P1"); n != 1 {
		t.Fatalf("Bears entered with %d counters, want 1", n)
	}
	if left := chorusPool(e); left != 0 {
		t.Fatalf("pool holds %d after paying the maximum, want 0", left)
	}
}

// TestEDHKeywordsChorusNoAskWhenNothingMoreIsPayable: exactly the base cost
// floating leaves nothing to offer, so the cast goes straight through.
func TestEDHKeywordsChorusNoAskWhenNothingMoreIsPayable(t *testing.T) {
	t.Parallel()
	e, _ := chorusEngine(t, true)
	bear := searchMoveByName(t, e, "Grizzly Bears", state.ZHand)
	addMana(t, e, 0, "GG")

	submitChoices(t, e, replicateOption(t, e, bear, "").Index)
	if d := e.Pending(); d != nil && len(d.Options) > 0 && d.Options[0].Kind == chorusAnswerKind {
		t.Fatalf("posed a chorus decision with nothing extra payable: %+v", d)
	}
	passUntilStackEmpty(t, e, 40)
	o := e.G.Obj(bear)
	if o.Zone != state.ZBattlefield || o.Counter("P1P1") != 0 {
		t.Fatalf("Bears %s with %d counters, want a plain battlefield entry", o.Zone, o.Counter("P1P1"))
	}
}

// TestEDHKeywordsChorusNeedsTheStaticAndACreatureSpell: without Chorus on the
// battlefield, and for a noncreature spell with it, no amount is asked.
func TestEDHKeywordsChorusNeedsTheStaticAndACreatureSpell(t *testing.T) {
	t.Parallel()
	e, _ := chorusEngine(t, false)
	bear := searchMoveByName(t, e, "Grizzly Bears", state.ZHand)
	addMana(t, e, 0, "GGGGG")
	submitChoices(t, e, replicateOption(t, e, bear, "").Index)
	if d := e.Pending(); d != nil && len(d.Options) > 0 && d.Options[0].Kind == chorusAnswerKind {
		t.Fatalf("no Chorus in play yet a chorus decision was posed: %+v", d)
	}

	e2, _ := chorusEngine(t, true)
	bolt := searchMoveByName(t, e2, "Lightning Bolt", state.ZHand)
	addMana(t, e2, 0, "RRRRR")
	d := e2.Pending()
	for _, o := range d.Options {
		if o.Kind == "cast" && o.Obj == bolt && o.Mode == "" {
			submitChoices(t, e2, o.Index)
			break
		}
	}
	if d := e2.Pending(); d != nil && len(d.Options) > 0 && d.Options[0].Kind == chorusAnswerKind {
		t.Fatalf("a noncreature spell was offered the chorus amount: %+v", d)
	}
}

// TestEDHKeywordsChorusStaticIsRegistered pins the registration against the
// behaviour tests above: the bare K: sentence is a supported primitive only
// because chorusAsk and the entry grant implement it.
func TestEDHKeywordsChorusStaticIsRegistered(t *testing.T) {
	t.Parallel()
	if !effects.Supported()["kw:"+chorusKeywordLine] {
		t.Fatal("Chorus's pay-any-amount static is not registered (proof: TestEDHKeywordsChorusOffersZeroToPoolAndPaidAmountBecomesCounters)")
	}
	reg := searchTestRegistry(t)
	c := searchCorpusCard(t, reg, "Chorus of the Conclave")
	if m := reg.Unsupported(c, effects.Supported()); len(m) != 0 {
		t.Fatalf("Chorus of the Conclave reports unsupported: %v", m)
	}
}

// TestEDHKeywordsChorusNotAskedForSuspend: suspending a creature card is a
// special action, not casting a creature spell (CR 116.2f, 702.62a), so
// Chorus's additional cost never applies -- no amount is asked, nothing extra
// is charged and the exiled card carries no counter grant.
func TestEDHKeywordsChorusNotAskedForSuspend(t *testing.T) {
	t.Parallel()
	e, _ := chorusEngine(t, true)
	id := searchMoveByName(t, e, "Keldon Halberdier", state.ZHand)
	addMana(t, e, 0, "RR")

	submitChoices(t, e, replicateOption(t, e, id, "suspend").Index)
	if d := e.Pending(); d != nil {
		for _, o := range d.Options {
			if o.Kind == chorusAnswerKind {
				t.Fatalf("suspend posed the chorus amount ask: %+v", d)
			}
		}
	}
	o := e.G.Obj(id)
	if o.Zone != state.ZExile || o.CastFlags&state.FlagSuspend == 0 {
		t.Fatalf("Halberdier in %s flags %v, want suspended in exile", o.Zone, o.CastFlags)
	}
	if left := chorusPool(e); left != 1 {
		t.Fatalf("pool holds %d after suspending for {R} from 2, want 1", left)
	}
}
