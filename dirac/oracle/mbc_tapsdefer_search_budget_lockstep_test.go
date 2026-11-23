package oracle

import (
	"testing"
)

// CR 601.2g-h with CR 605.3a-b: the {U}{U} attach of Neurok Stealthsuit is
// payable by activating Unknown Shores ("{1}, {T}: Add one mana of any
// color"), its fee paid from a land's floating mana, beside Silver Myr's
// "{T}: Add {U}" -- a play the planner prices through its budgeted prefix
// search (the fee-carrying Unknown Shores is its census gap, the plain taps
// fund it). Both engines offered the attach while Unknown Shores stayed
// tapped. Then Manabarbs entered: its TapsForMana watcher defers every land
// tap out of the planner's census, so the search's gap list grew from one
// source to the board's seven, and the walk reached the same funding line at
// 163 clones -- three past the pair's stated 160 -- so gorge's search was
// budget-limited, dropped the offer, and the kernel's mirror still priced the
// play (options options, decision 491 of 491; pool tail-34). The limit is the
// pair's stated capacity, so it moved to 164 on both sides (rules
// potential_script.go, dirac/mana_filter.go): the smallest limit past the 163
// clones, because a larger one (176, 192) moves other plays over the same
// boundary -- the kernel's fee-order run prices Otawara's channel in its last
// nodes where gorge's phased run does not. The kernel's name
// TestMBCNeurokStealthsuitReprosLockstep is taken by an earlier Stealthsuit
// ticket, so this one names its cause.
func TestMBCTapsdeferSearchBudgetReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "tapsdefer_search_budget")
}
