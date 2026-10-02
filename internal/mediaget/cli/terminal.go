package mediacli

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"unicode"

	core "github.com/matheusvcouto/cli-tools/cli"
	"golang.org/x/term"
)

// TerminalIO keeps terminal capabilities and native interaction out of the domain.
func TerminalIO(in, out, errOut *os.File) (core.IO, core.Interaction) {
	terminal := core.TerminalFromFiles(in, out, errOut)
	terminal.StdinTTY = term.IsTerminal(int(in.Fd()))
	terminal.StdoutTTY = term.IsTerminal(int(out.Fd()))
	terminal.StderrTTY = term.IsTerminal(int(errOut.Fd()))
	terminal.Width, _, _ = term.GetSize(int(errOut.Fd()))
	if strings.EqualFold(os.Getenv("TERM"), "dumb") {
		terminal.StderrTTY = false
	}
	streams := core.IO{In: in, Out: out, Err: errOut, Terminal: terminal}
	interaction := terminalInteraction{TextInteraction: core.TextInteraction{In: in, Out: errOut, Interactive: terminal.StdinTTY}, in: in, out: errOut, native: nativeTerminalAvailable() && terminal.StdinTTY && terminal.StderrTTY}
	return streams, interaction
}

type terminalInteraction struct {
	core.TextInteraction
	in, out *os.File
	native  bool
}

func (t terminalInteraction) Select(ctx context.Context, message string, choices []string, back bool) (index int, err error) {
	return t.SelectLive(ctx, message, choices, back, nil)
}

func (t terminalInteraction) SelectLive(ctx context.Context, message string, choices []string, back bool, snapshot func() []string) (index int, err error) {
	if !t.native {
		return -1, errTextSelect
	}
	state, err := term.MakeRaw(int(t.in.Fd()))
	if err != nil {
		return -1, fmt.Errorf("preparar terminal: %w", err)
	}
	defer func() { err = errors.Join(err, term.Restore(int(t.in.Fd()), state)) }()
	items := append([]string(nil), choices...)
	if back {
		items = append(items, "Voltar")
	}
	searchItems := append([]string(nil), items...)
	if snapshot != nil {
		for i, item := range searchItems {
			if split := strings.LastIndex(item, " — "); split >= 0 {
				searchItems[i] = item[:split]
			}
		}
	}
	model := selectionModel{choices: searchItems}
	rows := 0
	defer func() { fmt.Fprint(t.out, "\x1b[?25h") }()
	if _, err = fmt.Fprint(t.out, "\x1b[?25l"); err != nil {
		return -1, err
	}
	for {
		if snapshot != nil {
			updated := snapshot()
			if len(updated) == len(choices) {
				copy(items, updated)
			}
		}
		width, height, _ := term.GetSize(int(t.out.Fd()))
		if width <= 0 {
			width = 80
		}
		if width < 20 || height > 0 && height < 6 {
			return -1, errTextSelect
		}
		limit := 8
		if height > 0 {
			limit = max(1, min(limit, height-5))
		}
		visible := model.matches()
		model.selected = min(model.selected, max(0, len(visible)-1))
		start := max(0, model.selected-limit+1)
		lines := []string{fitLine(message, width-1), fitLine("↑/↓ escolher · Enter confirmar · Esc cancelar · digite para buscar", width-1), fitLine("Buscar: "+string(model.query), width-1)}
		for i := start; i < min(len(visible), start+limit); i++ {
			marker := "  "
			if i == model.selected {
				marker = "> "
			}
			lines = append(lines, fitLine(marker+items[visible[i]], width-1))
		}
		if len(visible) == 0 {
			lines = append(lines, fitLine("  Nenhuma opção encontrada", width-1))
		}
		if rows > 0 {
			fmt.Fprintf(t.out, "\x1b[%dA", rows)
		}
		for i := 0; i < max(rows, len(lines)); i++ {
			line := ""
			if i < len(lines) {
				line = lines[i]
			}
			if _, err = fmt.Fprintf(t.out, "\r\x1b[2K%s\r\n", line); err != nil {
				return -1, err
			}
		}
		// Keep a stable number of rows when filtering, so redraw never touches history.
		rows = max(rows, len(lines))
		key, readErr := readTerminalKey(ctx, t.in)
		if errors.Is(readErr, os.ErrDeadlineExceeded) {
			continue
		}
		if readErr != nil {
			return -1, readErr
		}
		switch key {
		case "cancel":
			return -1, errDeclined
		case "interrupt", "eof":
			return -1, context.Canceled
		case "enter":
			if len(visible) == 0 {
				continue
			}
			chosen := visible[model.selected]
			if err := summarizeSelection(t.out, rows, message, items[chosen], width); err != nil {
				return -1, err
			}
			if back && chosen == len(choices) {
				return -1, nil
			}
			return chosen, nil
		case "up":
			model.selected = max(0, model.selected-1)
		case "down":
			model.selected = min(max(0, len(visible)-1), model.selected+1)
		case "left", "right", "home", "end", "delete":
			continue
		case "backspace":
			if len(model.query) > 0 {
				model.query = model.query[:len(model.query)-1]
			}
			model.selected = 0
		default:
			if len(model.query) < 80 {
				model.query = append(model.query, []rune(key)...)
				model.selected = 0
			}
		}
	}
}

// Replace only our active menu rows with one selected-answer line. Completed
// selectors no longer leave search/instruction grids in terminal history.
func summarizeSelection(out *os.File, rows int, message, answer string, width int) error {
	if _, err := fmt.Fprintf(out, "\x1b[%dA", rows); err != nil {
		return err
	}
	for range rows {
		if _, err := fmt.Fprint(out, "\r\x1b[2K\r\n"); err != nil {
			return err
		}
	}
	if _, err := fmt.Fprintf(out, "\x1b[%dA", rows); err != nil {
		return err
	}
	_, err := fmt.Fprintf(out, "\r%s\r\n", fitLine("✓ "+message+": "+answer, width-1))
	return err
}

var errTextSelect = errors.New("use text selection")

type selectionModel struct {
	choices  []string
	query    []rune
	selected int
}

func (m selectionModel) matches() []int {
	var indices []int
	query := strings.ToLower(string(m.query))
	for i, label := range m.choices {
		if strings.Contains(strings.ToLower(label), query) {
			indices = append(indices, i)
		}
	}
	return indices
}

// Conservative cell widths prevent line wraps from corrupting in-place rendering.
// Controls from metadata, paths or user input never become terminal instructions.
func fitLine(s string, width int) string {
	if width <= 0 {
		width = 79
	}
	var b strings.Builder
	cells := 0
	for _, r := range s {
		if unicode.IsControl(r) || unicode.Is(unicode.Cf, r) {
			continue
		}
		size := 1
		if unicode.Is(unicode.Mn, r) {
			size = 0
		} else if r >= 0x1100 && (r <= 0x115f || r >= 0x2e80) {
			size = 2
		}
		if cells+size > width {
			break
		}
		b.WriteRune(r)
		cells += size
	}
	return b.String()
}
