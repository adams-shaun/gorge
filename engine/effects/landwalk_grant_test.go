package effects

import (
	"slices"
	"testing"

	"github.com/adams-shaun/gorge/engine/state"
)

// TestResolveGrantedLandwalk: a granted `Landwalk:ChosenType[:desc]` becomes
// landwalk of the HOST's chosen land type, a host with no choice grants
// nothing, and every other keyword passes through untouched.
func TestResolveGrantedLandwalk(t *testing.T) {
	t.Parallel()
	list := []string{"Flying", "Landwalk:ChosenType:desc", "Landwalk:Island", "Landwalk:ChosenTypeX"}
	chose := &state.Object{ChosenType: "Swamp"}
	if got, want := ResolveGrantedLandwalk(list, chose), []string{"Flying", "Landwalk:Swamp:desc", "Landwalk:Island", "Landwalk:ChosenTypeX"}; !slices.Equal(got, want) {
		t.Errorf("host choosing Swamp: %v, want %v", got, want)
	}
	if got, want := ResolveGrantedLandwalk(list, &state.Object{}), []string{"Flying", "Landwalk:Island", "Landwalk:ChosenTypeX"}; !slices.Equal(got, want) {
		t.Errorf("host with no choice: %v, want %v", got, want)
	}
	if got, want := ResolveGrantedLandwalk(list, nil), []string{"Flying", "Landwalk:Island", "Landwalk:ChosenTypeX"}; !slices.Equal(got, want) {
		t.Errorf("no host: %v, want %v", got, want)
	}
	plain := []string{"Flying", "Landwalk:Island"}
	if got := ResolveGrantedLandwalk(plain, chose); &got[0] != &plain[0] {
		t.Errorf("a list with no chosen-type landwalk was copied")
	}
	if chose.ChosenType != "Swamp" || list[1] != "Landwalk:ChosenType:desc" {
		t.Errorf("the host or the input list was mutated: %q %v", chose.ChosenType, list)
	}
}
