package dirac

// The stackEntry.trigPl marks of a spell entry. A spell entry's trigPl is read
// only by RefTriggerPlayer (a spell ability has no triggering player),
// buildCost, copy_gen.go and these marks, so each is a distinct value from
// 0x80 up; one block (iota) keeps a new mark from colliding with an old one
// (Airbend's 0x81 once equalled Madness's, pricing a madness cast at the
// airbend cost).
const (
	trigPlSneak    uint8 = 0x80 + iota // kw_sneak.go
	trigPlMadness                      // kw_madness.go
	trigPlMayhem                       // kw_mayhem.go
	trigPlAirbend                      // act_airbend.go
	trigPlWebSling                     // kw_websling.go
	trigPlMayFlash                     // kw_mayflash.go
	trigPlMiracle                      // kw_miracle.go
	trigPlWarpG                        // kw_warpgrant.go: a warp cast at a battlefield grant's cost
)
