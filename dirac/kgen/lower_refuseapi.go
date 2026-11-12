package kgen

// apiRefusalWhy names, for an effect API kgen deliberately does not lower, why
// the kernel cannot model it. refuseAPI appends it to the plain "api X" reason
// (the refusal class stays api:X: refusalClasses reads the word after "api ").
var apiRefusalWhy = map[string]string{
	// PreventDamage lowers (lower_preventnext.go, Ext PreventNext) for one
	// recipient; the shapes it cannot model refuse there, each with its own
	// reason under the same api:PreventDamage class.
}

// refuseAPI is the refusal for an effect API the lowering has no arm for.
func refuseAPI(api string) error {
	if why, ok := apiRefusalWhy[api]; ok {
		return refuse("api %s: %s", api, why)
	}
	return refuse("api %s", api)
}
