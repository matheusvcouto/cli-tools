// Package notioncli composes notion-export's declarative CLI and its
// interactive asset decisions. Domain rules stay in internal/notionexport.
package notioncli

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	core "github.com/matheusvcouto/cli-tools/cli"
	ne "github.com/matheusvcouto/cli-tools/internal/notionexport"
	"github.com/matheusvcouto/cli-tools/internal/notionexport/notionapi"
)

const prefix = "notionexport."

const (
	argSource       = prefix + "source"
	flagOut         = prefix + "out"
	flagFromJSON    = prefix + "from-json"
	flagFormat      = prefix + "format"
	flagYes         = prefix + "yes"
	flagImages      = prefix + "images"
	flagVideos      = prefix + "videos"
	flagAudio       = prefix + "audio"
	flagPDFs        = prefix + "pdfs"
	flagAttachments = prefix + "attachments"
	flagConcurrency = prefix + "concurrency"
	flagNoProbe     = prefix + "no-size-probe"
	flagForce       = prefix + "force"
	flagName        = prefix + "name"
)

// Dependencies are created by the composition root and used only by the
// export handler, so help/version/schema/completion stay side-effect free.
type Dependencies struct {
	HTTP    *http.Client
	BaseURL string
	Retry   ne.RetryPolicy
	Now     func() time.Time
}

// assetPolicy is one per-type ask/yes/no decision in prompt order.
type assetPolicy struct {
	flag     string
	long     string
	kind     ne.AssetType
	singular string
	plural   string
	verb     string
	question string
}

var policies = []assetPolicy{
	{flagImages, "images", ne.AssetImage, "imagem", "imagens", "Encontradas", "Baixar imagens?"},
	{flagVideos, "videos", ne.AssetVideo, "vídeo", "vídeos", "Encontrados", "Baixar vídeos?"},
	{flagAudio, "audio", ne.AssetAudio, "áudio", "áudios", "Encontrados", "Baixar áudios?"},
	{flagPDFs, "pdfs", ne.AssetPDF, "PDF", "PDFs", "Encontrados", "Baixar PDFs?"},
	{flagAttachments, "attachments", ne.AssetFile, "arquivo", "arquivos", "Encontrados", "Baixar arquivos?"},
}

func modeValue() core.Value {
	return core.EnumValue(core.Choice{Value: "ask", Description: "perguntar quando houver"}, core.Choice{Value: "yes", Description: "baixar"}, core.Choice{Value: "no", Description: "manter remoto"})
}

func concurrencyValue() core.Value {
	return core.TypedValue(core.CodecFuncs[int]{
		Name: "count",
		ParseFunc: func(raw string) (int, error) {
			n, err := strconv.Atoi(raw)
			if err != nil || n < ne.MinConcurrency || n > ne.MaxConcurrency {
				return 0, fmt.Errorf("use um inteiro entre %d e %d", ne.MinConcurrency, ne.MaxConcurrency)
			}
			return n, nil
		},
		FormatFunc: strconv.Itoa,
	})
}

