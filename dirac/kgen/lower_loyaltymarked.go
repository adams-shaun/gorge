package kgen

import "strings"

// loyaltyMarked reads an ability's Planeswalker$ marker as gorge's
// isLoyaltyMarked does: case-insensitively, so Jaya, Venerated Firemage's
// and Kiora, Behemoth Beckoner's "Planeswalker$ true" is a loyalty ability
// (sorcery speed, once per turn; CR 606.3) like every "True".
func loyaltyMarked(v string) bool { return strings.EqualFold(strings.TrimSpace(v), "True") }
