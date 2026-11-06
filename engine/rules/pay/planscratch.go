package pay

// PlanScratch is the payment planner's reusable table storage: the withheld
// census rows (planPaymentCostWithDemand's withhold), the phase tables and
// the class table a plan call builds and throws away. The engine hands one
// out with PlanScratch and takes it back with PutPlanScratch: a take pops the
// pooled scratch (the pool's slot is emptied first, so a nested take
// allocates its own) and a put hands it back whole, the board-scan buffer's
// discipline. Nothing that escapes a plan call may point into it: the search
// copies every adopted plan to fresh storage (adopt), and the query cache's
// classes are built by the plain Classes over the query arena's own rows,
// never by ClassesInto.

// PlanScratch is the planner's table scratch for one plan call.
type PlanScratch struct {
	// outer backs the withheld census's outer rows (planPaymentCostWithDemand).
	outer [][]Alt
	// phaseOuter backs the phase tables' outer rows; it is reused from phase 1
	// to phase 2, whose table replaces the first's before anything reads it.
	phaseOuter [][]Alt
	// arena backs the appended (filtered) Alt rows: withhold's out rows and
	// the phase tables' filtered rows. It is append-only within one plan call
	// (the withheld rows stay live while the phase tables build) and reset
	// when the next call takes the scratch.
	arena []Alt
	// classes backs the class table (ClassesInto).
	classes []Class
	// classCounts is ClassesInto's scratch: each class's member count while
	// the member spans are cut, then the fill cursor.
	classCounts []int
	// membersFlat / pickFlat / irrFlat / painFlat back the class spans
	// (resizeCleared + span split, like the search's countsFlat).
	membersFlat []int
	pickFlat    []int
	irrFlat     []int64
	painFlat    []int64
	// altsSearch is planAlternatives' alternatives walk: a search needs its
	// own working storage, and the plan search's scratch must stay untouched.
	altsSearch SearchScratch
}
