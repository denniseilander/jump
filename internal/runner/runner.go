package runner

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"
)

var spinFrames = []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}

type teeWriter struct {
	w   io.Writer
	buf bytes.Buffer
}

func (t *teeWriter) Write(p []byte) (n int, err error) {
	t.buf.Write(p)
	return t.w.Write(p)
}

// onFirstWrite signals notify on the first Write call, then forwards all writes.
type onFirstWrite struct {
	w      io.Writer
	once   sync.Once
	notify chan struct{}
}

func (w *onFirstWrite) Write(p []byte) (n int, err error) {
	w.once.Do(func() { close(w.notify) })
	return w.w.Write(p)
}

func SSH(alias string, printOnly bool, connectTimeout int) error {
	if printOnly {
		_, err := os.Stdout.WriteString("ssh " + alias + "\n")
		return err
	}

	args := []string{"-o", fmt.Sprintf("ConnectTimeout=%d", connectTimeout), alias}
	cmd := exec.Command("ssh", args...)
	cmd.Stdin = os.Stdin

	connected := make(chan struct{})
	cmd.Stdout = &onFirstWrite{w: os.Stdout, notify: connected}

	stderrTee := &teeWriter{w: os.Stderr}
	cmd.Stderr = stderrTee

	if err := cmd.Start(); err != nil {
		return err
	}

	done := make(chan struct{})
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		i := 0
		ticker := time.NewTicker(100 * time.Millisecond)
		defer ticker.Stop()
		deadline := time.After(time.Duration(connectTimeout) * time.Second)
		for {
			select {
			case <-connected:
				fmt.Fprintf(os.Stderr, "\r\033[K")
				return
			case <-done:
				fmt.Fprintf(os.Stderr, "\r\033[K")
				return
			case <-deadline:
				fmt.Fprintf(os.Stderr, "\r\033[K")
				return
			case <-ticker.C:
				fmt.Fprintf(os.Stderr, "\r%s connecting...", spinFrames[i%len(spinFrames)])
				i++
			}
		}
	}()

	err := cmd.Wait()
	close(done)
	wg.Wait()

	if err != nil {
		if hint := errorHint(stderrTee.buf.String()); hint != "" {
			fmt.Fprintf(os.Stderr, "hint: %s\n", hint)
		}
	}

	return err
}

func errorHint(stderr string) string {
	s := strings.ToLower(stderr)
	switch {
	case strings.Contains(s, "no route to host"),
		strings.Contains(s, "network is unreachable"),
		strings.Contains(s, "no path to host"):
		return "host unreachable — check VPN or network connection"
	case strings.Contains(s, "operation timed out"),
		strings.Contains(s, "connection timed out"):
		return "connection timed out — check VPN or firewall rules"
	case strings.Contains(s, "connection refused"):
		return "connection refused — SSH service may not be running"
	case strings.Contains(s, "could not resolve hostname"),
		strings.Contains(s, "name or service not known"):
		return "hostname not found — check DNS or VPN"
	case strings.Contains(s, "host key verification failed"):
		return "known_hosts mismatch — run: ssh-keygen -R <hostname>"
	case strings.Contains(s, "permission denied (publickey"),
		strings.Contains(s, "permission denied ("):
		return "authentication failed — check user, identity file, or authorized_keys"
	case strings.Contains(s, "too many authentication failures"):
		return "too many auth attempts — add -o IdentitiesOnly=yes to SSH config"
	}
	return ""
}
