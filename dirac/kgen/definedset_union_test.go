package kgen

import (
	"strings"
	"testing"
)

// ksetBuildRefusal lowers and builds c, returning every refusal reason.
func ksetBuildRefusal(t *testing.T, c *lCard) string {
	t.Helper()
	if _, _, _, err := buildPool(&lowered{cards: []*lCard{c}}); err != nil {
		t.Fatal(err)
	}
	var rs []string
	for _, r := range c.Refusals {
		rs = append(rs, r.Reason)
	}
	return strings.Join(rs, "; ")
}

// TestDefinedSetUnionOverlap: the kernel's SoUnion de-duplicates a member two
// parts both name; gorge's plain " & " path does not, so an object named twice
// is acted on twice. A union on an op that is not idempotent for an object
// (PumpUntilEOT, Destroy; PutCounter's own union path, putCounterUnion,
// already hits a twice-named object twice, as gorge does) refuses at build; a union on an op that
// does nothing the second time (Tap) builds. "Targeted & Self" on a source
// that is its own target is the overlapping case.
func TestDefinedSetUnionOverlap(t *testing.T) {
	for _, api := range []struct{ name, line string }{
		{"PumpUntilEOT", "SP$ Pump | ValidTgts$ Creature | Defined$ Targeted & Self | NumAtt$ +1"},
		{"Destroy", "SP$ Destroy | ValidTgts$ Creature | Defined$ Targeted & Self"},
	} {
		t.Run(api.name+" refuses", func(t *testing.T) {
			c := ksetCard(t, "Sorcery", api.line, "", map[string]string{}, nil)
			if len(c.Refusals) != 0 {
				t.Skipf("LIR already refuses (nothing reaches the set builder): %+v", c.Refusals)
			}
			got := ksetBuildRefusal(t, c)
			if !strings.Contains(got, "set union on "+api.name) {
				t.Fatalf("want a %q union refusal, got %q", api.name, got)
			}
		})
	}
	t.Run("Tap builds", func(t *testing.T) {
		c := ksetCard(t, "Sorcery", "SP$ Tap | ValidTgts$ Creature | Defined$ Targeted & Self", "", map[string]string{}, nil)
		if len(c.Refusals) != 0 {
			t.Fatalf("refused: %+v", c.Refusals)
		}
		if got := ksetBuildRefusal(t, c); got != "" {
			t.Fatalf("a union on Tap should build, got %q", got)
		}
	})
}

// TestDefinedSetTargetedSlots: a Defined$ read of a target slot that can hold
// several objects, or of several slots, refuses at the set base (the kernel
// keeps only a slot's first object in its target cell). The targets sit on the
// parent and the sub reads them, the shape that reaches the set builder (a
// node that declares its own ValidTgts$ beside Defined$ refuses earlier).
func TestDefinedSetTargetedSlots(t *testing.T) {
	const multi = "SP$ Tap | ValidTgts$ Creature | TargetMin$ 1 | TargetMax$ 2 | SubAbility$ DBX"
	const single = "SP$ Tap | ValidTgts$ Creature | SubAbility$ DBX"
	for _, tc := range []struct {
		name, root, sub string
		svars           map[string]string
		want            string // "" = lowers
	}{
		{"dotted multi-object slot", multi, "DB$ Tap | Defined$ Targeted.YouCtrl", nil, `Defined$ "Targeted.YouCtrl"`},
		{"bare union part, multi-object slot", multi, "DB$ Tap | Defined$ Targeted & Self", nil, `Defined$ "Targeted & Self"`},
		{"dotted, two slots", single, "DB$ Tap | ValidTgts$ Creature | SubAbility$ DBY",
			map[string]string{"DBY": "DB$ Tap | Defined$ Targeted.YouCtrl"}, `Defined$ "Targeted.YouCtrl"`},
		{"dotted, single slot lowers", single, "DB$ Tap | Defined$ Targeted.YouCtrl", nil, ""},
		{"bare union part, single slot lowers", single, "DB$ Tap | Defined$ Targeted & Self", nil, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			sv := map[string]string{}
			for k, v := range tc.svars {
				sv[k] = v
			}
			c := ksetCard(t, "Sorcery", tc.root, tc.sub, sv, nil)
			var rs []string
			for _, r := range c.Refusals {
				rs = append(rs, r.Reason)
			}
			got := strings.Join(rs, "; ")
			if tc.want == "" {
				if got != "" {
					t.Fatalf("refused: %s", got)
				}
				return
			}
			if !strings.Contains(got, tc.want) {
				t.Fatalf("want a refusal containing %q, got %q\n%s", tc.want, got, ksetDump(c))
			}
		})
	}
}

// TestDefinedSetUnionMoveOrigin: a union on a zone move passes the union check
// only when the node's Origin$ excludes its Destination$ (gorge's
// changeZoneMoveSkip then skips an object named twice); with no Origin$ the
// second move would happen, so it refuses as a union. The gated case still
// refuses at build, as an ordered set on a suspending op (a union is an
// OrderList set), which is the later guard, not the union one. An Untap union refuses too (TryUntap spends a stun
// counter on the first pass and untaps on the second).
func TestDefinedSetUnionMoveOrigin(t *testing.T) {
	for _, tc := range []struct {
		name, line string
		want       string
	}{
		{"exile with origin", "SP$ ChangeZone | Defined$ Valid Creature.YouCtrl & Self | Origin$ Battlefield | Destination$ Exile", "ordered set on a suspending op Exile"},
		{"exile no origin", "SP$ ChangeZone | Defined$ Valid Creature.YouCtrl & Self | Destination$ Exile", "set union on Exile"},
		{"origin equals destination", "SP$ ChangeZone | Defined$ Valid Creature.YouCtrl & Self | Origin$ Exile | Destination$ Exile", "set union on Exile"},
		{"untap", "SP$ Untap | Defined$ Valid Creature.YouCtrl & Self", "set union on Untap"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c := ksetCard(t, "Sorcery", tc.line, "", map[string]string{}, nil)
			if len(c.Refusals) != 0 {
				t.Fatalf("LIR refuses, so nothing reaches the set builder: %+v", c.Refusals)
			}
			got := ksetBuildRefusal(t, c)
			if tc.want == "" {
				if got != "" {
					t.Fatalf("want a clean build, got %q", got)
				}
				return
			}
			if !strings.Contains(got, tc.want) {
				t.Fatalf("want %q, got %q", tc.want, got)
			}
		})
	}
}
