package kop

// ExBlessing (high expr kind): Sub[0] if the reading controller has the
// city's blessing (CR 702.131), else Sub[1] (gorge Count$Blessing.<yes>.<no>,
// count_body_heads.go's evalCountBodyDottedBlessing arm). The latch is
// dirac/kw_ascend.go's. Explicit ordinal, the iota tail stays untouched.
const ExBlessing ExprKind = 217
