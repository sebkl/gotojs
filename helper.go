package gotojs

// Check if a slice contains a certain string.
func ContainsS(a []string, s string) bool {
	for _,v:= range a {
		if (v == s) {
			return true
		}
	}
	return false
}

// Append string parameter to a string parameter map.
func Append(to map[string]string,from map[string]string) map[string]string {
	for k,v:=range from {
		to[k] = v
	}
	return to
}
