package output

import (
	"errors"
	"fmt"
	"os"

	xterm "github.com/charmbracelet/x/term"
	"github.com/denniseilander/jump/internal/search"
)

// ErrQuit is returned when the user explicitly quits the picker (q / Ctrl-C).
var ErrQuit = errors.New("quit")

// InteractivePicker renders an arrow-key selector.
// Returns (result, nil) on confirm, (zero, ErrQuit) on user quit,
// (zero, other) when a TTY is unavailable.
func InteractivePicker(results []search.Result, limit int) (search.Result, error) {
	if limit <= 0 || limit > len(results) {
		limit = len(results)
	}
	results = results[:limit]

	tty, err := os.OpenFile("/dev/tty", os.O_RDWR, 0)
	if err != nil {
		return search.Result{}, err
	}
	defer tty.Close()

	state, err := xterm.MakeRaw(tty.Fd())
	if err != nil {
		return search.Result{}, err
	}
	defer xterm.Restore(tty.Fd(), state) //nolint:errcheck

	// precompute column widths
	aliasW, targetW := 5, 6
	for _, r := range results {
		h := r.Host
		if len(h.Alias) > aliasW {
			aliasW = len(h.Alias)
		}
		if t := target(h); len(t) > targetW {
			targetW = len(t)
		}
	}

	totalLines := len(results) + 2 // results + blank line + hint

	render := func(selected int) {
		fmt.Fprint(tty, "\033[?25l") // hide cursor
		for i, r := range results {
			h := r.Host
			var cursor, aliasStr string
			if i == selected {
				cursor = "▸"
				aliasStr = aliasStyle.Render(h.Alias)
			} else {
				cursor = " "
				aliasStr = h.Alias
			}
			tgt := target(h)
			tags := tagsFrom(h)
			fmt.Fprintf(tty, "\r  %s %-*s  %-*s  %s\033[K\r\n",
				cursor, aliasW, aliasStr, targetW, tgt, dim(tags))
		}
		fmt.Fprintf(tty, "\r\033[K\r\n") // blank line
		fmt.Fprintf(tty, "\r%s\033[K", dim("↑↓ navigate   enter select   q quit"))
		// hint has no \n, so cursor is on the hint line itself — move up totalLines-1
		fmt.Fprintf(tty, "\033[%dA", totalLines-1)
	}

	clear := func() {
		// move past list and erase
		fmt.Fprintf(tty, "\033[%dB", totalLines)
		for i := 0; i <= totalLines; i++ {
			fmt.Fprintf(tty, "\r\033[K\033[1A")
		}
		fmt.Fprint(tty, "\033[?25h") // show cursor
	}

	selected := 0
	render(selected)

	buf := make([]byte, 8)
	for {
		n, err := tty.Read(buf)
		if err != nil || n == 0 {
			clear()
			return search.Result{}, ErrQuit
		}

		switch {
		case buf[0] == '\r' || buf[0] == '\n':
			clear()
			return results[selected], nil

		case buf[0] == 'q' || buf[0] == 3 || buf[0] == 4: // q / Ctrl-C / Ctrl-D
			clear()
			return search.Result{}, ErrQuit

		case buf[0] >= '1' && buf[0] <= '9':
			if idx := int(buf[0]-'1'); idx < len(results) {
				clear()
				return results[idx], nil
			}

		case n >= 3 && buf[0] == '\x1b' && buf[1] == '[':
			switch buf[2] {
			case 'A': // up
				if selected > 0 {
					selected--
				}
			case 'B': // down
				if selected < len(results)-1 {
					selected++
				}
			}
			render(selected)
		}
	}
}
