package app

import "testing"

func TestCloseFolderGoesBackToHome(t *testing.T) {
	dir := tree(t, map[string]string{"README.md": "# Home\n"})
	z, _, _, _ := newTestZen(t, dir, "README.md", "read")
	if z.Startup().Workspace == nil {
		t.Fatal("no folder open to start with")
	}
	ok, err := z.CloseFolder()
	if err != nil || !ok {
		t.Fatalf("CloseFolder = %v, %v", ok, err)
	}
	st := z.Startup()
	if st.Workspace != nil || st.Page != "" || st.Error != "" {
		t.Fatalf("after closing: %+v", st)
	}
	if _, err := z.Page("README.md"); err == nil {
		t.Fatal("a page of the closed folder still renders")
	}
	// Closing again is harmless.
	if ok, err := z.CloseFolder(); err != nil || !ok {
		t.Fatalf("second CloseFolder = %v, %v", ok, err)
	}
}
