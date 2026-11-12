package notioncli

import (
	"fmt"
	"strings"
	"unicode"

	core "github.com/matheusvcouto/cli-tools/cli"
	ne "github.com/matheusvcouto/cli-tools/internal/notionexport"
)

// FormatBytes renders IEC binary sizes with one decimal ("14.8 MiB"), the
// same units media-get uses.
func FormatBytes(n int64) string {
	if n < 0 {
		return "desconhecido"
	}
	units := []string{"B", "KiB", "MiB", "GiB", "TiB"}
	value := float64(n)
	unit := 0
	for value >= 1024 && unit < len(units)-1 {
		value /= 1024
		unit++
	}
	if unit == 0 {
		return fmt.Sprintf("%d B", n)
	}
	return fmt.Sprintf("%.1f %s", value, units[unit])
}

// DescribeGroupSize explains known/unknown totals of one asset group.
func DescribeGroupSize(g ne.GroupSummary) string {
	switch {
	case g.Count == 0:
		return "0 B"
	case g.UnknownCount == 0:
		return FormatBytes(g.KnownBytes)
	case g.KnownBytes == 0 && g.UnknownCount == g.Count:
		return "tamanho total desconhecido"
	}
	return fmt.Sprintf("pelo menos %s; %d com tamanho desconhecido", FormatBytes(g.KnownBytes), g.UnknownCount)
}

var groupLabels = map[ne.AssetType]string{ne.AssetImage: "Imagens", ne.AssetVideo: "Vídeos", ne.AssetAudio: "Áudios", ne.AssetPDF: "PDFs", ne.AssetFile: "Arquivos"}

func printAssetSummary(inv *core.Invocation, assets []ne.Asset) {
	out := inv.IO.Err
	fmt.Fprintln(out, "\nAssets encontrados:")
	if len(assets) == 0 {
		fmt.Fprintln(out, "  Nenhum asset para baixar.")
		return
	}
	summary := ne.Summarize(assets)
	for _, t := range ne.AssetTypes {
		if g := summary[t]; g.Count > 0 {
			fmt.Fprintf(out, "  %s: %d (%s)\n", groupLabels[t], g.Count, DescribeGroupSize(g))
		}
	}
}

// chooseDownloads applies ask/yes/no per type. Explicit per-type flags win
// over --yes. On non-interactive stdin "ask" means no, with one warning.
func chooseDownloads(inv *core.Invocation, assets []ne.Asset) (map[string]bool, error) {
	summary := ne.Summarize(assets)
	types := map[ne.AssetType]bool{}
	warned := false
	for _, p := range policies {
		g := summary[p.kind]
		if g.Count == 0 {
			continue
		}
		mode := "ask"
		if boolValue(inv, flagYes) {
			mode = "yes"
		}
		if inv.Present(p.flag) {
			mode = stringValue(inv, p.flag)
		}
		switch mode {
		case "yes":
			types[p.kind] = true
		case "no":
		default:
			if inv.Interaction == nil || !inv.Terminal.StdinTTY {
				if !warned {
					warned = true
					fmt.Fprintln(inv.IO.Err, "Entrada não interativa: perguntas \"ask\" serão tratadas como não. Use --yes ou --images/--videos/--audio/--pdfs/--attachments yes.")
				}
				continue
			}
			noun := p.plural
			if g.Count == 1 {
				noun = p.singular
			}
			question := fmt.Sprintf("%s %d %s (%s). %s [s/N]", p.verb, g.Count, noun, DescribeGroupSize(g), p.question)
			answer, err := ask(inv, question)
			if err != nil {
				return nil, err
			}
			types[p.kind] = isYes(answer)
		}
	}
	return ne.SelectAll(assets, types), nil
}

func isYes(answer string) bool {
	switch strings.ToLower(strings.TrimSpace(answer)) {
	case "s", "sim", "y", "yes":
		return true
	}
	return false
}

// ask returns on cancellation even while the line reader is blocked; only one
// prompt is active at a time, so the abandoned read cannot steal later input.
func ask(inv *core.Invocation, message string) (string, error) {
	type answer struct {
		s   string
		err error
	}
	ch := make(chan answer, 1)
	go func() {
		s, err := inv.Interaction.Text(inv.Context, core.Prompt{Message: message})
		ch <- answer{s, err}
	}()
	select {
	case <-inv.Context.Done():
		return "", inv.Context.Err()
	case a := <-ch:
		return a.s, a.err
	}
}

// sanitize keeps remote titles, names and errors from injecting terminal
// controls or bidi/zero-width format characters that disguise text.
func sanitize(s string) string {
	return strings.Map(func(r rune) rune {
		if unicode.IsControl(r) || unicode.Is(unicode.Cf, r) {
			return -1
		}
		return r
	}, s)
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}
