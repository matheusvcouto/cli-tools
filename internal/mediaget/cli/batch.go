package mediacli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"

	core "github.com/matheusvcouto/cli-tools/cli"
	"github.com/matheusvcouto/cli-tools/internal/mediaget"
	manifest "github.com/matheusvcouto/cli-tools/internal/mediaget/batch"
)

const batchPrefix = prefix + "batch."

func batchCommand(service mediaget.Service, singleFlags []core.Flag) core.Command {
	fileArg := func(id string) core.Arg {
		return core.Arg{ID: id, Name: "file", Summary: "local JSON manifest", Value: core.FileValue(), Required: true}
	}
	var flags []core.Flag
	for _, f := range singleFlags {
		if f.ID == prefix+"name" {
			continue
		}
		f.ID = batchPrefix + strings.TrimPrefix(f.ID, prefix)
		// Resolve env lazily per effective item, after JSON overrides. Static
		// commands and manifests with explicit values never consult fallbacks.
		f.Providers = nil
		f.Default = nil
		if f.Long == "yes" {
			f.Summary = "confirm the complete batch without prompts"
		}
		flags = append(flags, f)
	}
	flags = append(flags,
		core.Flag{ID: batchPrefix + "origin", Long: "origin", Summary: "HTTP(S) Origin header override", Value: core.StringValue(), Sensitive: true},
		core.Flag{ID: batchPrefix + "jobs", Long: "jobs", Summary: "simultaneous downloads (1 to 8; default 2)", Value: jobsValue()},
		core.Flag{ID: batchPrefix + "on-error", Long: "on-error", Summary: "continue other items or stop starting new ones", Value: core.EnumValue(core.Choice{Value: "continue"}, core.Choice{Value: "stop"})},
	)
	return core.Command{ID: batchPrefix + "root", Name: "batch", Summary: "download a reviewed JSON batch", OptionPolicy: core.OptionsInterspersed,
		Args: []core.Arg{fileArg(batchPrefix + "file")}, Flags: flags,
		Handler: func(inv *core.Invocation) error { return runBatch(inv, service) },
		Commands: []core.Command{
			{ID: batchPrefix + "validate", Name: "validate", Summary: "validate a manifest offline; does not check media", Args: []core.Arg{fileArg(batchPrefix + "validate.file")}, Handler: func(inv *core.Invocation) error {
				file, _ := core.ValueAs[string](inv, batchPrefix+"validate.file")
				m, err := readManifest(file)
				if err != nil {
					return usage(err.Error())
				}
				_, err = fmt.Fprintf(inv.IO.Out, "Manifesto válido: %d itens. Mídias não verificadas.\n", len(m.Items))
				return err
			}},
			{ID: batchPrefix + "schema", Name: "schema", Summary: "print the embedded batch JSON Schema", Handler: func(inv *core.Invocation) error { return writeJSON(inv.IO.Out, manifest.Schema()) }},
			{ID: batchPrefix + "example", Name: "example", Summary: "print a synthetic batch example", Handler: func(inv *core.Invocation) error { return writeJSON(inv.IO.Out, manifest.Example()) }},
		},
	}
}

func jobsValue() core.Value {
	return core.TypedValue(core.CodecFuncs[int]{Name: "batch_jobs", ParseFunc: func(s string) (int, error) {
		n, err := strconv.Atoi(s)
		if err != nil || n < 1 || n > 8 {
			return 0, errors.New("use 1 a 8")
		}
		return n, nil
	}, FormatFunc: strconv.Itoa})
}
func writeJSON(w io.Writer, data []byte) error {
	n, err := w.Write(data)
	if err == nil && n != len(data) {
		err = io.ErrShortWrite
	}
	return err
}

type checkedBatchWriter struct{ io.Writer }