// New compiles the notion-export CLI specification.
func New(deps Dependencies, product core.ProductMetadata, interaction core.Interaction) (*core.CompiledApp, error) {
	flags := []core.Flag{
		{ID: flagOut, Long: "out", Short: 'o', Summary: "base output directory (created when needed)", Value: core.DirectoryValue(), Default: []string{"exports"}},
		{ID: flagFromJSON, Long: "from-json", Summary: "reprocess a saved page.json recordMap", Value: core.FileValue()},
		{ID: flagFormat, Long: "format", Summary: "outputs to render", Value: core.EnumValue(core.Choice{Value: "both", Description: "Markdown and HTML"}, core.Choice{Value: "md", Description: "Markdown only"}, core.Choice{Value: "html", Description: "HTML only"}), Default: []string{"both"}},
		{ID: flagYes, Long: "yes", Short: 'y', Summary: "download every asset type without prompting (per-type flags still win)", Action: core.FlagSwitch},
	}
	for _, p := range policies {
		flags = append(flags, core.Flag{ID: p.flag, Long: p.long, Summary: "ask, yes or no for " + p.long + " (default: ask)", Value: modeValue()})
	}
	flags = append(flags,
		core.Flag{ID: flagConcurrency, Long: "concurrency", Summary: fmt.Sprintf("parallel requests/downloads (%d-%d)", ne.MinConcurrency, ne.MaxConcurrency), Value: concurrencyValue(), Default: []string{strconv.Itoa(ne.DefaultConcurrency)}},
		core.Flag{ID: flagNoProbe, Long: "no-size-probe", Summary: "skip the 1-byte range requests that estimate asset sizes", Action: core.FlagSwitch},
		core.Flag{ID: flagName, Long: "name", Short: 'n', Summary: "export directory name inside --out (default: slug of the page title)", Value: core.StringValue()},
		core.Flag{ID: flagForce, Long: "force", Short: 'f', Summary: "replace a previous export of the same page (validated by its manifest)", Action: core.FlagSwitch},
	)
	app := core.App{
		ID: "notion-export", Name: "notion-export", Summary: "export a public Notion page to Markdown/HTML with local assets",
		Description: "Uses Notion's unofficial internal web API for pages shared to the web; no token is sent. Not affiliated with Notion Labs, Inc.",
		Product:     product, Interaction: interaction,
		Builtins: core.Builtins{Help: true, Version: true, Completion: true, Schema: true},
		Root: core.Command{
			ID: prefix + "root", Name: "notion-export", OptionPolicy: core.OptionsInterspersed,
			Examples: []string{
				"notion-export https://example.notion.site/Page-0123456789abcdef0123456789abcdef",
				"notion-export 0123456789abcdef0123456789abcdef --yes --format html",
				"notion-export --from-json exports/page/page.json --out rebuilt",
			},
			Args:  []core.Arg{{ID: argSource, Name: "source", Summary: "public page URL or page ID", Value: core.StringValue()}},
			Flags: flags,
			Constraints: []core.Constraint{
				{Kind: core.ExactlyOne, IDs: []string{argSource, flagFromJSON}, Message: "informe uma URL/page ID pública ou --from-json <page.json>, não os dois"},
			},
			Handler: func(inv *core.Invocation) error { return run(inv, deps, product) },
		},
	}
	return core.Compile(app)
}

func stringValue(inv *core.Invocation, id string) string {
	s, _ := core.ValueAs[string](inv, id)
	return s
}

func boolValue(inv *core.Invocation, id string) bool {
	b, _ := core.ValueAs[bool](inv, id)
	return b
}

func usage(message string, cause error) error {
	return &core.Diagnostic{Code: core.CodeConstraint, Kind: "usage", Message: message, Class: core.ExitUsage, Cause: cause}
}

