package effects

import "strings"

// sharesReferentSpec adapts Forge's `<Referent>.<bare shares predicate>`
// ValidCards spelling -- Faces of the Past's
// `TriggeredCard.sharesCreatureTypeWith` (the same shape Martyr's Bond's
// `TriggeredCard.sharesCardTypeWith` writes). The grammar's shares-family
// predicates carry their referent as a SECOND token (`<pred> <Referent>`,
// Heirloom Blade's `sharesCreatureTypeWith TriggeredCard`); the bare
// one-token form is the source-anchored reading whose referent defaults to
// Self (Titan of Littjara, Kinfall) -- not what a base token names. When the
// base token IS a referent the two-token form accepts -- verified by asking
// sharesTypeArg the composed token, so only referents the shared table
// already resolves (TriggeredCard, Remembered, Targeted, ...) are rewritten
// -- the base is the shared referent and the candidate universe falls back
// to the API's default. Anything else passes through unchanged and keeps
// whatever the ordinary grammar makes of it (an unrecognised base fails
// closed), so no unrecognised spelling is widened.
func sharesReferentSpec(spec, def string) string {
	base, rest, has := strings.Cut(spec, ".")
	if !has || base == "" || rest == "" {
		return spec
	}
	if _, _, ok := sharesTypeArg(rest + " " + base); !ok {
		return spec
	}
	return def + "." + rest + " " + base
}
