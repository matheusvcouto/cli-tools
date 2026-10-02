package mediacli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"unicode"

	core "github.com/matheusvcouto/cli-tools/cli"
	"golang.org/x/term"
)

type textModel struct {
	text   []rune
	cursor int
}

func (m *textModel) apply(key string) {
	switch key {
	case "left":
		m.cursor = max(0, m.cursor-1)
	case "right":
		m.cursor = min(len(m.text), m.cursor+1)
	case "home":
		m.cursor = 0
	case "end":
		m.cursor = len(m.text)
	case "backspace":
		if m.cursor > 0 {
			m.text = append(m.text[:m.cursor-1], m.text[m.cursor:]...)
			m.cursor--
		}
	case "delete":
		if m.cursor < len(m.text) {
			m.text = append(m.text[:m.cursor], m.text[m.cursor+1:]...)
		}
	case "up", "down", "":
	default:
		// Decoder returns one printable rune; named keys never become field content.
		r := []rune(key)
		if len(r) == 1 && !unicode.IsControl(r[0]) && !unicode.Is(unicode.Cf, r[0]) && len(m.text) < 8192 {
			m.text = append(m.text, 0)
			copy(m.text[m.cursor+1:], m.text[m.cursor:])
			m.text[m.cursor] = r[0]
			m.cursor++
		}
	}
}

func (t terminalInteraction) Text(ctx context.Context, prompt core.Prompt) (answer string, err error) {
	if !t.native {
		return t.TextInteraction.Text(ctx, prompt)
	}
	state, err := term.MakeRaw(int(t.in.Fd()))
	if err != nil {
		return "", fmt.Errorf("preparar campo de texto: %w", err)
	}
	defer func() { err = errors.Join(err, term.Restore(int(t.in.Fd()), state)) }()
	defer fmt.Fprint(t.out, "\r\n")
	label := prompt.Message
	if prompt.Default != "" {
		label += " [" + prompt.Default + "]"
	}
	label += ": "
	model := textModel{}
	for {
		width, _, _ := term.GetSize(int(t.out.Fd()))
		if width <= 0 {
			width = 80
		}
		// One physical line with a viewport around the insertion point. External
		// text is sanitized by fitLine; cursor escapes are generated only here.
		prefix := fitLine(label, max(1, width/2))
		available := max(1, width-1-displayCells(prefix))
		start := model.cursor
		used := 0
		for start > 0 {
			n := displayCells(string(model.text[start-1]))
			if used+n > available/2 {
				break
			}
			used += n
			start--
		}
		text := fitLine(string(model.text[start:]), available)
		left := displayCells(prefix) + displayCells(string(model.text[start:model.cursor]))
		if _, err = fmt.Fprintf(t.out, "\r\x1b[2K%s%s\r\x1b[%dC", prefix, text, left); err != nil {
			return "", err
		}
		key, e := readTerminalKey(ctx, t.in)
		if errors.Is(e, os.ErrDeadlineExceeded) {
			continue
		}
		if e != nil {
			return "", e
		}
		switch key {
		case "enter":
			answer = strings.TrimSpace(string(model.text))
			if answer == "" {
				answer = prompt.Default
			}
			return answer, nil
		case "interrupt", "cancel":
			return "", context.Canceled
		case "eof":
			return "", io.EOF
		default:
			model.apply(key)
		}
	}
}

func displayCells(s string) int {
	cells := 0
	for _, r := range s {
		if unicode.IsControl(r) || unicode.Is(unicode.Cf, r) || unicode.Is(unicode.Mn, r) {
			continue
		}
		if r >= 0x1100 && (r <= 0x115f || r >= 0x2e80) {
			cells += 2
		} else {
			cells++
		}
	}
	return cells
}
