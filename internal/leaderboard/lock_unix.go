//go:build linux || darwin

package leaderboard

import (
	"golang.org/x/sys/unix"
	"os"
)

// LockData holds an OS file lock until release or process death. The stable
// lock file is deliberately retained so other processes lock the same inode.
func LockData(path string) (func(), error) {
	f, err := os.OpenFile(path+".lock", os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return nil, err
	}
	if err = unix.Flock(int(f.Fd()), unix.LOCK_EX|unix.LOCK_NB); err != nil {
		f.Close()
		return nil, err
	}
	return func() { _ = unix.Flock(int(f.Fd()), unix.LOCK_UN); _ = f.Close() }, nil
}
