package diff

import (
	"sort"
	"strings"
)

// canonKey is the canonical form of one param value: whitespace, SVar names,
// lists and booleans, in that order. svars is the face's SVar table.
func canonKey(key, val string, svars map[string]string) string {
	v := space(val)
	if body, ok := svars[v]; ok {
		return "=" + canonBody(key, body)
	}
	return listAndBool(key, v)
}

// canonBody is canonKey on an SVar body, one level: a body that names another
// SVar is left as written.
func canonBody(key, body string) string { return listAndBool(key, space(body)) }

func listAndBool(key, v string) string {
	v = lists(key, v)
	if strings.EqualFold(v, "true") {
		return "True"
	}
	if strings.EqualFold(v, "false") {
		return "False"
	}
	return v
}

func space(s string) string { return strings.Join(strings.Fields(s), " ") }

func lists(key, v string) string {
	switch key {
	case "Cost":
		return v
	case "AddKeyword":
		parts := strings.Split(v, " & ")
		for i := range parts {
			parts[i] = strings.TrimSpace(parts[i])
		}
		sort.Strings(parts)
		return strings.Join(parts, " & ")
	}
	elems := strings.Split(v, ",")
	for i, e := range elems {
		e = strings.TrimSpace(e)
		if dot := strings.Index(e, "."); dot >= 0 && strings.Contains(e[dot:], "+") {
			terms := strings.Split(e[dot+1:], "+")
			sort.Strings(terms)
			e = e[:dot+1] + strings.Join(terms, "+")
		}
		elems[i] = e
	}
	sort.Strings(elems)
	return strings.Join(elems, ",")
}
