package searchseat

import (
	"testing"

	"github.com/adams-shaun/gorge/botpolicy"
	"github.com/adams-shaun/gorge/decision"
	"github.com/adams-shaun/gorge/internal/searchprobe"
)

// Defaults must BE cmd/searchteacher's flag defaults, because those are the
// knobs the +5.80pp +/- 1.25 paired-dev result was measured with. If someone
// retunes a default here, "the seat plays the teacher" silently stops meaning
// the teacher that was measured -- so the values are pinned literally rather
// than asserted loosely.
func TestDefaultsAreTheMeasuredKnobs(t *testing.T) {
	d := Defaults()
	if d.Worlds != 8 || d.Attempts != 64 || d.Limit != 6 || d.MaxSubmits != 5000 {
		t.Errorf("worlds/attempts/limit/max-submits = %d/%d/%d/%d, want 8/64/6/5000",
			d.Worlds, d.Attempts, d.Limit, d.MaxSubmits)
	}
	if d.MinESS != 0 || d.Margin != 0 || d.HorizonTurns != 0 {
		t.Errorf("min-ess/margin/horizon = %v/%v/%v, want 0/0/0 (0 horizon = roll to game end)",
			d.MinESS, d.Margin, d.HorizonTurns)
	}
	if d.SampleSeed != 54321 {
		t.Errorf("SampleSeed = %d, want 54321", d.SampleSeed)
	}
	if d.Clairvoyant {
		t.Error("Clairvoyant must default false: it cheats by construction and is a measurement ceiling only")
	}
	if !d.Kinds["attackers"] || !d.Kinds["cast"] {
		t.Errorf("Kinds = %v, want both implemented kinds on", d.Kinds)
	}
}

// CastOptions counts DISTINCT objects, not options. One card offered several
// ways (an alternative cost, a kicked mode) is one choice of card, so a
// priority decision offering the same object twice is not a two-candidate
// decision and must not open a search.
func TestCastOptionsCountsDistinctObjects(t *testing.T) {
	for _, tc := range []struct {
		name string
		opts []decision.Option
		want int
	}{
		{"none", []decision.Option{{Kind: "pass"}}, 0},
		{"one card", []decision.Option{{Kind: "cast", Obj: 10}, {Kind: "pass"}}, 1},
		{"same card twice", []decision.Option{{Kind: "cast", Obj: 10}, {Kind: "cast", Obj: 10}}, 1},
		{"two cards", []decision.Option{{Kind: "cast", Obj: 10}, {Kind: "cast", Obj: 11}}, 2},
		{"non-cast ignored", []decision.Option{{Kind: "cast", Obj: 10}, {Kind: "ability", Obj: 11}}, 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			d := &decision.Decision{Kind: decision.KPriority, Options: tc.opts}
			if got := CastOptions(d); got != tc.want {
				t.Errorf("CastOptions = %d, want %d", got, tc.want)
			}
		})
	}
}

// CastOptions IS botpolicy.CastableObjects: the teacher's priority
// eligibility and the learned seat's priority gate (seat.PolicyNetBot) read
// one definition, so the distribution the head is trained on and the one it
// is asked to answer cannot drift. Pinned on hand-built decisions covering
// every branch of the count.
func TestCastOptionsIsTheBotpolicyCount(t *testing.T) {
	for _, opts := range [][]decision.Option{
		nil,
		{{Kind: "pass"}},
		{{Kind: "cast", Obj: 10}, {Kind: "pass"}},
		{{Kind: "cast", Obj: 10}, {Kind: "cast", Obj: 10}, {Kind: "pass"}},
		{{Kind: "cast", Obj: 10}, {Kind: "cast", Obj: 11}, {Kind: "ability", Obj: 12}, {Kind: "pass"}},
		{{Kind: "play_land", Obj: 9}, {Kind: "activate", Obj: 8}, {Kind: "cast", Obj: 10}, {Kind: "cast", Obj: 11}, {Kind: "cast", Obj: 12}},
	} {
		d := &decision.Decision{Kind: decision.KPriority, Options: opts}
		if got, want := CastOptions(d), botpolicy.CastableObjects(d); got != want {
			t.Errorf("options %+v: CastOptions = %d, botpolicy.CastableObjects = %d", opts, got, want)
		}
	}
}

