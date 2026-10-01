// Package mediacli composes media-get's declarative CLI and interactive flow.
package mediacli

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	core "github.com/matheusvcouto/cli-tools/cli"
	"github.com/matheusvcouto/cli-tools/internal/mediaget"
)

const prefix = "mediaget."
const DownloadEnv = "CLI_TOOLS_MEDIA_GET_DOWNLOAD_DIR"

func qualityValue() core.Value {
	choices := []core.Choice{{Value: "best"}, {Value: "2160"}, {Value: "1080"}, {Value: "720"}, {Value: "480"}, {Value: "360"}}
	return core.TypedValue(core.CodecFuncs[int]{
		Name: "video_height", ValueHint: core.HintEnum, StaticChoices: choices,
		ParseFunc: func(raw string) (int, error) {
			for _, choice := range choices {
				if raw == choice.Value {
					if raw == "best" {
						return 0, nil
					}
					return strconv.Atoi(raw)
				}
			}
			return 0, errors.New("use best, 2160, 1080, 720, 480 ou 360")
		},
		FormatFunc: func(height int) string {
			if height == 0 {
				return "best"
			}
			return strconv.Itoa(height)
		},
	})
}

func New(service mediaget.Service, product core.ProductMetadata) (*core.CompiledApp, error) {
	return core.Compile(core.App{
		ID: "media-get", Name: "media-get", Summary: "download video, audio or subtitles with system yt-dlp", Product: product,
		Builtins: core.Builtins{Help: true, Version: true, Completion: true, Schema: true},
		Root: core.Command{ID: prefix + "root", Name: "media-get", OptionPolicy: core.OptionsInterspersed,
			Args: []core.Arg{{ID: prefix + "url", Name: "url", Summary: "HTTP(S) media URL (prompt when omitted)", Value: core.StringValue(), Sensitive: true}},
			Flags: []core.Flag{
				{ID: prefix + "referer", Long: "referer", Summary: "optional origin page; empty disables the prompt", Value: core.StringValue(), Sensitive: true},
				{ID: prefix + "output", Long: "output-dir", Short: 'o', Summary: "existing download directory", Value: core.DirectoryValue(), Providers: []core.ResolutionProvider{
					core.EnvProvider(DownloadEnv),
					{Source: core.SourceProvider, Name: "Downloads", Resolve: func(context.Context) ([]string, bool, error) {
						home, err := os.UserHomeDir()
						if err != nil {
							return nil, false, err
						}
						return []string{mediaget.DefaultDownloadDir(home)}, true, nil
					}},
				}},
				{ID: prefix + "kind", Long: "kind", Summary: "video with audio, audio only, or subtitle", Value: core.EnumValue(core.Choice{Value: "video"}, core.Choice{Value: "audio"}, core.Choice{Value: "subtitle"})},
				{ID: prefix + "quality", Long: "quality", Summary: "video height limit; best has no height limit", Value: qualityValue()},
				{ID: prefix + "lang", Long: "subtitle-lang", Summary: "exact subtitle language from metadata", Value: core.StringValue()},
				{ID: prefix + "auto", Long: "auto-subs", Summary: "choose an automatic subtitle track", Action: core.FlagSwitch},
				{ID: prefix + "name", Long: "name", Summary: "base filename (default: media title)", Value: core.StringValue()},
				{ID: prefix + "yes", Long: "yes", Short: 'y', Summary: "confirm download; requires URL and --kind", Action: core.FlagSwitch},
			},
			Constraints: []core.Constraint{
				{Kind: core.Requires, IDs: []string{prefix + "yes", prefix + "url"}, Message: "--yes exige URL"},
				{Kind: core.Requires, IDs: []string{prefix + "yes", prefix + "kind"}, Message: "--yes exige --kind"},
				{Kind: core.ValuePredicate, PredicateID: prefix + "selection-options", IDs: []string{prefix + "kind", prefix + "quality", prefix + "lang", prefix + "auto"}, Validate: func(values core.ConstraintValues) error {
					kind, _ := core.ConstraintValueAs[string](values, prefix+"kind")
					if values.Present(prefix+"quality") && kind != "video" {
						return errors.New("--quality exige --kind video")
					}
					if (values.Present(prefix+"lang") || values.Present(prefix+"auto")) && kind != "subtitle" {
						return errors.New("--subtitle-lang e --auto-subs exigem --kind subtitle")
					}
					return nil
				}},
			}, Handler: func(inv *core.Invocation) error { return run(inv, service) },
		},
	})
}
func value(inv *core.Invocation, name string) string {
	s, _ := core.ValueAs[string](inv, prefix+name)
	return s
}
func flag(inv *core.Invocation, name string) bool {
	b, _ := core.ValueAs[bool](inv, prefix+name)
	return b
}
func usage(message string) error {
	return &core.Diagnostic{Code: core.CodeConstraint, Kind: "usage", Message: message, Class: core.ExitUsage}
}

