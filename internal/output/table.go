package output

import (
	"fmt"
	"io"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

type Table struct {
	headers []string
	rows    [][]string
	widths  []int // visual widths (no ANSI codes)
}

func NewTable(headers ...string) *Table {
	w := make([]int, len(headers))
	for i, h := range headers {
		w[i] = lipgloss.Width(h)
	}
	return &Table{headers: headers, widths: w}
}

func (t *Table) Row(cols ...string) {
	for len(cols) < len(t.widths) {
		cols = append(cols, "")
	}
	for i, c := range cols {
		if i < len(t.widths) {
			if vw := lipgloss.Width(c); vw > t.widths[i] {
				t.widths[i] = vw
			}
		}
	}
	t.rows = append(t.rows, cols)
}

// pad appends spaces so the visible width of s equals width.
func pad(s string, width int) string {
	vw := lipgloss.Width(s)
	if vw >= width {
		return s
	}
	return s + strings.Repeat(" ", width-vw)
}

func (t *Table) Render(w io.Writer) {
	sep := dimStyle.Render("─")

	// header line
	fmt.Fprint(w, "  ")
	for i, h := range t.headers {
		fmt.Fprint(w, pad(headerStyle.Render(h), t.widths[i]))
		if i < len(t.headers)-1 {
			fmt.Fprint(w, "  ")
		}
	}
	fmt.Fprintln(w)

	// separator line
	fmt.Fprint(w, "  ")
	for i, width := range t.widths {
		fmt.Fprint(w, strings.Repeat(sep, width))
		if i < len(t.widths)-1 {
			fmt.Fprint(w, "  ")
		}
	}
	fmt.Fprintln(w)

	// rows
	for _, row := range t.rows {
		fmt.Fprint(w, "  ")
		for i := range t.widths {
			col := ""
			if i < len(row) {
				col = row[i]
			}
			fmt.Fprint(w, pad(col, t.widths[i]))
			if i < len(t.widths)-1 {
				fmt.Fprint(w, "  ")
			}
		}
		fmt.Fprintln(w)
	}
}