// Eligible is the cheap pre-test a driver uses to decide whether a decision is
// worth observing at all, so it must agree exactly with the arm Choose would
// take. The kinds gate is part of that: a caller that turned a kind off must
// see it declined here too, or it would pay for sampling Choose then refuses.
func TestEligibleMatchesTheImplementedKinds(t *testing.T) {
	on := Defaults()
	twoCasts := &decision.Decision{Kind: decision.KPriority, Options: []decision.Option{
		{Kind: "cast", Obj: 1}, {Kind: "cast", Obj: 2}, {Kind: "pass"},
	}}
	oneCast := &decision.Decision{Kind: decision.KPriority, Options: []decision.Option{
		{Kind: "cast", Obj: 1}, {Kind: "pass"},
	}}
	attackers := &decision.Decision{Kind: decision.KAttackers}

	if !Eligible(twoCasts, on) {
		t.Error("a priority decision with two distinct castable objects should be eligible")
	}
	if Eligible(oneCast, on) {
		t.Error("a priority decision with ONE castable object is not a choice between casts; want ineligible")
	}
	if !Eligible(attackers, on) {
		t.Error("an attackers decision should be eligible")
	}

	// Every other kind delegates. These are the kinds the teacher has never
	// covered; listing them explicitly is what would catch a future arm added
	// to Choose without Eligible learning about it.
	for _, k := range []decision.Kind{
		decision.KTarget, decision.KBlockers, decision.KChoose, decision.KModes,
		decision.KMulligan, decision.KTriggerOrder, decision.KTriggerOptional,
		decision.KCommanderZone, decision.KReplacement, decision.KArrange,
	} {
		if Eligible(&decision.Decision{Kind: k}, on) {
			t.Errorf("kind %q should delegate, but Eligible returned true", k)
		}
	}
}

func TestManaTapSearchIsOptInAndBareOnly(t *testing.T) {
	d := &decision.Decision{Kind: decision.KPriority, Options: []decision.Option{
		{Index: 0, Kind: "activate", Obj: 1},
		{Index: 1, Kind: "activate", Obj: 2, Cost: "PayLife<2>"},
		{Index: 2, Kind: "activate", Obj: 3},
		{Index: 3, Kind: "pass"},
	}}
	opts := Defaults()
	if Eligible(d, opts) {
		t.Fatal("default search must not branch over mana taps")
	}
	opts.Kinds["mana"] = true
	if !Eligible(d, opts) {
		t.Fatal("two bare mana taps must be eligible when enabled")
	}
	if got := bareManaAlternatives(d, 0); len(got) != 1 || got[0] != 2 {
		t.Fatalf("bare alternatives = %v, want [2]", got)
	}
	d.Options[2].Cost = "Sac<1/Creature>"
	if Eligible(d, opts) {
		t.Fatal("a costly second source must not open the bare-tap arm")
	}
}

// recordSample is the ONLY transport between the sampler's SampleResult and
// the cost report's per-bucket rejection census: botbench reads
// Trace.Rejections, and nothing else copies SampleResult.Rejections into a
// Trace. If this copy is dropped the report still renders, but every
// rejection-shape line is empty and a late "insufficient worlds/ESS"
// fallback can no longer be attributed to a sampler rejection reason -- the
// exact diagnostic the cost report exists to produce. The test therefore
// pins the transport itself, not the report renderer (whose fixture builds
// Trace values directly and so cannot see this copy disappear).
func TestRecordSampleCarriesEveryRejectionBucket(t *testing.T) {
	// Distinct component/shape pairs and counts, deliberately out of sort
	// order in the source, so a transport that reorders or drops entries is
	// visible. Frame is carried too: the report aggregates by component/shape,
	// but a copy that lost the frame would still be a lossy census.
	sr := searchprobe.SampleResult{
		Attempts: 64, Accepted: 2, PrefixRejected: 62, ESS: 1.25,
		Rejections: []searchprobe.RejectionBucket{
			{Frame: 3, Component: "identities", Shape: "hand_to_stack", Count: 41},
			{Frame: 1, Component: "board", Shape: "state", Count: 9},
			{Frame: 2, Component: "events", Shape: "count", Count: 12},
		},
	}

	var tr Trace
	recordSample(&tr, sr)

	// Precondition: the source really carries buckets, or the assertions
	// below would be asserting zero equals zero.
	if len(sr.Rejections) != 3 {
		t.Fatalf("fixture precondition: sr.Rejections has %d buckets, want 3", len(sr.Rejections))
	}
	if len(tr.Rejections) != len(sr.Rejections) {
		t.Fatalf("Trace.Rejections has %d buckets, want %d (recordSample must carry every bucket)",
			len(tr.Rejections), len(sr.Rejections))
	}
	for i, want := range sr.Rejections {
		if got := tr.Rejections[i]; got != want {
			t.Errorf("Trace.Rejections[%d] = %+v, want %+v", i, got, want)
		}
	}

	// The trace must own its slice: a later in-place edit of the sampler's
	// result (the sampler grows and rewrites this slice across attempts) must
	// never mutate diagnostics already recorded for a previous decision.
	sr.Rejections[0].Count = 999
	sr.Rejections = append(sr.Rejections, searchprobe.RejectionBucket{Frame: 4, Component: "decision", Shape: "missing", Count: 1})
	if tr.Rejections[0].Count != 41 {
		t.Errorf("Trace.Rejections[0].Count = %d after mutating the source, want 41: the copy must not alias sr.Rejections",
			tr.Rejections[0].Count)
	}
	if len(tr.Rejections) != 3 {
		t.Errorf("Trace.Rejections grew to %d after appending to the source, want 3: the copy must not share the source's backing array",
			len(tr.Rejections))
	}

	// TopRejection is derived from the same census and is what a one-line
	// summary reads; it must name the largest bucket (identities/hand_to_stack
	// at 41).
	if tr.TopRejection != "identities/hand_to_stack" {
		t.Errorf("TopRejection = %q, want %q (the largest bucket)", tr.TopRejection, "identities/hand_to_stack")
	}
}

