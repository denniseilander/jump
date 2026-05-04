package clip

import (
	"fmt"
	"os/exec"
	"strings"
)

// Copy writes text to the system clipboard.
func Copy(text string) error {
	candidates := []string{"pbcopy", "wl-copy", "xclip", "xsel"}
	for _, bin := range candidates {
		path, err := exec.LookPath(bin)
		if err != nil {
			continue
		}
		var args []string
		switch bin {
		case "xclip":
			args = []string{"-selection", "clipboard"}
		case "xsel":
			args = []string{"--clipboard", "--input"}
		}
		cmd := exec.Command(path, args...)
		pipe, err := cmd.StdinPipe()
		if err != nil {
			continue
		}
		if err := cmd.Start(); err != nil {
			continue
		}
		_, _ = pipe.Write([]byte(text))
		_ = pipe.Close()
		return cmd.Wait()
	}
	return fmt.Errorf("no clipboard tool found (tried: %s)", strings.Join(candidates, ", "))
}
