package transform

import "fmt"

func ParseObjectPath(path string) ([]string, error) {
	if path == "" {
		return nil, fmt.Errorf("empty path")
	}
	if path[0] != '.' {
		return nil, fmt.Errorf("path must start with '.'")
	}
	if path == "." {
		return nil, fmt.Errorf("path must include at least one segment")
	}

	var segments []string
	start := 1
	for i := 1; i <= len(path); i++ {
		if i == len(path) || path[i] == '.' {
			if i == start {
				return nil, fmt.Errorf("empty path segment")
			}
			segment := path[start:i]
			if !validIdent(segment) {
				return nil, fmt.Errorf("invalid path segment %q", segment)
			}
			segments = append(segments, segment)
			start = i + 1
		}
	}
	return segments, nil
}

func validIdent(s string) bool {
	if s == "" {
		return false
	}
	for i, r := range s {
		if i == 0 {
			if (r >= 'A' && r <= 'Z') || (r >= 'a' && r <= 'z') || r == '_' {
				continue
			}
			return false
		}
		if (r >= 'A' && r <= 'Z') || (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '_' {
			continue
		}
		return false
	}
	return true
}

func PathString(path []string) string {
	if len(path) == 0 {
		return "."
	}
	out := ""
	for _, segment := range path {
		out += "." + segment
	}
	return out
}