func run(inv *core.Invocation, service mediaget.Service) error {
	automatic := flag(inv, "yes")
	if automatic && (value(inv, "url") == "" || value(inv, "kind") == "") {
		return usage("--yes exige URL e --kind")
	}
	if !automatic && !inv.Terminal.StdinTTY {
		return &core.Diagnostic{Code: core.CodeNonInteractive, Kind: "interaction", Message: "use um terminal ou informe URL, --kind e --yes", Class: core.ExitUnavailable}
	}
	kind := value(inv, "kind")
	src := mediaget.Source{URL: value(inv, "url"), Referer: value(inv, "referer")}
	var err error
	if src.URL == "" {
		src.URL, err = ask(inv, "Cole a URL da mídia", "")
		if err != nil {
			return err
		}
	}
	if !automatic && !inv.Present(prefix+"referer") {
		src.Referer, err = ask(inv, "Referer (página de origem; Enter para nenhum)", "")
		if err != nil {
			return err
		}
	}
	if err := mediaget.ValidateSource(src); err != nil {
		return usage(err.Error())
	}
	dir := value(inv, "output")
	if strings.TrimSpace(dir) == "" {
		return usage("diretório de download vazio; ajuste --output-dir ou " + DownloadEnv)
	}
	// Reject invalid destinations before any network access or download.
	if err := service.ValidateDestination(dir); err != nil {
		return err
	}
	info, err := withLoading(inv, "Consultando mídia", func() (mediaget.Info, error) {
		return service.Inspect(inv.Context, src, mediaget.Selection{})
	})
	if err != nil {
		return err
	}
	fmt.Fprintln(inv.IO.Err, "Mídia:", mediaget.SafeName(info.Title))
	sel := mediaget.Selection{Kind: mediaget.Kind(kind)}
	if automatic {
		if sel.Kind == mediaget.Video {
			sel.Height, _ = core.ValueAs[int](inv, prefix+"quality")
		}
		if sel.Kind == mediaget.Subtitle {
			track, err := findTrack(info, value(inv, "lang"), flag(inv, "auto"))
			if err != nil {
				return err
			}
			sel.Track = track
		}
		if err := service.Check(inv.Context, sel); err != nil {
			return err
		}
		if err := printEstimate(inv, service, src, sel); err != nil {
			return err
		}
	} else {
		sel, err = wizard(inv, service, src, info, sel)
		if errors.Is(err, errDeclined) {
			fmt.Fprintln(inv.IO.Err, "Cancelado.")
			return nil
		}
		if err != nil {
			return err
		}
	}
	name := value(inv, "name")
	if name == "" {
		name = info.Title
	}
	if !automatic {
		name, err = ask(inv, "Nome do arquivo", mediaget.SafeName(name))
		if err != nil {
			return err
		}
		answer, err := ask(inv, "Baixar em "+dir+"? (s/N)", "n")
		if err != nil {
			return err
		}
		if !isYes(answer) {
			fmt.Fprintln(inv.IO.Err, "Cancelado.")
			return nil
		}
	}
	var lastUpdate time.Time
	lastPercent := -1
	var lastStage mediaget.ProgressStage
	result, err := service.Download(inv.Context, mediaget.Request{Source: src, Selection: sel, OutputDir: dir, Name: name}, func(p mediaget.Progress) {
		if p.Stage != "" && p.Stage != lastStage {
			lastStage = p.Stage
			lastUpdate = time.Time{}
			lastPercent = -1
			message := "Baixando mídia..."
			switch p.Stage {
			case mediaget.Processing:
				message = "Processando mídia (conversão/mesclagem)..."
			case mediaget.Publishing:
				message = "Verificando e salvando arquivo..."
			}
			if inv.Terminal.StderrTTY {
				fmt.Fprint(inv.IO.Err, "\r\x1b[2K")
			}
			fmt.Fprintln(inv.IO.Err, message)
		}
		if p.Stage == mediaget.Processing || p.Stage == mediaget.Publishing || p.Downloaded == 0 && p.Total == 0 {
			return
		}
		percent := -1
		if p.Total > 0 {
			percent = int(min(100.0, float64(p.Downloaded)*100/float64(p.Total)))
		}
		if !lastUpdate.IsZero() && time.Since(lastUpdate) < time.Second && percent != 100 {
			return
		}
		if !inv.Terminal.StderrTTY && percent >= 0 && percent/10 == lastPercent/10 && time.Since(lastUpdate) < 5*time.Second {
			return
		}
		lastUpdate = time.Now()
		lastPercent = percent
		message := "Transferência atual: " + mediaget.HumanSize(p.Downloaded)
		if p.Total > 0 {
			message = fmt.Sprintf("Transferência atual: %d%% (%s / %s)", percent, mediaget.HumanSize(p.Downloaded), mediaget.HumanSize(p.Total))
		}
		if p.Speed > 0 {
			message += " | " + mediaget.HumanSize(int64(p.Speed)) + "/s"
		}
		if p.ETA > 0 {
			message += " | restante: " + (time.Duration(p.ETA) * time.Second).String()
		}
		if inv.Terminal.StderrTTY {
			fmt.Fprintf(inv.IO.Err, "\r\x1b[2K%s", message)
		} else {
			fmt.Fprintln(inv.IO.Err, message)
		}
	})
	if inv.Terminal.StderrTTY && !lastUpdate.IsZero() {
		fmt.Fprintln(inv.IO.Err)
	}
	if err != nil {
		return err
	}
	if result.CleanupWarning != nil {
		fmt.Fprintln(inv.IO.Err, "Download concluído; não foi possível limpar a área de trabalho:", result.CleanupWarning)
	}
	fmt.Fprintln(inv.IO.Err, "Pronto:", mediaget.HumanSize(result.Bytes))
	_, err = fmt.Fprintln(inv.IO.Out, result.Path)
	return err
}