func run(inv *core.Invocation, deps Dependencies, product core.ProductMetadata) error {
	ctx, out := inv.Context, inv.IO.Err
	// Every status line may carry remote text (titles, errors); strip controls.
	say := func(format string, args ...any) { fmt.Fprintln(out, sanitize(fmt.Sprintf(format, args...))) }
	client := deps.HTTP
	if client == nil {
		client = ne.NewHTTPClient()
	}
	retry := deps.Retry
	if retry.Sleep == nil && retry.MaxRetries == 0 && retry.BaseDelay == 0 {
		retry = ne.DefaultRetryPolicy()
	}
	retry.Notify = func(wait time.Duration, err error) {
		say("Aviso: %v; nova tentativa em %s.", err, wait.Round(time.Second))
	}
	api := notionapi.Client{HTTP: client, BaseURL: deps.BaseURL, Retry: retry,
		Progress: func(s string) { say("%s", s) },
		Warn:     func(s string) { say("Aviso: %s", s) }}
	downloadRetry := retry
	downloadRetry.MaxRetries = min(retry.MaxRetries, 2)
	service := ne.Service{Source: api, HTTP: client, Retry: downloadRetry, Now: deps.Now,
		Generator: ne.Generator{Name: ne.GeneratorName, Version: product.Version},
		Progress:  func(s string) { say("%s", s) }}

	format := ne.Format(stringValue(inv, flagFormat))
	outDir := stringValue(inv, flagOut)
	if strings.TrimSpace(outDir) == "" {
		return usage("--out não pode ser vazio", nil)
	}
	concurrency, _ := core.ValueAs[int](inv, flagConcurrency)
	force := boolValue(inv, flagForce)
	fromJSON := stringValue(inv, flagFromJSON)
	name := stringValue(inv, flagName)
	if inv.Present(flagName) {
		if err := ne.ValidateExportName(name); err != nil {
			return usage(err.Error(), err)
		}
	}

	var page *ne.Page
	var err error
	if fromJSON != "" {
		say("Lendo recordMap local: %s", fromJSON)
		page, err = service.LoadRecordMap(fromJSON)
	} else {
		source := stringValue(inv, argSource)
		if _, perr := ne.ParsePageID(source); perr != nil {
			return usage(perr.Error(), perr)
		}
		say("Buscando página pública: %s", source)
		page, err = service.Fetch(ctx, source)
	}
	if err != nil {
		return failure(ctx, "não foi possível carregar a página", err)
	}
	say("Título: %s", page.Title)
	if name != "" {
		page.Slug = name
	}
	if err := service.CheckDestination(outDir, page, force); err != nil {
		return destinationFailure(err)
	}

	assets := page.Assets
	if len(assets) > 0 && !boolValue(inv, flagNoProbe) {
		say("Estimando tamanho de %d asset(s)...", len(assets))
		assets = ne.ProbeSizes(ctx, client, assets, concurrency, nil)
		if err := ctx.Err(); err != nil {
			return err
		}
		page.Assets = assets
	}
	printAssetSummary(inv, assets)
	selected, err := chooseDownloads(inv, assets)
	if err != nil {
		return err
	}

	var chosen []ne.Asset
	for _, a := range page.Assets {
		if selected[a.BlockID] {
			chosen = append(chosen, a)
		}
	}
	reporter := newDownloadReporter(inv, chosen)
	result, err := service.Export(ctx, page, ne.ExportOptions{
		OutDir: outDir, Format: format, Selected: selected, Concurrency: concurrency, Force: force,
		OnDownload: reporter.event, OnBytes: reporter.bytes,
	})
	reporter.finish()
	if err != nil {
		if errors.Is(err, ne.ErrDestinationExists) || errors.Is(err, ne.ErrForeignDestination) {
			return destinationFailure(err)
		}
		return failure(ctx, "export não concluído; nenhum arquivo de destino foi alterado", err)
	}
	if result.CleanupWarning != nil {
		say("Aviso: export publicado, mas o export anterior não foi removido: %v", result.CleanupWarning)
	}
	say("")
	say("Export concluído.")
	say("Título: %s", result.Title)
	say("Page ID: %s", result.PageID)
	say("Assets encontrados: %d", result.Discovered)
	say("Assets baixados: %d (%s)", result.Downloaded, FormatBytes(result.DownloadedBytes))
	say("Assets ignorados: %d", result.Skipped)
	say("Assets com falha: %d", result.Failed)
	if result.MarkdownPath != "" {
		say("Markdown: %s", result.MarkdownPath)
	}
	if result.HTMLPath != "" {
		say("HTML: %s", result.HTMLPath)
	}
	if result.Failed > 0 {
		say("Aviso: %d asset(s) falharam e permaneceram com referência remota. Veja manifest.json.", result.Failed)
		if fromJSON != "" {
			say("As signed URLs do page.json podem ter expirado; exporte novamente pela URL pública para renová-las.")
		}
	}
	_, err = fmt.Fprintln(inv.IO.Out, result.Dir)
	return err
}

func failure(ctx context.Context, message string, err error) error {
	if ctx.Err() != nil {
		return errors.Join(ctx.Err(), err)
	}
	if errors.Is(err, ne.ErrInvalidPageID) {
		return usage(err.Error(), err)
	}
	return &core.Diagnostic{Code: "notionexport.failed", Kind: "export", Message: message + ": " + err.Error(), Class: core.ExitExecution, Cause: err}
}

func destinationFailure(err error) error {
	hint := ""
	if errors.Is(err, ne.ErrDestinationExists) {
		hint = "use --force para substituir um export anterior da mesma página, --name para outro nome ou outro --out"
	} else if errors.Is(err, ne.ErrForeignDestination) {
		hint = "o destino não foi alterado; use --name ou outro --out"
	}
	return &core.Diagnostic{Code: "notionexport.destination", Kind: "destination", Message: err.Error(), Hint: hint, Class: core.ExitExecution, Cause: err}
}