func (w checkedBatchWriter) Write(p []byte) (int, error) {
	n, err := w.Writer.Write(p)
	if err == nil && n != len(p) {
		err = io.ErrShortWrite
	}
	return n, err
}
func readManifest(file string) (manifest.Manifest, error) {
	before, err := os.Stat(file)
	if err != nil || !before.Mode().IsRegular() {
		return manifest.Manifest{}, errors.New("manifesto deve ser arquivo regular")
	}
	f, err := os.Open(file)
	if err != nil {
		return manifest.Manifest{}, errors.New("não foi possível abrir o manifesto")
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil || !info.Mode().IsRegular() || !os.SameFile(before, info) {
		return manifest.Manifest{}, errors.New("manifesto deve ser arquivo regular")
	}
	return manifest.Read(f)
}
func batchString(inv *core.Invocation, key string) string {
	s, _ := core.ValueAs[string](inv, batchPrefix+key)
	return s
}
func batchOverrides(inv *core.Invocation) manifest.Options {
	o := manifest.Options{}
	for _, field := range []struct {
		key    string
		target **string
	}{
		{"kind", &o.Kind}, {"subtitle-format", &o.SubtitleFormat}, {"video-format", &o.VideoFormat}, {"lang", &o.SubtitleLang}, {"referer", &o.Referer}, {"origin", &o.Origin}, {"output", &o.OutputDir},
	} {
		if inv.Present(batchPrefix + field.key) {
			v := batchString(inv, field.key)
			*field.target = &v
		}
	}
	if inv.Present(batchPrefix + "quality") {
		height, _ := core.ValueAs[int](inv, batchPrefix+"quality")
		v := "best"
		if height > 0 {
			v = strconv.Itoa(height)
		}
		o.Quality = &v
	}
	if inv.Present(batchPrefix + "auto") {
		v, _ := core.ValueAs[bool](inv, batchPrefix+"auto")
		o.AutoSubs = &v
	}
	if inv.Present(batchPrefix + "fragments") {
		v, _ := core.ValueAs[int](inv, batchPrefix+"fragments")
		o.ConcurrentFragments = &v
	}
	return o
}

type batchEntry struct {
	id       string
	req      mediaget.Request
	info     mediaget.Info
	estimate transferEstimate
	err      error
	skip     bool
}

func runBatch(inv *core.Invocation, service mediaget.Service) error {
	local := *inv
	local.IO.Err = checkedBatchWriter{inv.IO.Err}
	local.IO.Out = checkedBatchWriter{inv.IO.Out}
	inv = &local
	for _, key := range []string{"name", "kind", "quality", "output", "fragments", "referer", "yes", "lang", "auto", "subtitle-format", "video-format"} {
		if inv.Present(prefix + key) {
			return usage("informe flags de lote depois do comando batch; --name não é aceito no lote")
		}
	}
	automatic, _ := core.ValueAs[bool](inv, batchPrefix+"yes")
	if !automatic && !inv.Terminal.StdinTTY {
		return usage("use um terminal ou batch arquivo.json --yes")
	}
	m, err := readManifest(batchString(inv, "file"))
	if err != nil {
		return usage(err.Error())
	}
	jobs := m.Execution.Jobs
	if inv.Present(batchPrefix + "jobs") {
		jobs, _ = core.ValueAs[int](inv, batchPrefix+"jobs")
	}
	if jobs < 1 || jobs > 8 {
		return usage("jobs deve estar entre 1 e 8")
	}
	onError := m.Execution.OnError
	if inv.Present(batchPrefix + "on-error") {
		onError = batchString(inv, "on-error")
	}
	overrides := batchOverrides(inv)
	entries := make([]batchEntry, len(m.Items))
	for i := range m.Items {
		// Fallbacks are read only for missing fields on this item.
		effective := m.Defaults.Overlay(m.Items[i].Options).Overlay(overrides)
		fallback := manifest.Options{}
		if effective.OutputDir == nil {
			dir, ok := os.LookupEnv(DownloadEnv)
			if !ok {
				home, e := os.UserHomeDir()
				if e != nil {
					return e
				}
				dir = mediaget.DefaultDownloadDir(home)
			}
			fallback.OutputDir = &dir
		}
		if effective.ConcurrentFragments == nil {
			fragments := mediaget.DefaultConcurrentFragments
			if v, ok := os.LookupEnv(FragmentsEnv); ok {
				n, e := strconv.Atoi(v)
				if e != nil || n < 1 || n > mediaget.MaxConcurrentFragments {
					return usage("variável de fragmentos inválida")
				}
				fragments = n
			}
			fallback.ConcurrentFragments = &fragments
		}
		req, e := m.Request(i, fallback, overrides)
		if e != nil {
			return usage(fmt.Sprintf("Item %d: %v", i+1, e))
		}
		req.KeepIncomplete = !automatic
		entries[i] = batchEntry{id: m.Items[i].ID, req: req}
	}
	for _, warning := range m.Warnings {
		if _, err := fmt.Fprintln(inv.IO.Err, warning); err != nil {
			return err
		}
	}
	_, err = withLoadingProgress(inv, "Verificando destinos e dependências", len(entries), func(report func()) (struct{}, error) {
		// Validate every destination/dependency before making network requests.
		for i := range entries {
			e := &entries[i]
			e.req.OutputDir, err = filepath.Abs(e.req.OutputDir)
			if err != nil {
				return struct{}{}, err
			}
			if err = service.ValidateDestination(e.req.OutputDir); err != nil {
				e.err = err
				report()
				continue
			}
			e.err = service.Check(inv.Context, e.req.Selection)
			report()
		}
		return struct{}{}, inv.Context.Err()
	})
	if err != nil {
		return err
	}

	if automatic {
		for i, e := range entries {
			if e.err != nil {
				return fmt.Errorf("item %d: %w; nenhum download iniciado", i+1, e.err)
			}
		}
	}
	_, err = withLoadingProgress(inv, "Consultando mídias do lote", len(entries), func(report func()) (struct{}, error) {
		var wg sync.WaitGroup
		queue := make(chan int)
		for range min(3, len(entries)) {
			wg.Add(1)
			go func() {
				defer wg.Done()
				for i := range queue {
					if entries[i].err == nil {
						prepareEntry(inv.Context, service, &entries[i])
					}
					report()
				}
			}()
		}
		for i := range entries {
			queue <- i
		}
		close(queue)
		wg.Wait()
		return struct{}{}, inv.Context.Err()
	})
	if err != nil {
		return err
	}
	for i := range entries {
		e := &entries[i]
		if e.err == nil && strings.TrimSpace(e.req.Name) == "" {
			if strings.TrimSpace(e.info.Title) != "" {
				e.req.Name = e.info.Title
			} else if automatic {
				e.err = errors.New("nome ausente; informe name/title no manifesto")
			} else {
				e.req.Name, err = ask(inv, fmt.Sprintf("Nome do item %d", i+1), "")
				if err != nil {
					return err
				}
				if e.req.Name == "" {
					e.err = errors.New("nome vazio")
				}
			}
		}
		previous := e.req.Selection
		e.req.Name, e.req.Selection = mediaget.OutputSelection(e.req.Name, e.req.Selection)
		if e.err == nil && previous != e.req.Selection {
			prepareEntryLoading(inv, service, e)
		}
	}
	for {
		uniqueBatchNames(entries)
		if err := printBatchSummary(inv, entries, jobs, onError); err != nil {
			return err
		}
		bad := false
		included := 0
		for _, e := range entries {
			if !e.skip {
				included++
				bad = bad || e.err != nil
			}
		}
		if included == 0 {
			return usage("nenhum item selecionado")
		}
		if automatic {
			if bad {
				return errors.New("preparação do lote falhou; nenhum download iniciado")
			}
			break
		}
		action, e := choose(inv, "O que fazer com este lote?", []string{"Baixar todos com estas opções", "Editar ou excluir um item", "Formato de saída de todos os vídeos", "Configurar simultaneidade", "Cancelar"}, false)
		if e != nil {
			return e
		}
		if action == 4 {
			return nil
		}
		if action == 0 {
			if bad {
				fmt.Fprintln(inv.IO.Err, "Corrija ou exclua os itens com falha antes de baixar.")
				continue
			}
			break
		}
		if action == 2 {
			f, ex := chooseVideoFormat(inv)
			if ex != nil {
				return ex
			}
			for i := range entries {
				e := &entries[i]
				if e.skip || e.req.Selection.Kind != mediaget.Video {
					continue
				}
				if e.req.Selection.VideoFormat != f {
					e.req.Selection.VideoFormat = f
					prepareEntryLoading(inv, service, e)
				}
			}
			if err := inv.Context.Err(); err != nil {
				return err
			}
			continue
		}
		if action == 3 {
			v, e := ask(inv, "Downloads simultâneos (1 a 8)", strconv.Itoa(jobs))
			if e != nil {
				return e
			}
			n, e := strconv.Atoi(v)
			if e != nil || n < 1 || n > 8 {
				fmt.Fprintln(inv.IO.Err, "Use 1 a 8.")
				continue
			}
			jobs = n
			continue
		}
		labels := make([]string, len(entries))
		for i, e := range entries {
			labels[i] = fmt.Sprintf("%d. %s", i+1, mediaget.SafeName(e.req.Name))
			if e.skip {
				labels[i] += " (excluído)"
			}
		}
		i, e := choose(inv, "Qual item?", labels, true)
		if e != nil {
			return e
		}
		if i < 0 {
			continue
		}
		if err := editBatchEntry(inv, service, &entries[i]); err != nil {
			return err
		}
	}
	requests := []mediaget.Request{}
	indices := []int{}
	processing := make(chan struct{}, 1)
	acquire := func(ctx context.Context) (func(), error) {
		select {
		case processing <- struct{}{}:
			return func() { <-processing }, nil
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
	for i, e := range entries {
		if !e.skip {
			e.req.AcquireProcessing = acquire
			requests = append(requests, e.req)
			indices = append(indices, i)
		}
	}
	results, runErr := downloadBatchWithProgress(inv, service, requests, indices, entries, jobs, onError == "stop")
	for i, r := range results {
		if r.Result.Incomplete != nil {
			cleanupErr := finishIncomplete(inv, r.Result.Incomplete, r.Err)
			if cleanupErr != nil {
				fmt.Fprintln(inv.IO.Err, cleanupErr)
			}
		}
		state := "não iniciado"
		if r.Started {
			state = "concluído"
			if r.Err != nil {
				state = "falhou"
				if errors.Is(r.Err, context.Canceled) {
					state = "cancelado"
				}
			}
		}
		if _, e := fmt.Fprintf(inv.IO.Err, "Item %d: %s\n", indices[i]+1, state); e != nil && runErr == nil {
			runErr = e
		}
	}
	return runErr
}

func prepareEntry(ctx context.Context, service mediaget.Service, e *batchEntry) {
	if err := mediaget.ValidateSource(e.req.Source); err != nil {
		e.err = err
		return
	}
	if err := mediaget.ValidateSelection(e.req.Selection); err != nil {
		e.err = err
		return
	}
	if err := service.ValidateDestination(e.req.OutputDir); err != nil {
		e.err = err
		return
	}
	if err := service.Check(ctx, e.req.Selection); err != nil {
		e.err = err
		return
	}
	info, err := service.Inspect(ctx, e.req.Source, e.req.Selection)
	if err != nil {
		e.err = err
		return
	}
	if e.req.Selection.Kind == mediaget.Subtitle {
		track, err := findTrack(info, e.req.Selection.Track.Lang, e.req.Selection.Track.Auto)
		if err != nil {
			e.err = err
			return
		}
		e.req.Selection.Track = track
	}
	e.info = info
	e.err = nil
	e.estimate = estimateFromInfo(info)
	if e.req.Selection.Kind == mediaget.Subtitle {
		e.estimate = transferEstimate{}
	}
}

// Deduplicate proposed bases in manifest order, without reserving/replacing
// existing files. Service.Download still arbitrates external collisions.
func uniqueBatchNames(entries []batchEntry) {
	used := map[string]bool{}
	for i := range entries {
		e := &entries[i]
		if e.skip || e.err != nil {
			continue
		}
		base := mediaget.SafeName(e.req.Name)
		name := base
		for n := 1; used[strings.ToLower(name)]; n++ {
			suffix := fmt.Sprintf(" (%d)", n)
			trimmed := []rune(base)
			for len(string(trimmed))+len(suffix) > 180 {
				trimmed = trimmed[:len(trimmed)-1]
			}
			name = string(trimmed) + suffix
		}
		used[strings.ToLower(name)] = true
		e.req.Name = name
	}
}
func printBatchSummary(inv *core.Invocation, entries []batchEntry, jobs int, onError string) error {
	if _, err := fmt.Fprintf(inv.IO.Err, "\nResumo do lote: %d itens | %d downloads simultâneos | falhas: %s\n", len(entries), jobs, onError); err != nil {
		return err
	}
	var knownBytes int64
	unknown := 0
	for i, e := range entries {
		if _, err := fmt.Fprintf(inv.IO.Err, "\nItem %d — %s\n", i+1, mediaget.SafeName(e.id)); err != nil {
			return err
		}
		if e.skip {
			if _, err := fmt.Fprintln(inv.IO.Err, "Excluído da execução."); err != nil {
				return err
			}
			continue
		}
		if e.err != nil {
			unknown++
			if _, err := fmt.Fprintln(inv.IO.Err, "Preparação falhou:", e.err); err != nil {
				return err
			}
			continue
		}
		if err := printDownloadSummary(inv, e.info, e.req.Source, e.req.Selection, e.estimate, e.req.Name, e.req.OutputDir); err != nil {
			return err
		}
		if e.estimate.known && e.estimate.bytes <= int64(^uint64(0)>>1)-knownBytes {
			knownBytes += e.estimate.bytes
		} else {
			unknown++
		}
		if jobs*e.req.Source.FragmentConcurrency() > 64 {
			if _, err := fmt.Fprintln(inv.IO.Err, "Aviso: combinação alta de jobs e fragmentos; o servidor pode limitar as conexões."); err != nil {
				return err
			}
		}
	}
	_, err := fmt.Fprintf(inv.IO.Err, "Transferência conhecida: ≈ %s; %d itens sem estimativa.\n", mediaget.HumanSize(knownBytes), unknown)
	return err
}

func editBatchEntry(inv *core.Invocation, service mediaget.Service, e *batchEntry) error {
	field, err := choose(inv, "O que deseja editar?", []string{"Nome", "Destino", "Tipo e formato", "Qualidade/idioma", "Referer", "Origin", "Fragmentos", "URL", "Excluir/incluir item", "Tentar preparação novamente", "Formato de saída"}, true)
	if err != nil || field < 0 {
		return err
	}
	if field == 8 {
		e.skip = !e.skip
		return nil
	}
	old := e.req
	var answer string
	switch field {
	case 10:
		if e.req.Selection.Kind == mediaget.Video {
			e.req.Selection.VideoFormat, err = chooseVideoFormat(inv)
		} else if e.req.Selection.Kind == mediaget.Subtitle {
			f, ex := choose(inv, "Formato da legenda", []string{"SRT", "TXT — somente texto"}, false)
			err = ex
			if ex == nil {
				e.req.Selection.SubtitleFormat = []string{"srt", "txt"}[f]
			}
		} else {
			fmt.Fprintln(inv.IO.Err, "Áudio usa M4A.")
		}
	case 0:
		answer, err = ask(inv, "Nome do arquivo", e.req.Name)
		e.req.Name = answer
	case 1:
		answer, err = ask(inv, "Destino (será criado se necessário)", e.req.OutputDir)
		e.req.OutputDir = answer
	case 2:
		k, ex := choose(inv, "Tipo", []string{"Vídeo", "Áudio M4A", "Legenda"}, false)
		if ex != nil {
			return ex
		}
		e.req.Selection = mediaget.Selection{Kind: []mediaget.Kind{mediaget.Video, mediaget.Audio, mediaget.Subtitle}[k]}
		if k == 0 {
			f, ex := choose(inv, "Formato", []string{"Automático", "MP4 compatível (pode recodificar)"}, false)
			if ex != nil {
				return ex
			}
			if f == 1 {
				e.req.Selection.VideoFormat = "mp4"
			}
		}
		if k == 2 {
			f, ex := choose(inv, "Formato", []string{"SRT", "TXT — somente texto"}, false)
			if ex != nil {
				return ex
			}
			e.req.Selection.SubtitleFormat = []string{"srt", "txt"}[f]
			e.req.Selection.Track.Lang, err = ask(inv, "Idioma exato da legenda", "")
		}
	case 3:
		if e.req.Selection.Kind == mediaget.Video {
			answer, err = ask(inv, "Qualidade (best, 2160, 1080, 720, 480, 360)", "best")
			if err == nil {
				var height int
				switch answer {
				case "best":
				case "2160", "1080", "720", "480", "360":
					height, _ = strconv.Atoi(answer)
				default:
					err = usage("qualidade inválida")
				}
				e.req.Selection.Height = height
			}
		}
		if e.req.Selection.Kind == mediaget.Subtitle {
			e.req.Selection.Track.Lang, err = ask(inv, "Idioma exato", e.req.Selection.Track.Lang)
			if err == nil {
				k, ex := choose(inv, "Faixa", []string{"Manual", "Automática"}, false)
				err = ex
				e.req.Selection.Track.Auto = k == 1
			}
		}
	case 4:
		e.req.Source.Referer, err = ask(inv, "Referer (vazio remove)", "")
	case 5:
		e.req.Source.Origin, err = ask(inv, "Origin (vazio remove)", "")
	case 6:
		answer, err = ask(inv, "Fragmentos (1 a 256)", strconv.Itoa(e.req.Source.FragmentConcurrency()))
		if err == nil {
			e.req.Source.ConcurrentFragments, err = strconv.Atoi(answer)
		}
	case 7:
		e.req.Source.URL, err = ask(inv, "URL da mídia", e.req.Source.URL)
	}
	if err != nil {
		e.req = old
		if errors.Is(err, context.Canceled) || errors.Is(err, errDeclined) {
			return err
		}
		fmt.Fprintln(inv.IO.Err, "Edição inválida; opções anteriores preservadas.")
		return nil
	}
	e.req.Name, e.req.Selection = mediaget.OutputSelection(e.req.Name, e.req.Selection)
	if strings.TrimSpace(e.req.Name) == "" {
		e.err = errors.New("nome vazio")
		return nil
	}
	// Name/destination/fragments reuse metadata. Other edits invalidate this
	// item only; retries explicitly renew its inspection.
	before, after := old.Source, e.req.Source
	before.ConcurrentFragments = 0
	after.ConcurrentFragments = 0
	if field == 9 || before != after || old.Selection != e.req.Selection || e.err != nil {
		prepareEntryLoading(inv, service, e)
	} else {
		e.err = service.ValidateDestination(e.req.OutputDir)
	}
	return nil
}

func chooseVideoFormat(inv *core.Invocation) (string, error) {
	f, err := choose(inv, "Formato de saída do vídeo", []string{"Automático (contêiner da fonte)", "MP4 compatível (.mp4; pode recodificar)"}, false)
	if err != nil {
		return "", err
	}
	return []string{"auto", "mp4"}[f], nil
}
func prepareEntryLoading(inv *core.Invocation, service mediaget.Service, e *batchEntry) {
	_, err := withLoading(inv, "Atualizando mídia e formato", func() (struct{}, error) { prepareEntry(inv.Context, service, e); return struct{}{}, inv.Context.Err() })
	if err != nil {
		e.err = err
	}
}
