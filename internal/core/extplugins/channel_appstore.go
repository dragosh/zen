//go:build appstore

package extplugins

// Available reports whether plugins of kind may run in this build. The App
// Store build refuses what runs a program outside the bundle: "program"
// (a backend from the folder), "tool" and "sandbox" (the container plugin).
// Tengo scripts ("script"), page plugins ("page"), built-ins and layouts stay.
func Available(kind string) error {
	switch kind {
	case "program", "tool", "sandbox":
		return ErrUnavailable
	}
	return nil
}
