package dirac

// FeatCommander is seat p's commander (CR 903.3), NoObj when the seat has none.
func (s *State) FeatCommander(p uint8) Obj { return s.pl[SeatIx(p)].cmd }

// FeatCommanderGame reports whether s is a Commander game (Config.Commander).
func (s *State) FeatCommanderGame() bool { return s.cfg&cfgCommander != 0 }

// FeatCommanderCast is seat p's commander casts from the command zone
// (CR 903.8).
func (s *State) FeatCommanderCast(p uint8) uint8 { return s.pl[SeatIx(p)].cmdCast }

// FeatCommanderDamage is the combat damage player p has taken from seat q's
// commander (CR 903.10a).
func (s *State) FeatCommanderDamage(p, q uint8) uint8 { return s.pl[SeatIx(p)].cmdDmg[SeatIx(q)] }

// FeatLost reports whether seat p has left the game.
func (s *State) FeatLost(p uint8) bool { return s.pl[SeatIx(p)].lost }
