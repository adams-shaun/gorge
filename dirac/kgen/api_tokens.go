package kgen

// The api: effects for tokens, copies and face-down creatures (lower_api.go).
func init() {
	apiPS("Token", (*lowerCtx).token)
	apiPS("Clone", (*lowerCtx).cloneAttached) // lower_cloneattached.go
	apiPS("CopyPermanent", (*lowerCtx).copyPermanent)
	apiPS("CopySpellAbility", (*lowerCtx).copySpell)
	apiPS("Incubate", (*lowerCtx).incubate)
	apiPS("Manifest", (*lowerCtx).manifest)
	apiPS("Cloak", (*lowerCtx).cloak)
	apiPS("ManifestDread", (*lowerCtx).manifestDread)
	apiPS("Investigate", (*lowerCtx).investigate)
	apiSA("Myriad", (*lowerCtx).myriad) // lower_myriad.go
}
