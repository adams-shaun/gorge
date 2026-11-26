package oracle

import (
	"encoding/json"
	"fmt"
	"slices"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac/canon"
)

// labelChoose is the canonical creature_type choose of word, as optionAction
// packs it.
func labelChoose(t *testing.T, word string) Canon {
	t.Helper()
	i := slices.Index(chooseKinds, "creature_type")
	if i < 0 {
		t.Fatal("no creature_type choose kind")
	}
	h := labelValue(word)
	return Pack(ActChoose+canon.CanonKind(i), RefNone, Ref(h>>8), uint8(h))
}

// labelPick serves one decision offering opts and returns the pick, or the
// panic text the replay failed with.
func labelPick(s *ScriptPolicy, opts ...Canon) (pick Canon, fail string) {
	defer func() {
		if r := recover(); r != nil {
			fail = fmt.Sprint(r)
		}
	}()
	return s.Pick(&PickCtx{D: &CDecision{Options: opts}, Seq: 7}), ""
}

func TestScriptPolicyLabelMatches(t *testing.T) {
	soldier, elf, goblin := labelChoose(t, "Soldier"), labelChoose(t, "Elf"), labelChoose(t, "Goblin")
	if soldier == elf || soldier == goblin {
		t.Fatal("the words pack alike: the case cannot tell them apart")
	}
	s := &ScriptPolicy{Actions: []Canon{soldier}, Labels: []ActionLabel{{At: 0, Label: "Soldier"}}}
	pick, fail := labelPick(s, elf, soldier, goblin)
	if fail != "" || pick != soldier || s.Skipped != 0 {
		t.Fatalf("pick %v fail %q skipped %d, want %v served", pick, fail, s.Skipped, soldier)
	}
}

func TestScriptPolicyLabelMismatchFails(t *testing.T) {
	soldier, elf, goblin := labelChoose(t, "Soldier"), labelChoose(t, "Elf"), labelChoose(t, "Goblin")
	// The int names an offered option that is another word: the vocabulary
	// shifted under the recording.
	s := &ScriptPolicy{Actions: []Canon{elf}, Labels: []ActionLabel{{At: 0, Label: "Soldier"}}}
	_, fail := labelPick(s, elf, soldier, goblin)
	for _, want := range []string{`"Soldier"`, "stale", soldier.String(), goblin.String()} {
		if !strings.Contains(fail, want) {
			t.Errorf("failure %q does not name %q", fail, want)
		}
	}
	// The int names no offered option, as the Horn repro's flipped pick.
	s = &ScriptPolicy{Actions: []Canon{Canon(0x9ffe013b)}, Labels: []ActionLabel{{At: 0, Label: "Soldier"}}}
	if _, fail = labelPick(s, elf, soldier, goblin); !strings.Contains(fail, `"Soldier"`) || !strings.Contains(fail, elf.String()) {
		t.Errorf("failure %q does not name the label and the offered options", fail)
	}
	// The label is not offered at all and the int is: the label does not name the pick.
	s = &ScriptPolicy{Actions: []Canon{elf}, Labels: []ActionLabel{{At: 0, Label: "Soldier"}}}
	if _, fail = labelPick(s, elf, goblin); !strings.Contains(fail, "does not name") {
		t.Errorf("failure %q, want the label not naming the offered pick", fail)
	}
	// Remap serves the label's option instead.
	s = &ScriptPolicy{Actions: []Canon{elf}, Labels: []ActionLabel{{At: 0, Label: "Soldier"}}, Remap: true}
	if pick, fail := labelPick(s, elf, soldier); fail != "" || pick != soldier {
		t.Errorf("remap picked %v (%q), want %v", pick, fail, soldier)
	}
}

func TestScriptPolicyLabelSkippedFails(t *testing.T) {
	soldier, elf := labelChoose(t, "Soldier"), labelChoose(t, "Elf")
	s := &ScriptPolicy{Actions: []Canon{soldier}}
	_, fail := labelPick(s, elf)
	if !strings.Contains(fail, "allow_skips") || !strings.Contains(fail, elf.String()) {
		t.Fatalf("failure %q, want the skip refused naming the offered options", fail)
	}
	s = &ScriptPolicy{Actions: []Canon{soldier}, AllowSkips: true}
	if pick, fail := labelPick(s, elf); fail != "" || s.Skipped != 1 || pick != elf {
		t.Fatalf("opted in: pick %v fail %q skipped %d, want the skip counted and the tail's pick", pick, fail, s.Skipped)
	}
	// The repro's field is the opt-in.
	var r Repro
	if err := json.Unmarshal([]byte(`{"actions":[`+fmt.Sprint(uint64(soldier))+`],"allow_skips":true}`), &r); err != nil {
		t.Fatal(err)
	}
	if !r.Script().AllowSkips {
		t.Fatal("Repro.AllowSkips did not reach its ScriptPolicy")
	}
}

