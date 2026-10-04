//go:build !linux && !darwin && !windows

package leaderboard

import "errors"

func LockData(string) (func(), error) {
	return nil, errors.New("leaderboard persistence requires Linux, macOS or Windows")
}
