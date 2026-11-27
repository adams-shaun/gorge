package kgen

import "strings"

// playerCounterKind reports a Forge PLAYER counter type (kop's PlCtr*), never
// a permanent counter kind: energy and experience. gorge's CounterAdded
// matcher reads only CounterChange, a permanent's placement, so a
// CounterAddedOnce line naming one never fires there (trigmatch/counters.go).
func playerCounterKind(ct string) bool {
	switch strings.ToUpper(strings.TrimSpace(ct)) {
	case "ENERGY", "EXPERIENCE":
		return true
	}
	return false
}
