package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
)

// "As CARDNAME enters, pay any amount of life" in lockstep (dirac/
// repl_paylife.go): Phyrexian Processor's token, Minion of the Wastes' and
// Nameless Race's CDA power and toughness all read the life paid as the card
// entered, and Nameless Race's XMax$ caps the ask at the white permanents and
// white cards the opponent holds. The three probes are the printed scripts
// under synthetic names (built in process through kgen.TableFromCards). The
// pay policy answers each ask with the least, the most or a middle amount by
// seed, so the zero, full and partial payments are all played. The opponent's
// Naturalizes destroy the probe, and a destroyed probe reads its printed 0
// again (CR 400.7; the token of an ability that outlives its Processor is 0/0).
//
// Two seats only: dirac is a two-seat kernel and kgen refuses a pod card.

const payLifeProcessorSrc = `Name:Pay Life Processor
ManaCost:4
Types:Artifact
R:Event$ Moved | Destination$ Battlefield | ValidCard$ Card.Self | ReplaceWith$ PayAny | ReplacementResult$ Updated | Description$ Pay any amount of life as this enters.
SVar:PayAny:AB$ StoreSVar | SVar$ LifePaidOnETB | Type$ Calculate | Expression$ X | Cost$ Mandatory PayLife<X>
A:AB$ Token | Cost$ 4 T | TokenScript$ b_x_x_phyrexian_minion | TokenPower$ LifePaidOnETB | TokenToughness$ LifePaidOnETB | TokenOwner$ You | SpellDescription$ Make a token with the life paid.
SVar:X:Count$xPaid
SVar:LifePaidOnETB:Number$0
`

const payLifeMinionSrc = `Name:Pay Life Minion
ManaCost:3 B B B
Types:Creature Minion
PT:*/*
K:Trample
R:Event$ Moved | Destination$ Battlefield | ValidCard$ Card.Self | ReplaceWith$ PayAny | ReplacementResult$ Updated | Description$ Pay any amount of life as this enters.
SVar:PayAny:AB$ StoreSVar | SVar$ LifePaidOnETB | Type$ Calculate | Expression$ X | Cost$ Mandatory PayLife<X>
S:Mode$ Continuous | SetPower$ LifePaidOnETB | SetToughness$ LifePaidOnETB | CharacteristicDefining$ True | Description$ Power and toughness equal the life paid.
SVar:X:Count$xPaid
SVar:LifePaidOnETB:Number$0
SVar:NoZeroToughnessAI:True
`

const payLifeRaceSrc = `Name:Pay Life Race
ManaCost:3 B
Types:Creature
PT:*/*
K:Trample
R:Event$ Moved | Destination$ Battlefield | ValidCard$ Card.Self | ReplaceWith$ PayAny | ReplacementResult$ Updated | Description$ Pay any amount of life as this enters. The amount you pay can't be more than the white permanents plus the white cards.
SVar:PayAny:AB$ StoreSVar | SVar$ LifePaidOnETB | Type$ Calculate | Expression$ X | Cost$ Mandatory PayLife<X> | XMax$ Cap
S:Mode$ Continuous | SetPower$ LifePaidOnETB | SetToughness$ LifePaidOnETB | CharacteristicDefining$ True | Description$ Power and toughness equal the life paid.
SVar:X:Count$xPaid
SVar:Cap:SVar$Held/Plus.Yard
SVar:Held:Count$Valid Permanent.White+!token+OppCtrl
SVar:Yard:Count$ValidGraveyard Card.White+OppCtrl
SVar:LifePaidOnETB:Number$0
`

// payLifeCase is one probe and the two decks it plays against.
type payLifeCase struct {
	name, src string
	decks     [2][]cardset.Entry
}

