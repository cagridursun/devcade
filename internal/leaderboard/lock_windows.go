package leaderboard

import (
	"golang.org/x/sys/windows"
	"os"
)

func LockData(path string) (func(), error) {
	f, err := os.OpenFile(path+".lock", os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return nil, err
	}
	var overlapped windows.Overlapped
	h := windows.Handle(f.Fd())
	if err = windows.LockFileEx(h, windows.LOCKFILE_EXCLUSIVE_LOCK|windows.LOCKFILE_FAIL_IMMEDIATELY, 0, 1, 0, &overlapped); err != nil {
		f.Close()
		return nil, err
	}
	return func() { _ = windows.UnlockFileEx(h, 0, 1, 0, &overlapped); _ = f.Close() }, nil
}