var errDeclined = errors.New("cancelado pelo usuário")

func isYes(s string) bool {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "s", "sim", "y", "yes":
		return true
	}
	return false
}

// Returning on cancellation lets the entrypoint exit even while a terminal
// reader is blocked. Only one prompt is active; it cannot read future prompts.
func ask(inv *core.Invocation, message, def string) (string, error) {
	type answer struct {
		s   string
		err error
	}
	ch := make(chan answer, 1)
	go func() {
		s, err := inv.Interaction.Text(inv.Context, core.Prompt{Message: message, Default: def})
		ch <- answer{s, err}
	}()
	select {
	case <-inv.Context.Done():
		return "", inv.Context.Err()
	case a := <-ch:
		return strings.TrimSpace(a.s), a.err
	}
}
func choose(inv *core.Invocation, message string, choices []string, back bool) (int, error) {
	for i, label := range choices {
		fmt.Fprintf(inv.IO.Err, "  %d. %s\n", i+1, label)
	}
	if back {
		fmt.Fprintln(inv.IO.Err, "  0. Voltar")
	}
	for {
		answer, err := ask(inv, message+" (q para cancelar)", "1")
		if err != nil {
			return -1, err
		}
		if strings.EqualFold(answer, "q") || strings.EqualFold(answer, "cancelar") {
			return -1, errDeclined
		}
		n, err := strconv.Atoi(answer)
		if err == nil && n >= 1 && n <= len(choices) {
			return n - 1, nil
		}
		if back && answer == "0" {
			return -1, nil
		}
		fmt.Fprintln(inv.IO.Err, "Escolha um número da lista.")
	}
}
func wizard(inv *core.Invocation, service mediaget.Service, src mediaget.Source, info mediaget.Info, sel mediaget.Selection) (mediaget.Selection, error) {
	kinds := []mediaget.Kind{mediaget.Video, mediaget.Audio}
	labels := []string{"Vídeo com áudio (melhor qualidade)", "Somente áudio (M4A)"}
	estimates := make(map[mediaget.Selection]transferEstimate)
	if len(info.Tracks) > 0 {
		kinds = append(kinds, mediaget.Subtitle)
		labels = append(labels, "Somente legenda (SRT)")
	}
	for {
		if sel.Kind == "" {
			selections := []mediaget.Selection{{Kind: mediaget.Video}, {Kind: mediaget.Audio}}
			if err := estimateOptions(inv, service, src, selections, estimates); err != nil {
				return sel, err
			}
			options := append([]string(nil), labels...)
			for i, kind := range kinds {
				options[i] += " — " + estimates[mediaget.Selection{Kind: kind}].label()
			}
			index, err := choose(inv, "O que deseja baixar?", options, false)
			if err != nil {
				return sel, err
			}
			sel.Kind = kinds[index]
		}
		switch sel.Kind {
		case mediaget.Video:
			if inv.Present(prefix + "quality") {
				sel.Height, _ = core.ValueAs[int](inv, prefix+"quality")
				break
			}
			heights := []int{0}
			options := []string{"Melhor qualidade disponível"}
			for _, height := range []int{2160, 1080, 720, 480, 360} {
				available := len(info.Heights) == 0
				for _, h := range info.Heights {
					if h <= height {
						available = true
						break
					}
				}
				if available {
					heights = append(heights, height)
					options = append(options, fmt.Sprintf("Até %dp", height))
				}
			}
			selections := make([]mediaget.Selection, len(heights))
			for i, height := range heights {
				selections[i] = mediaget.Selection{Kind: mediaget.Video, Height: height}
			}
			if err := estimateOptions(inv, service, src, selections, estimates); err != nil {
				return sel, err
			}
			for i, selection := range selections {
				options[i] += " — " + estimates[selection].label()
			}
			index, err := choose(inv, "Qualidade do vídeo", options, true)
			if err != nil {
				return sel, err
			}
			if index < 0 {
				sel.Kind = ""
				continue
			}
			sel.Height = heights[index]
		case mediaget.Subtitle:
			if value(inv, "lang") != "" {
				track, err := findTrack(info, value(inv, "lang"), flag(inv, "auto"))
				if err != nil {
					return sel, err
				}
				sel.Track = track
				break
			}
			if len(info.Tracks) == 0 {
				return sel, usage("esta mídia não oferece faixas de legenda")
			}
			var labels []string
			for _, track := range info.Tracks {
				label := mediaget.SafeName(track.Name) + " (" + mediaget.SafeName(track.Lang) + ")"
				if track.Auto {
					label += " — automática"
				}
				labels = append(labels, label+" — tamanho indisponível")
			}
			index, err := choose(inv, "Escolha a legenda", labels, true)
			if err != nil {
				return sel, err
			}
			if index < 0 {
				sel.Kind = ""
				continue
			}
			sel.Track = info.Tracks[index]
		}
		if err := service.Check(inv.Context, sel); err != nil {
			return sel, err
		}
		if err := estimateOptions(inv, service, src, []mediaget.Selection{sel}, estimates); err != nil {
			return sel, err
		}
		printTransferEstimate(inv, estimates[sel])
		index, err := choose(inv, "Continuar?", []string{"Continuar com estas opções", "Ajustar opções", "Cancelar"}, false)
		if err != nil {
			return sel, err
		}
		if index == 0 {
			return sel, nil
		}
		if index == 2 {
			return sel, errDeclined
		}
		sel.Kind = ""
	}
}
func findTrack(info mediaget.Info, lang string, auto bool) (mediaget.Track, error) {
	if lang == "" {
		return mediaget.Track{}, usage("--kind subtitle com --yes exige --subtitle-lang")
	}
	for _, track := range info.Tracks {
		if track.Lang == lang && track.Auto == auto {
			return track, nil
		}
	}
	return mediaget.Track{}, usage("faixa de legenda indisponível; confira o idioma e --auto-subs")
}
func printEstimate(inv *core.Invocation, service mediaget.Service, src mediaget.Source, sel mediaget.Selection) error {
	estimates := make(map[mediaget.Selection]transferEstimate)
	if err := estimateOptions(inv, service, src, []mediaget.Selection{sel}, estimates); err != nil {
		return err
	}
	printTransferEstimate(inv, estimates[sel])
	return nil
}
