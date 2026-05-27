package spec

import "fmt"

func RejectRefs(shapes map[string]any) error {
	for name, shape := range shapes {
		if err := rejectRefs(shape, "shapes."+name); err != nil {
			return err
		}
	}
	return nil
}

func rejectRefs(v any, path string) error {
	switch x := v.(type) {
	case map[string]any:
		for k, child := range x {
			if k == "$ref" {
				return fmt.Errorf("shapeshifter: $ref is not supported in MVP 1 at %s.$ref", path)
			}
			if err := rejectRefs(child, path+"."+k); err != nil {
				return err
			}
		}
	case map[any]any:
		for k, child := range x {
			key, ok := k.(string)
			if !ok {
				return fmt.Errorf("shapeshifter: non-string schema key at %s", path)
			}
			if key == "$ref" {
				return fmt.Errorf("shapeshifter: $ref is not supported in MVP 1 at %s.$ref", path)
			}
			if err := rejectRefs(child, path+"."+key); err != nil {
				return err
			}
		}
	case []any:
		for i, child := range x {
			if err := rejectRefs(child, fmt.Sprintf("%s[%d]", path, i)); err != nil {
				return err
			}
		}
	}
	return nil
}
