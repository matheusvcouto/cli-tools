// Package mediacli composes media-get's declarative CLI and interactive flow.
package mediacli

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	core "github.com/matheusvcouto/cli-tools/cli"
	"github.com/matheusvcouto/cli-tools/internal/mediaget"
)

const prefix = "mediaget."
const DownloadEnv = "CLI_TOOLS_MEDIA_GET_DOWNLOAD_DIR"
const FragmentsEnv = "CLI_TOOLS_MEDIA_GET_CONCURRENT_FRAGMENTS"

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

func New(service mediaget.Service, product core.ProductMetadata, interaction ...core.Interaction) (*core.CompiledApp, error) {
	var prompts core.Interaction
	if len(interaction) > 0 {
		prompts = interaction[0]
	}
	return core.Compile(core.App{
		Interaction: prompts, ID: "media-get", Name: "media-get", Summary: "download video, audio or subtitles with system yt-dlp", Product: product,
		Builtins: core.Builtins{Help: true, Version: true, Completion: true, Schema: true},
		Root: core.Command{ID: prefix + "root", Name: "media-get", OptionPolicy: core.OptionsInterspersed,
			Args: []core.Arg{{ID: prefix + "url", Name: "url", Summary: "HTTP(S) media URL (prompt when omitted)", Value: core.StringValue(), Sensitive: true}},
			Flags: []core.Flag{
				{ID: prefix + "fragments", Long: "concurrent-fragments", Summary: fmt.Sprintf("parallel native HLS/DASH fragments (1 to %d)", mediaget.MaxConcurrentFragments), Value: fragmentsValue(), Providers: []core.ResolutionProvider{core.EnvProvider(FragmentsEnv)}, Default: []string{strconv.Itoa(mediaget.DefaultConcurrentFragments)}},
				{ID: prefix + "referer", Long: "referer", Summary: "optional origin page; omitted means none", Value: core.StringValue(), Sensitive: true},
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
				{ID: prefix + "subtitle-format", Long: "subtitle-format", Summary: "subtitle output: srt or plain txt", Value: core.EnumValue(core.Choice{Value: "srt"}, core.Choice{Value: "txt"})},
				{ID: prefix + "video-format", Long: "video-format", Summary: "auto container or compatible MP4 (H.264/AAC; may re-encode)", Value: core.EnumValue(core.Choice{Value: "auto"}, core.Choice{Value: "mp4"})},
				{ID: prefix + "lang", Long: "subtitle-lang", Summary: "exact subtitle language from metadata", Value: core.StringValue()},
				{ID: prefix + "auto", Long: "auto-subs", Summary: "choose an automatic subtitle track", Action: core.FlagSwitch},
				{ID: prefix + "name", Long: "name", Summary: "base filename (default: media title)", Value: core.StringValue()},
				{ID: prefix + "yes", Long: "yes", Short: 'y', Summary: "confirm download; requires URL and --kind", Action: core.FlagSwitch},
			},
			Constraints: []core.Constraint{
				{Kind: core.Requires, IDs: []string{prefix + "yes", prefix + "url"}, Message: "--yes exige URL"},
				{Kind: core.Requires, IDs: []string{prefix + "yes", prefix + "kind"}, Message: "--yes exige --kind"},
				{Kind: core.ValuePredicate, PredicateID: prefix + "selection-options", IDs: []string{prefix + "kind", prefix + "quality", prefix + "lang", prefix + "auto", prefix + "subtitle-format", prefix + "video-format"}, Validate: func(values core.ConstraintValues) error {
					kind, _ := core.ConstraintValueAs[string](values, prefix+"kind")
					if values.Present(prefix+"quality") && kind != "video" {
						return errors.New("--quality exige --kind video")
					}
					if values.Present(prefix+"video-format") && kind != "video" {
						return errors.New("--video-format exige --kind video")
					}
					if values.Present(prefix+"subtitle-format") && kind != "subtitle" {
						return errors.New("--subtitle-format exige --kind subtitle")
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
	fragments, _ := core.ValueAs[int](inv, prefix+"fragments")
	src := mediaget.Source{URL: value(inv, "url"), Referer: value(inv, "referer"), ConcurrentFragments: fragments}
	var err error
	if src.URL == "" {
		src.URL, err = ask(inv, "Cole a URL da mídia", "")
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
	var prefetch *estimateSession
	defer func() {
		if prefetch != nil {
			prefetch.close()
		}
	}()
	info, err := inspectInitial(inv, service, &src, automatic, &prefetch)
	if errors.Is(err, errDeclined) {
		fmt.Fprintln(inv.IO.Err, "Cancelado.")
		return nil
	}
	if err != nil {
		return err
	}
	if !automatic {
		fmt.Fprintln(inv.IO.Err, "Mídia:", mediaget.SafeName(info.Title))
	}
	selectedEstimate := transferEstimate{}
	sel := mediaget.Selection{Kind: mediaget.Kind(kind), SubtitleFormat: value(inv, "subtitle-format"), VideoFormat: value(inv, "video-format")}
	if automatic {
		_, sel = mediaget.OutputSelection(value(inv, "name"), sel)
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
		if err := inspectEstimate(inv, service, src, sel, &selectedEstimate); err != nil {
			return err
		}
	} else {
		sel, err = wizard(inv, service, &src, &info, sel, &selectedEstimate, &prefetch, true)
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
	if !automatic && !inv.Present(prefix+"name") {
		name, err = ask(inv, "Nome do arquivo", mediaget.SafeName(name))
		if err != nil {
			return err
		}
	}
	name, sel = mediaget.OutputSelection(name, sel)
	review := downloadReview{Name: name, Dir: dir, Selection: sel, Estimate: selectedEstimate}
	if automatic {
		err = printDownloadSummary(inv, info, src, sel, selectedEstimate, name, dir)
	} else {
		err = reviewDownload(inv, service, &src, &info, &review, &prefetch)
	}
	if errors.Is(err, errDeclined) {
		fmt.Fprintln(inv.IO.Err, "Cancelado.")
		return nil
	}
	if err != nil {
		return err
	}
	name, dir, sel, selectedEstimate = review.Name, review.Dir, review.Selection, review.Estimate
	if prefetch != nil {
		selectedEstimate = prefetch.get(sel).estimate
		prefetch.close()
		prefetch = nil
	}
	renderer := newDownloadRenderer(inv)
	renderer.setEstimate(selectedEstimate)
	result, err := downloadWithProgress(inv, service, mediaget.Request{Source: src, Selection: sel, OutputDir: dir, Name: name, KeepIncomplete: !automatic && inv.Terminal.StdinTTY}, renderer)
	if err != nil {
		if result.Incomplete == nil && errors.Is(err, context.Canceled) {
			if result.CleanupWarning != nil {
				return cancellationOutcome(err, "Cancelado. Limpeza incompleta: "+err.Error())
			}
			return cancellationOutcome(err, "Cancelado. Arquivos incompletos descartados.")
		}
		return finishIncomplete(inv, result.Incomplete, err)
	}
	if result.CleanupWarning != nil {
		fmt.Fprintln(inv.IO.Err, "Download concluído; não foi possível limpar a área de trabalho:", result.CleanupWarning)
	}
	fmt.Fprintln(inv.IO.Err, "Pronto:", mediaget.HumanSize(result.Bytes))
	_, err = fmt.Fprintln(inv.IO.Out, result.Path)
	return err
}

var errDeclined = errors.New("cancelado pelo usuário")

// Returning on cancellation lets the entrypoint exit even while a terminal
// reader is blocked. Only one prompt is active; it cannot read future prompts.
func ask(inv *core.Invocation, message, def string) (string, error) {
	if native, ok := inv.Interaction.(terminalInteraction); ok && native.native {
		answer, err := native.Text(inv.Context, core.Prompt{Message: message, Default: def})
		return strings.TrimSpace(answer), err
	}
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
	if native, ok := inv.Interaction.(interface {
		Select(context.Context, string, []string, bool) (int, error)
	}); ok {
		index, err := native.Select(inv.Context, message, choices, back)
		if !errors.Is(err, errTextSelect) {
			return index, err
		}
	}

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
func wizard(inv *core.Invocation, service mediaget.Service, src *mediaget.Source, info *mediaget.Info, sel mediaget.Selection, selectedEstimate *transferEstimate, prefetch **estimateSession, honorFlags bool) (mediaget.Selection, error) {
	scheduleQualities(inv, *prefetch, *info)
chooseMedia:
	for {
		kinds := []mediaget.Kind{mediaget.Video, mediaget.Audio}
		labels := []string{"Vídeo com áudio (melhor qualidade)", "Somente áudio (M4A)"}
		if len(info.Tracks) > 0 {
			kinds = append(kinds, mediaget.Subtitle)
			labels = append(labels, "Somente legenda (SRT ou TXT)")
		}
		if sel.Kind == "" {
			selections := []mediaget.Selection{{Kind: mediaget.Video}, {Kind: mediaget.Audio}}
			if len(kinds) > 2 {
				selections = append(selections, mediaget.Selection{Kind: mediaget.Subtitle})
			}
			index, err := chooseLive(inv, "O que deseja baixar?", func() []string { return (*prefetch).labels(selections, labels) }, false)
			if err != nil {
				return sel, err
			}
			sel = mediaget.Selection{Kind: kinds[index]}
		}
		switch sel.Kind {
		case mediaget.Video:
			if honorFlags && inv.Present(prefix+"quality") {
				sel.Height, _ = core.ValueAs[int](inv, prefix+"quality")
				goto outputFormat
			}
			heights, options := videoQualities(*info)
			selections := make([]mediaget.Selection, len(heights))
			for i, height := range heights {
				selections[i] = mediaget.Selection{Kind: mediaget.Video, Height: height}
			}
			index, err := chooseLive(inv, "Qualidade do vídeo", func() []string { return (*prefetch).labels(selections, options) }, true)
			if err != nil {
				return sel, err
			}
			if index < 0 {
				sel.Kind = ""
				continue
			}
			sel.Height = heights[index]
		case mediaget.Subtitle:
			if honorFlags && value(inv, "lang") != "" {
				track, err := findTrack(*info, value(inv, "lang"), flag(inv, "auto"))
				if err != nil {
					return sel, err
				}
				sel.Track = track
				goto outputFormat
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
	outputFormat:
		if sel.Kind == mediaget.Subtitle {
			if honorFlags && inv.Present(prefix+"subtitle-format") {
				sel.SubtitleFormat = value(inv, "subtitle-format")
			} else {
				index, err := choose(inv, "Formato da legenda", []string{"SRT — com tempos e numeração", "TXT — somente texto"}, true)
				if err != nil {
					return sel, err
				}
				if index < 0 {
					sel.Kind = ""
					continue
				}
				sel.SubtitleFormat = []string{"srt", "txt"}[index]
			}
		}
		if sel.Kind == mediaget.Video {
			if honorFlags && inv.Present(prefix+"video-format") {
				sel.VideoFormat = value(inv, "video-format")
			} else if honorFlags && strings.EqualFold(filepath.Ext(value(inv, "name")), ".mp4") {
				sel.VideoFormat = "mp4"
			} else {
				index, err := choose(inv, "Formato do vídeo", []string{"Automático — mantém os codecs da fonte", "MP4 compatível — H.264/AAC; pode recodificar e demorar mais"}, true)
				if err != nil {
					return sel, err
				}
				if index < 0 {
					sel.Kind = ""
					continue
				}
				sel.VideoFormat = []string{"auto", "mp4"}[index]
			}
		}
		for {
			if err := service.Check(inv.Context, sel); err != nil {
				return sel, err
			}
			entry := (*prefetch).get(sel)
			if entry.ready {
				printTransferEstimate(inv, entry.estimate)
			} else {
				fmt.Fprintln(inv.IO.Err, "Tamanho sendo calculado em segundo plano; você pode continuar.")
			}
			printDownloadConfiguration(inv, *src, sel.Kind)
			index, err := chooseLive(inv, "Continuar?", func() []string {
				entry := (*prefetch).get(sel)
				size := "calculando…"
				if entry.ready {
					size = entry.estimate.label()
				}
				return []string{"Continuar com estas opções — " + size, "Ajustar opções", "Cancelar", "Adicionar configuração"}
			}, false)
			if err != nil {
				return sel, err
			}
			if index == 0 {
				*selectedEstimate = (*prefetch).get(sel).estimate
				return sel, nil
			}
			if index == 3 {
				changed, err := configure(inv, src)
				if err != nil {
					return sel, err
				}
				if changed {
					(*prefetch).close()
					*info, err = withLoading(inv, "Atualizando mídia", func() (mediaget.Info, error) { return service.Inspect(inv.Context, *src, mediaget.Selection{}) })
					if err != nil {
						return sel, err
					}
					*prefetch = newEarlyEstimateSession(inv.Context, service, *src)
					scheduleQualities(inv, *prefetch, *info)
					sel = mediaget.Selection{}
					continue chooseMedia
				}
				continue
			}
			if index == 2 {
				return sel, errDeclined
			}
			sel.Kind = ""
			continue chooseMedia
		}
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
func inspectEstimate(inv *core.Invocation, service mediaget.Service, src mediaget.Source, sel mediaget.Selection, selectedEstimate *transferEstimate) error {
	estimates := make(map[mediaget.Selection]transferEstimate)
	if err := estimateOptions(inv, service, src, []mediaget.Selection{sel}, estimates); err != nil {
		return err
	}
	*selectedEstimate = estimates[sel]
	return nil
}
