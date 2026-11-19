//go:build addon_vale

package app

// The Vale linting add-on is linked only in builds made with ADDONS=vale
// (build tag addon_vale; kit/addons).
import _ "github.com/zenmark-org/zenmd/addons/vale"
