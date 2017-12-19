package main

func newBool(v bool) *bool {
	return &v
}

func toBool(v *bool) bool {
	if v == nil {
		return false
	}
	return *v
}
