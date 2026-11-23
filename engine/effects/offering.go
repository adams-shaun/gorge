package effects

// kw:Offering (CR 702.48) is a casting option read directly off the K: line by
// engine/rules/offering.go (an "offering" cast mode beside the ordinary cast),
// registered here as the Crew/Craft markers are.
func init() { RegisterNonAPI("kw:Offering") }
