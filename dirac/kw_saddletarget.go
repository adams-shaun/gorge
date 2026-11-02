package dirac

// DebugSaddleTgtNoClear is the mutation hook for
// TestSaddletgtMutation. unsaddleAll (kw_crewsaddle.go) keeps a saddled
// designation into the next turn instead of clearing it at the turn change,
// so a Mount saddled by Guidelight Matrix or Alacrian Armory is still saddled
// when it attacks; the lockstep must diverge from gorge (which clears it).
var DebugSaddleTgtNoClear bool
