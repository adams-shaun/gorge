package oracle

import "testing"

// CR 601.2c, 601.2f, 601.2g: a Chain of Vapor cast free from Urza, Lord High
// Artificer's exile under the opponent's Kasmina ({2} more for a spell that
// targets her side's creatures), with one untapped Plains. Urza's artifact tap
// is a source gorge's window census can't price, so gorge keeps every target on
// the menu (affordableTargetCandidates' census fallback); the kernel skipped
// the window probe for a free cast and withheld the taxed ones (decision 355 of
// dr-mbc-baaa pair 191, Aven Mindcensor among them).
func TestMBCAvenMindcensorReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "free_cast_taxed_target_menu_asks_window_census")
}
