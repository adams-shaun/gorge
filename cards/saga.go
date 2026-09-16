package cards

import (
	"strconv"
	"strings"
)

// SagaChapters parses a face's K:Chapter parameter (Forge's Saga keyword,
// "Chapter:<count>:<svar1>,<svar2>,...") into its chapter count and the
// per-chapter SVar names, in parameter order. It lives in cards -- the one
// package both the engine (rules) and the event layer (events, whose Move
// grants the entering lore counter) can import -- so the two sides can never
// disagree about what a Saga is.
//
// A face with no Chapter keyword, or a malformed parameter (no count, or a
// non-positive one), returns count 0 and nil: no lore counters are added and
// no chapter ability can resolve -- the same fail-closed convention an
// unparseable T: line gets. Names shorter than the count still return the
// parsed count: the lore counters (and the SBA's final-chapter sacrifice)
// are keyword-driven, while a missing name only means that chapter's ability
// is a no-op at trigger time.
func SagaChapters(f *Face) (int, []string) {
	if f == nil {
		return 0, nil
	}
	raw, ok := f.KeywordParam("Chapter")
	if !ok {
		return 0, nil
	}
	countStr, list, _ := strings.Cut(raw, ":")
	n, err := strconv.Atoi(strings.TrimSpace(countStr))
	if err != nil || n <= 0 {
		return 0, nil
	}
	var names []string
	if strings.TrimSpace(list) != "" {
		for _, nm := range strings.Split(list, ",") {
			if nm = strings.TrimSpace(nm); nm != "" {
				names = append(names, nm)
			}
		}
	}
	return n, names
}
