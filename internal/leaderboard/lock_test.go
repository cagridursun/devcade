package leaderboard

import (
	"path/filepath"
	"testing"
)

func TestOnlyOneProcessMayOwnScoreFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "scores.json")
	unlock, err := LockData(path)
	if err != nil {
		t.Fatal(err)
	}
	release, err := LockData(path)
	if err == nil {
		release()
		unlock()
		t.Fatal("two writers acquired the same score store")
	}
	unlock()
	release, err = LockData(path)
	if err != nil {
		t.Fatal("released lock not reusable", err)
	}
	release()
}
