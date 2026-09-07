package main

import (
	"fmt"
	"strings"
)

// normalizeName produces one portable path segment and a valid Agent Skills name.
// Rejecting overlong names, rather than truncating them, avoids hidden collisions.
func normalizeName(name string) (string, error) {
	var b strings.Builder
	separator := false
	for _, r := range name {
		if r >= 'A' && r <= 'Z' {
			r += 'a' - 'A'
		}
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			if separator && b.Len() > 0 {
				b.WriteByte('-')
			}
			b.WriteRune(r)
			separator = false
		} else {
			separator = true
		}
	}
	name = b.String()
	if name == "" {
		return "", fmt.Errorf("name must contain at least one ASCII letter or number")
	}
	// Windows device names remain reserved even when used as directory names.
	switch name {
	case "con", "prn", "aux", "nul", "com1", "com2", "com3", "com4", "com5", "com6", "com7", "com8", "com9", "lpt1", "lpt2", "lpt3", "lpt4", "lpt5", "lpt6", "lpt7", "lpt8", "lpt9":
		name += "-recall"
	}
	if len(name) > 64 {
		return "", fmt.Errorf("normalized name must be at most 64 characters")
	}
	return name, nil
}
