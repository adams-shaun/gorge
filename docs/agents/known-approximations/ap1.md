# ap1

**Stand-in.** (ap1) AddPhase loud-degrades (one Note, grant still applied) on an unresolvable `ExtraPhase$`/`AfterPhase$`/`FollowedBy$`/`NumPhases$`/DelTrig value or a multi-step set. Deliberate: a grant whose splice point has passed is dropped silently at TurnChange; a splice mid-range of another active extra phase is untested; a multi-completion takes the LAST entry's resume point. `FirstAttack$` unread.

**Where.** `engine/effects/addphase.go` (`effAddPhase`, `parseExtraPhaseValue`), `engine/rules/turn.go` (`extraPhaseBoundary`)

**Removed by.** M4 (a real per-turn attack history for the unread `FirstAttack$` gate)