func payLifeCases() []payLifeCase {
	opp := []cardset.Entry{{Name: "Naturalize", Count: 6}, {Name: "Savannah Lions", Count: 8}, {Name: "Plains", Count: 13}, {Name: "Forest", Count: 13}}
	return []payLifeCase{
		{"Pay Life Processor", payLifeProcessorSrc, [2][]cardset.Entry{
			{{Name: "Pay Life Processor", Count: 12}, {Name: "Island", Count: 14}, {Name: "Plains", Count: 14}},
			{{Name: "Naturalize", Count: 6}, {Name: "Pay Life Processor", Count: 4}, {Name: "Forest", Count: 15}, {Name: "Island", Count: 15}}}},
		{"Pay Life Minion", payLifeMinionSrc, [2][]cardset.Entry{
			{{Name: "Pay Life Minion", Count: 12}, {Name: "Swamp", Count: 28}}, opp}},
		{"Pay Life Race", payLifeRaceSrc, [2][]cardset.Entry{
			{{Name: "Pay Life Race", Count: 12}, {Name: "Swamp", Count: 28}}, opp}},
	}
}

// payLifeStats counts what the ask posed and the pay policy answered.
type payLifeStats struct {
	asks, zero, partial, full int
}

// payLifePolicy plays castActivatePlayedPolicy and answers a pay-life ask with
// the least, the most or the middle option by seed.
func payLifePolicy(seed uint64, played map[string]int, st *payLifeStats) Policy {
	base := castActivatePlayedPolicy(seed, played)
	code, _ := chooseCode("paylife")
	kind := ActChoose + CanonKind(code)
	return PolicyFunc(func(c *PickCtx) Canon {
		n := len(c.D.Options)
		if n > 0 && c.D.Options[0].Kind() == kind {
			var a Canon
			switch seed % 3 {
			case 0:
				a = c.D.Options[0]
			case 1:
				a = c.D.Options[n-1]
			default:
				a = c.D.Options[n/2]
			}
			st.asks++
			switch {
			case a.Mode() == 0:
				st.zero++
			case int(a.Mode()) == n-1:
				st.full++
			default:
				st.partial++
			}
			return a
		}
		return base.Pick(c)
	})
}

// payLifeRun plays seeds 1..seeds of the case; mutate runs before the games.
// It returns how many seeds diverged (report: each is an error, else the
// first stops the run).
func payLifeRun(t *testing.T, reg *cards.Registry, tc payLifeCase, seeds uint64, report bool, st *payLifeStats, mutate func()) int {
	t.Helper()
	set, gd := sweepcapBuild(t, reg, tc.src, tc.decks)
	tbl := sweepcapTable(t, set, tc.decks)
	if len(tbl.SVarNames) != 1 || tbl.SVarNames[0] != "LifePaidOnETB" {
		t.Fatalf("table SVarNames = %v, want [LifePaidOnETB]", tbl.SVarNames)
	}
	if mutate != nil {
		mutate()
	}
	div := 0
	played := map[string]int{}
	for seed := uint64(1); seed <= seeds; seed++ {
		m, err := NewMatchup(reg, [2]string{"paylife-a", "paylife-b"}, gd, 1)
		if err != nil {
			t.Fatal(err)
		}
		cfg := Config{GorgeAutopaySurface: seed%2 == 0}
		k, err := SetSide(set, tbl, cfg)(m, seed)
		if err != nil {
			t.Fatal(err)
		}
		res := Lockstep(m, seed, k, payLifePolicy(seed, played, st), RunConfig{Config: cfg, MaxDecisions: 4000, MaxTurns: 40})
		CloseSide(k)
		if d := res.Div; d != nil {
			div++
			if !report {
				t.Logf("seed %d diverges (decision %d: %s)", seed, d.At, d.What)
				return div
			}
			t.Errorf("seed %d: diverged at decision %d: %s %s %v %s", seed, d.At, d.What, d.Field, d.Diff, d.Detail)
		}
	}
	if played[tc.name] == 0 {
		t.Fatalf("%s was never played", tc.name)
	}
	return div
}

