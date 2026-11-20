package app

// CloseFolder closes the open folder and goes back to the home screen (File >
// Close Folder). Editor windows close first, each asking about unsaved
// changes; false (and nothing closed) when one keeps its changes.
func (z *Zen) CloseFolder() (r0 bool, err error) {
	defer z.logCall("CloseFolder", &err)
	if z.windows != nil && !z.windows.CloseEditorsWait() {
		return false, nil
	}
	if err := z.s.Open(""); err != nil {
		return false, err
	}
	z.page = ""
	z.folderOpened(false)
	z.setLaunchErr(nil)
	return true, nil
}
