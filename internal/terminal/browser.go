package terminal

import (
	"context"
	"fmt"
	"os/exec"
	"runtime"
)

// OpenCreatorProfile launches only this fixed public URL, with no shell and no
// terminal output. Unsupported/headless systems retain the visible URL.
func OpenCreatorProfile(ctx context.Context, url string) error {
	if url != "https://x.com/c__dursun" {
		return fmt.Errorf("unsupported profile URL")
	}
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "windows":
		cmd = exec.CommandContext(ctx, "rundll32", "url.dll,FileProtocolHandler", url)
	case "darwin":
		cmd = exec.CommandContext(ctx, "open", url)
	default:
		cmd = exec.CommandContext(ctx, "xdg-open", url)
	}
	return cmd.Run()
}
