package session

import (
	"os"
	"syscall"
)

// birthTime is the creation time in Unix milliseconds (0 when unknown).
func birthTime(fi os.FileInfo) int64 {
	st, ok := fi.Sys().(*syscall.Stat_t)
	if !ok {
		return 0
	}
	return st.Birthtimespec.Sec*1000 + st.Birthtimespec.Nsec/1e6
}
