package spec

func RequiredDefaultTrue(v *bool) bool {
	if v == nil {
		return true
	}
	return *v
}