func TestScriptPolicyLabelOldReproReplays(t *testing.T) {
	soldier, elf := labelChoose(t, "Soldier"), labelChoose(t, "Elf")
	var r Repro
	if err := json.Unmarshal([]byte(`{"actions":[`+fmt.Sprint(uint64(soldier))+`]}`), &r); err != nil {
		t.Fatal(err)
	}
	if r.Labels != nil || r.AllowSkips {
		t.Fatalf("an old repro decoded with labels %v / allow_skips %v", r.Labels, r.AllowSkips)
	}
	pick, fail := labelPick(r.Script(), elf, soldier)
	if fail != "" || pick != soldier {
		t.Fatalf("old repro: pick %v fail %q, want %v", pick, fail, soldier)
	}
	b, err := json.Marshal(&r)
	if err != nil || strings.Contains(string(b), "labels") || strings.Contains(string(b), "allow_skips") {
		t.Fatalf("an unlabelled repro wrote %s (%v): the corpus bytes would change", b, err)
	}
}

// hornChosenPick is the Horn repro's chosen-type pick: Soldier at action
// hornChosenPickAt, as the creature-type vocabulary indexed it before #4601
// (the int the corpus carried until hotfix #5050, 0x9ffe013b, is the stale one).
const (
	hornChosenPickAt = 2019
	hornStalePick    = Canon(0x9ffe013b)
)

// TestFourSeatHornChosenTypeRevealLabelled pins the Horn repro's label: the
// recording names its pick, and the lockstep replay's own labels (what a
// fresh recording would write) are the repro's.
func TestFourSeatHornChosenTypeRevealLabelled(t *testing.T) {
	if isolate(t) {
		return
	}
	res := fourSeatEdhReplay(t, hornChosenRepro)
	r, err := ReadRepro("testdata/repro/" + hornChosenRepro + ".repro.json")
	if err != nil {
		t.Fatal(err)
	}
	want := []ActionLabel{{At: hornChosenPickAt, Label: "Soldier"}}
	if !slices.Equal(r.Labels, want) {
		t.Fatalf("repro labels %v, want %v", r.Labels, want)
	}
	if k := chooseKindOf(r.Actions[hornChosenPickAt]); k != "type" {
		t.Fatalf("action %d is %q, want the type pick", hornChosenPickAt, k)
	}
	var got []ActionLabel
	for _, l := range res.Labels {
		if l.At < len(r.Actions) {
			got = append(got, l)
		}
	}
	if !slices.Equal(got, want) {
		t.Fatalf("replay labelled %v, want %v", got, want)
	}
}

// TestFourSeatHornChosenTypeRevealStalePickFails flips the pick back to the
// int it held before the vocabulary shift: the replay must fail naming the
// label and the offered options, not drop the pick and lose its subject.
func TestFourSeatHornChosenTypeRevealStalePickFails(t *testing.T) {
	if isolate(t) {
		return
	}
	r, err := ReadRepro("testdata/repro/" + hornChosenRepro + ".repro.json")
	if err != nil {
		t.Fatal(err)
	}
	if r.Actions[hornChosenPickAt] == hornStalePick {
		t.Fatal("the repro already holds the stale pick: the flip tests nothing")
	}
	r.Actions = slices.Clone(r.Actions)
	r.Actions[hornChosenPickAt] = hornStalePick
	res := fourSeatEdhReplayOf(t, synthReg(t), r)
	if res.Div == nil || res.Div.What != "policy" {
		t.Fatalf("the stale pick replayed (div %+v): the label did not catch it", res.Div)
	}
	for _, want := range []string{`"Soldier"`, "offered: [", "choose:type(-,#1,58)"} {
		if !strings.Contains(res.Div.Detail, want) {
			t.Errorf("failure does not name %q:\n%.600s", want, res.Div.Detail)
		}
	}
}