// TestPayLifeLockstep: each probe plays identically in gorge and dirac over
// 60 seeds, and the pool reached the zero, partial and full payments and the
// number store's write, read and clear.
func TestPayLifeLockstep(t *testing.T) {
	reg := synthReg(t)
	var ns dirac.NumStoreStats
	dirac.DebugNumStoreStats = &ns
	defer func() { dirac.DebugNumStoreStats = nil }()
	for _, tc := range payLifeCases() {
		var st payLifeStats
		ns = dirac.NumStoreStats{}
		if div := payLifeRun(t, reg, tc, 60, true, &st, nil); div != 0 {
			t.Fatalf("%s: %d seeds diverged", tc.name, div)
		}
		if st.asks == 0 || st.zero == 0 || st.full == 0 {
			t.Errorf("%s: the pool never reached the ask's arms: %+v", tc.name, st)
		}
		if tc.name != "Pay Life Race" && st.partial == 0 {
			t.Errorf("%s: no partial payment: %+v", tc.name, st)
		}
		if ns.Writes == 0 || ns.Hits == 0 {
			t.Errorf("%s: no stored number was written and read: %+v", tc.name, ns)
		}
		t.Logf("%s: asks %+v, store %+v", tc.name, st, ns)
	}
}

// TestPayLifeMutation: with the answer paying its life but storing nothing some
// seed diverges, so the lockstep does observe the stored number. Test-only
// global: never parallel.
func TestPayLifeMutation(t *testing.T) {
	reg := synthReg(t)
	defer func() { dirac.DebugPayLifeNoStore = false }()
	tc := payLifeCases()[0]
	var st payLifeStats
	if div := payLifeRun(t, reg, tc, 60, false, &st, func() { dirac.DebugPayLifeNoStore = true }); div == 0 {
		t.Fatal("no seed diverged with the store dropped: the pool does not read the paid life")
	}
}

// payLifeOrggSrc is a rewritten Soulgorger Orgg: the stored number is written by
// a trigger (lose all but one life), not the pay-life ask, and read by the
// departure trigger (the number store clears on the departure, so gorge's
// refund reads the printed 0).
const payLifeOrggSrc = `Name:Pay Life Orgg
ManaCost:3 R R
Types:Creature Nightmare Orgg
PT:6/6
T:Mode$ ChangesZone | Origin$ Any | Destination$ Battlefield | ValidCard$ Card.Self | Execute$ TrigDrain | TriggerDescription$ drain
SVar:TrigDrain:DB$ StoreSVar | SVar$ LifePaidOnETB | Type$ Calculate | Expression$ AllButOne | SubAbility$ DBLose
SVar:DBLose:DB$ LoseLife | LifeAmount$ LifePaidOnETB
SVar:LifePaidOnETB:Number$0
SVar:AllButOne:Count$YourLifeTotal/Minus.1
T:Mode$ ChangesZone | Origin$ Battlefield | Destination$ Any | ValidCard$ Card.Self | Execute$ TrigRefund | TriggerDescription$ refund
SVar:TrigRefund:DB$ GainLife | LifeAmount$ LifePaidOnETB
`

// TestPayLifeOrggLockstep: the trigger-written form of the same number plays
// identically (no ask: it is the Orgg's own enters trigger).
func TestPayLifeOrggLockstep(t *testing.T) {
	reg := synthReg(t)
	tc := payLifeCase{"Pay Life Orgg", payLifeOrggSrc, [2][]cardset.Entry{
		{{Name: "Pay Life Orgg", Count: 12}, {Name: "Mountain", Count: 28}},
		{{Name: "Doom Blade", Count: 10}, {Name: "Swamp", Count: 30}}}}
	var st payLifeStats
	if div := payLifeRun(t, reg, tc, 40, true, &st, nil); div != 0 {
		t.Fatalf("%d seeds diverged", div)
	}
}
