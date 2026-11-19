package app

import (
	"github.com/zenmark-org/zenmd/engine/session"
	"github.com/zenmark-org/zenmd/kit/addons"
)

// AddonInfo is one add-on of this build as the frontend sees it (Startup.Addons).
type AddonInfo = session.AddonInfo

// addonInfos lists the add-ons this build includes, in a fixed order (by id).
func addonInfos() []AddonInfo {
	list := addons.List()
	out := make([]AddonInfo, 0, len(list))
	for _, a := range list {
		out = append(out, AddonInfo{ID: a.ID, Name: a.Name, Summary: a.Summary})
	}
	return out
}
