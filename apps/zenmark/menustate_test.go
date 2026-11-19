package app

import "testing"

func TestFolderOnlyLabels(t *testing.T) {
	for _, l := range []string{labelNewPage, labelCloseFolder, labelExport, labelExportWebsite, labelPrintPage, labelExportPDF, labelFind, labelFindNext, labelFindPrevious} {
		if !folderOnly(l) {
			t.Errorf("%q should need an open folder", l)
		}
	}
	for _, l := range []string{labelNewSite, labelOpenFolder, labelSettings, "Quit Zenmark", "Copy"} {
		if folderOnly(l) {
			t.Errorf("%q should stay enabled without a folder", l)
		}
	}
}

type fakeItem struct{ enabled bool }

func (f *fakeItem) SetEnabled(on bool) { f.enabled = on }

func TestMenuStateFollowsFolder(t *testing.T) {
	st := newMenuState(false)
	a, b := &fakeItem{enabled: true}, &fakeItem{enabled: true}
	st.add(a.SetEnabled)
	st.add(b.SetEnabled)
	if a.enabled || b.enabled {
		t.Fatal("items must start disabled when no folder is open")
	}
	st.setFolderOpen(true)
	if !a.enabled || !b.enabled {
		t.Fatal("opening a folder must enable the items")
	}
	st.setFolderOpen(false)
	if a.enabled || b.enabled {
		t.Fatal("closing the folder must disable the items")
	}
}

func TestMenuStateInitiallyOpen(t *testing.T) {
	st := newMenuState(true)
	a := &fakeItem{}
	st.add(a.SetEnabled)
	if !a.enabled {
		t.Fatal("a folder given at launch means the items start enabled")
	}
}

func TestMenuStateNilSafe(t *testing.T) {
	var st *menuState
	st.setFolderOpen(true) // headless runs and tests have no menu
}

func TestFolderChangedUpdatesMenu(t *testing.T) {
	z := &Zen{}
	st := newMenuState(false)
	a := &fakeItem{}
	st.add(a.SetEnabled)
	z.menu = st
	z.folderOpened(true)
	if !a.enabled {
		t.Fatal("folderOpened(true) must enable the items")
	}
	z.folderOpened(false)
	if a.enabled {
		t.Fatal("folderOpened(false) must disable the items")
	}
}

// The Export items and the Export submenu entry itself are greyed out without a folder (app/export behaviour 2).
func TestExportFolderOnly(t *testing.T) {
	for _, l := range []string{labelExport, labelExportWebsite, labelExportPDF} {
		if !folderOnly(l) {
			t.Errorf("%q should need an open folder", l)
		}
	}
	if labelExport != "Export" || labelExportWebsite != "Website…" || labelExportPDF != "PDF…" {
		t.Errorf("labels %q %q %q", labelExport, labelExportWebsite, labelExportPDF)
	}
	if folderOnly("Build Website…") || folderOnly("Print Site…") {
		t.Error("the old labels are gone")
	}
}
