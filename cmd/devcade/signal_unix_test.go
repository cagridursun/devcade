//go:build unix

package main

import (
	"context"
	"syscall"
	"testing"

	"github.com/cagridursun/devcade/internal/terminal"
)

func TestSignalsCancelPlayAndSetExitStatus(t *testing.T) {
	for _, tc := range []struct {
		sig  syscall.Signal
		want int
	}{
		{syscall.SIGTERM, exitTerm},
		{syscall.SIGINT, exitInterrupt},
	} {
		stub(t, nil, nil)
		play = func(ctx context.Context, _ terminal.Program) error {
			if err := syscall.Kill(syscall.Getpid(), tc.sig); err != nil {
				t.Error(err)
			}
			<-ctx.Done() // the loop returns nil on cancellation
			return nil
		}
		if code, _, errOut := runArgs(); code != tc.want {
			t.Errorf("%v: code=%d stderr=%q", tc.sig, code, errOut)
		}
	}
}
