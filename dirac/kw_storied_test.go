package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
	v "github.com/adams-shaun/gorge/dirac/vanilla"
)

// storyTable is vanilla's table with the scan's three printed shapes: a
// legendary creature carrying K:Storied (the corpus carriers are legendary
// Dwarfs), a non-Storied legendary artifact and a non-Storied Saga.
func storyTable() *kop.Table {
	tbl := v.NewTable()
	tbl.Cards = append(tbl.Cards,
		kop.CardDef{Name: "Test Storied Bard", Types: kop.TCreature | kop.TLegendary, Power: 2, Toughness: 2, Storied: true},
		kop.CardDef{Name: "Test Storied Relic", Types: kop.TArtifact | kop.TLegendary, Storied: true},
		kop.CardDef{Name: "Test Story Sage", Types: kop.TCreature | kop.TLegendary, Power: 1, Toughness: 1},
		kop.CardDef{Name: "Test Story Saga", Types: kop.TEnchantment, SagaN: 1},
	)
	return tbl
}

// TestStoriedScanGrantsTheLatch: CR 702.195's grant — a seat that controls
// three or more qualifying permanents (artifact, legendary and/or Saga,
// counted once each) among which one carries Storied and keeps its abilities
// holds the enduring story for the rest of the game, and the entry hooks
// (enter's storyGrant) run the scan, so the qualifying third permanent's own arrival
// grants it. Two qualifying permanents grant nothing; neither does a Storied
// carrier whose abilities were lost, nor a board with the count but no
// Storied carrier; the mutation switch denies every grant.
func TestStoriedScanGrantsTheLatch(t *testing.T) {
	sc := newScenTable(t, storyTable(), Config{NoElide: true})
	s := sc.s
	bard, relic := CardID(len(v.Table.Cards)), CardID(len(v.Table.Cards)+1)
	sage, saga := CardID(len(v.Table.Cards)+2), CardID(len(v.Table.Cards)+3)
	if !sc.k.storied {
		t.Fatal("prep.storied is false over a table with a printed Storied card")
	}

	// two qualifying permanents, no Storied carrier yet: the SBA pass and a
	// direct scan grant nothing.
	sc.perm(sage, 0)
	sc.perm(saga, 0)
	s.storiedScan(sc.k)
	if s.hasStory(0) {
		t.Fatal("two qualifying permanents granted the latch")
	}

	// the Storied carrier is the third qualifying permanent: entering grants
	// the latch through the entry hook.
	sc.perm(bard, 0)
	if !s.hasStory(0) {
		t.Fatal("entering the third qualifying Storied permanent did not grant the latch")
	}

	// the latch is one-way for the rest of the game, and it does not reach
	// the other seat.
	if s.hasStory(1) {
		t.Fatal("seat 1 holds the latch with no Storied card of its own")
	}

	// a Storied carrier that lost its abilities (genLost) does not qualify:
	// seat 1 controls one Storied relic and two qualifying permanents, and
	// relic's abilities are removed, so the scan grants nothing. The removal
	// is armed after the relic enters (its own entry scanned a board of one).
	s.g.story = 0
	relicObj := sc.perm(relic, 1)
	if s.hasStory(1) {
		t.Fatal("seat 1 was latched before the lost-abilities read")
	}
	s.perms[sc.slot(relicObj)].ovr.flags |= ovLoseAll // a table with no lose-all static: the flag is what genLost reads
	sc.perm(sage, 1)
	sc.perm(saga, 1)
	s.storiedScan(sc.k)
	if s.hasStory(1) {
		t.Fatal("a lost-abilities Storied carrier granted the latch")
	}
}

// TestStoriedScanCountsLostAbilityTypes: losing abilities removes keywords,
// not types (CR 613.1f vs 613.1d), so a lose-all legendary still counts as a
// qualifying permanent; gorge skips only the Storied keyword read
// (hasKeywordH). Seat 0's third qualifier is the Storied Bard, with the Sage
// (a non-Storied legendary under a lose-all) and the Saga already out: the
// count is 3 only if the Sage counts.
func TestStoriedScanCountsLostAbilityTypes(t *testing.T) {
	sc := newScenTable(t, storyTable(), Config{NoElide: true})
	s := sc.s
	bard := CardID(len(v.Table.Cards))
	sage, saga := CardID(len(v.Table.Cards)+2), CardID(len(v.Table.Cards)+3)
	sageObj := sc.perm(sage, 0)
	s.perms[sc.slot(sageObj)].ovr.flags |= ovLoseAll
	if !s.genLost(sc.k, sc.slot(sageObj)) {
		t.Fatal("the lose-all flag is not what genLost reads")
	}
	sc.perm(saga, 0)
	if s.hasStory(0) {
		t.Fatal("two qualifying permanents granted the latch")
	}
	sc.perm(bard, 0)
	if !s.hasStory(0) {
		t.Fatal("a lost-abilities legendary was not counted as a qualifying permanent")
	}
}

// TestStoriedScanMutation: DebugStoriedNone denies the grant (the oracle's
// TestStaticZonesMutation must diverge on it).
func TestStoriedScanMutation(t *testing.T) {
	DebugStoriedNone = true
	defer func() { DebugStoriedNone = false }()
	sc := newScenTable(t, storyTable(), Config{NoElide: true})
	s := sc.s
	bard, sage, saga := CardID(len(v.Table.Cards)), CardID(len(v.Table.Cards)+2), CardID(len(v.Table.Cards)+3)
	sc.perm(sage, 0)
	sc.perm(saga, 0)
	sc.perm(bard, 0)
	if s.hasStory(0) {
		t.Fatal("DebugStoriedNone did not deny the grant")
	}
}
