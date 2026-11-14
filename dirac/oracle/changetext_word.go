package oracle

import "strings"

// changeTextWord is the canonical index of a ChangeText word option's label
// (gorge effChangeText's changetext_from / changetext_to): the colour names
// white blue black red green are 0..4 and the basic land types Plains Island
// Swamp Mountain Forest 5..9, both in WUBRG order; -1 for any other label.
func changeTextWord(label string) int {
	for i, w := range [...]string{"white", "blue", "black", "red", "green", "plains", "island", "swamp", "mountain", "forest"} {
		if strings.EqualFold(label, w) {
			return i
		}
	}
	return -1
}
