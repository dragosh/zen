package app

import "github.com/zenmark-org/zenmd/kit/desktop"

// revealFile shows a file in Finder (open -R); tests replace it.
var revealFile = func(path string) error { return desktop.OpenFile("-R", path) }

// RevealInFinder shows a page's file in Finder (the Inspector's File section).
// path is relative to the open folder and validated like Page's: only a
// markdown file inside the folder is ever handed to Finder.
func (z *Zen) RevealInFinder(path string) error {
	info, err := z.s.PageInfo(path)
	if err != nil {
		return err
	}
	return revealFile(info.AbsPath)
}
