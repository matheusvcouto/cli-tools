package mediacli

import (
	"fmt"
	"io"
	"strings"

	core "github.com/matheusvcouto/cli-tools/cli"
	"github.com/matheusvcouto/cli-tools/internal/mediaget"
)

// Keep the review on stderr; stdout remains the published path. Source URLs
// and Referer values may contain credentials and never belong in this summary.
func printDownloadSummary(inv *core.Invocation, info mediaget.Info, src mediaget.Source, sel mediaget.Selection, estimate transferEstimate, name, dir string) error {
	rows := []string{"Mídia: " + mediaget.SafeName(info.Title)}
	switch sel.Kind {
	case mediaget.Video:
		quality := "Melhor disponível"
		if sel.Height > 0 {
			quality = fmt.Sprintf("Até %dp", sel.Height)
		}
		rows = append(rows, "Tipo: Vídeo com áudio", "Qualidade: "+quality)
	case mediaget.Audio:
		rows = append(rows, "Tipo: Áudio M4A")
	case mediaget.Subtitle:
		origin := "manual"
		if sel.Track.Auto {
			origin = "automática"
		}
		format := "SRT"
		if sel.SubtitleFormat == "txt" {
			format = "TXT — somente texto"
		}
		rows = append(rows, "Tipo: Legenda "+format, "Idioma: "+sel.Track.Lang+" ("+origin+")")
	}
	if sel.Kind != mediaget.Subtitle {
		size := estimate.label() + " (o arquivo final pode variar)"
		if !estimate.known {
			size = "Não foi possível calcular o tamanho"
		}
		rows = append(rows, "Transferência: "+size, fmt.Sprintf("Fragmentos paralelos: %d (HLS/DASH)", src.FragmentConcurrency()))
	}
	rows = append(rows, "Nome base: "+mediaget.SafeName(name), "Destino: "+dir)
	format, extension := outputFormat(sel)
	rows = append(rows, "Formato de saída: "+format)
	if extension != "" {
		rows = append(rows, "Arquivo previsto: "+mediaget.SafeName(name)+extension+" (colisões recebem sufixo)")
	}
	if src.Referer != "" {
		rows = append(rows, "Referer: definido")
	}
	if src.Origin != "" {
		rows = append(rows, "Origin: definido")
	}
	var b strings.Builder
	b.WriteByte('\n')
	width := inv.Terminal.Width
	if width <= 0 {
		width = 80
	}
	boxed := inv.Terminal.StderrTTY && width >= 24
	if boxed {
		outer := min(80, width-1)
		inner := outer - 4
		border := strings.Repeat("─", outer-2)
		b.WriteString("┌" + border + "┐\n")
		for _, row := range append([]string{"Resumo do download", ""}, rows...) {
			line := fitLine(row, inner)
			b.WriteString("│ " + line + strings.Repeat(" ", inner-displayCells(line)) + " │\n")
		}
		b.WriteString("└" + border + "┘\n")
	} else {
		b.WriteString(fitLine("Resumo do download", summaryWidth(inv)) + "\n")
		for _, row := range rows {
			b.WriteString(fitLine(row, summaryWidth(inv)) + "\n")
		}
	}
	output := b.String()
	n, err := io.WriteString(inv.IO.Err, output)
	if err == nil && n != len(output) {
		return io.ErrShortWrite
	}
	return err
}

func outputFormat(sel mediaget.Selection) (string, string) {
	switch sel.Kind {
	case mediaget.Audio:
		return "M4A (.m4a)", ".m4a"
	case mediaget.Subtitle:
		if sel.SubtitleFormat == "txt" {
			return "TXT (.txt)", ".txt"
		}
		return "SRT (.srt)", ".srt"
	case mediaget.Video:
		if sel.VideoFormat == "mp4" {
			return "MP4 (.mp4) — H.264/AAC; pode recodificar", ".mp4"
		}
	}
	return "Automático — extensão a confirmar após processamento", ""
}

func summaryWidth(inv *core.Invocation) int {
	if inv.Terminal.StderrTTY && inv.Terminal.Width > 1 {
		return inv.Terminal.Width - 1
	}
	return int(^uint(0) >> 1)
}