// A caller may turn a kind off, and then that kind must delegate even when its
// shape would otherwise qualify.
func TestEligibleHonoursTheKindsGate(t *testing.T) {
	off := Defaults()
	off.Kinds = map[string]bool{"attackers": true} // cast deliberately absent

	twoCasts := &decision.Decision{Kind: decision.KPriority, Options: []decision.Option{
		{Kind: "cast", Obj: 1}, {Kind: "cast", Obj: 2},
	}}
	if Eligible(twoCasts, off) {
		t.Error("cast is off, so a cast decision must delegate")
	}
	if !Eligible(&decision.Decision{Kind: decision.KAttackers}, off) {
		t.Error("attackers is on and must stay eligible")
	}

	none := Defaults()
	none.Kinds = nil
	if Eligible(twoCasts, none) || Eligible(&decision.Decision{Kind: decision.KAttackers}, none) {
		t.Error("a nil Kinds map must delegate everything rather than defaulting to on")
	}
}

// "blockers" is opt-in: Defaults leaves it off (so the measured seat is
// unchanged), and turning it on admits every KBlockers decision -- the
// candidate builder, not Eligible, decides whether it has two to compare.
func TestEligibleBlockersIsOptIn(t *testing.T) {
	blockers := &decision.Decision{Kind: decision.KBlockers}
	if Eligible(blockers, Defaults()) {
		t.Error("Defaults must keep blockers off")
	}
	on := Defaults()
	on.Kinds = map[string]bool{"blockers": true}
	if !Eligible(blockers, on) {
		t.Error("blockers is on, so a KBlockers decision must be eligible")
	}
	if Eligible(&decision.Decision{Kind: decision.KAttackers}, on) {
		t.Error("attackers is off, so an attackers decision must delegate")
	}
}

// "target" is opt-in and admits only the single-choice, unbudgeted KTarget
// shape (searchprobe.SingleTarget); every other target ask stays the bot's.
func TestEligibleTargetIsOptInAndSingleChoice(t *testing.T) {
	opts := []decision.Option{{Index: 0, Kind: "target", Obj: 1}, {Index: 1, Kind: "target", Player: 1}}
	single := &decision.Decision{Kind: decision.KTarget, Min: 1, Max: 1, Options: opts}
	if Eligible(single, Defaults()) {
		t.Error("Defaults must keep target off")
	}
	on := Defaults()
	on.Kinds = map[string]bool{"target": true}
	if !Eligible(single, on) {
		t.Error("target is on, so a single-choice KTarget must be eligible")
	}
	for name, d := range map[string]*decision.Decision{
		"max 2":      {Kind: decision.KTarget, Min: 1, Max: 2, Options: opts},
		"min 0":      {Kind: decision.KTarget, Min: 0, Max: 1, Options: opts},
		"max sum":    {Kind: decision.KTarget, Min: 1, Max: 1, MaxSum: 3, Options: opts},
		"budgeted":   {Kind: decision.KTarget, Min: 1, Max: 1, Budgeted: true, Options: opts},
		"one option": {Kind: decision.KTarget, Min: 1, Max: 1, Options: opts[:1]},
		"attackers":  {Kind: decision.KAttackers},
	} {
		if Eligible(d, on) {
			t.Errorf("%s: want ineligible", name)
		}
	}
}
