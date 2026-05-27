package transform

import "fmt"

func SetPath(target map[string]any, path []string, value any) error {
	if len(path) == 0 {
		return fmt.Errorf("empty target path")
	}
	current := target
	for _, segment := range path[:len(path)-1] {
		next, exists := current[segment]
		if !exists {
			child := map[string]any{}
			current[segment] = child
			current = child
			continue
		}
		child, ok := next.(map[string]any)
		if !ok {
			return fmt.Errorf("target path collision at %q", segment)
		}
		current = child
	}
	current[path[len(path)-1]] = value
	return nil
}

func GetPath(target map[string]any, path []string) (any, bool) {
	if len(path) == 0 {
		return nil, false
	}
	current := target
	for _, segment := range path[:len(path)-1] {
		next, exists := current[segment]
		if !exists {
			return nil, false
		}
		child, ok := next.(map[string]any)
		if !ok {
			return nil, false
		}
		current = child
	}
	value, exists := current[path[len(path)-1]]
	return value, exists
}
