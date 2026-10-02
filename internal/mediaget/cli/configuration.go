package mediacli

import (
	"fmt"
	"strconv"
	"strings"

	core "github.com/matheusvcouto/cli-tools/cli"
	"github.com/matheusvcouto/cli-tools/internal/mediaget"
)

// A closed catalog deliberately avoids arbitrary yt-dlp arguments, cookies or shell commands.
func configure(inv *core.Invocation, src *mediaget.Source) (sourceChanged bool, err error) {
	labels := []string{"Referer — página de origem", fmt.Sprintf("Fragmentos paralelos — HLS/DASH (1 a %d)", mediaget.MaxConcurrentFragments)}
	catalog := labels
	indices := []int{0, 1}
	// The native selector searches as you type. Text terminals keep a small searchable catalog.
	if _, native := inv.Interaction.(terminalInteraction); !native || !inv.Terminal.StderrTTY {
		query, err := ask(inv, "Buscar configuração (Enter mostra todas)", "")
		if err != nil {
			return false, err
		}
		labels = nil
		indices = nil
		for i, label := range catalog {
			if strings.Contains(strings.ToLower(label), strings.ToLower(query)) {
				labels = append(labels, label)
				indices = append(indices, i)
			}
		}
		if len(labels) == 0 {
			fmt.Fprintln(inv.IO.Err, "Nenhuma configuração encontrada.")
			return false, nil
		}
	}
	selected, err := choose(inv, "Adicionar ou editar configuração", labels, true)
	if err != nil || selected < 0 {
		return false, err
	}
	switch indices[selected] {
	case 0:
		for {
			referer, err := ask(inv, "Referer (URL da página; Enter remove)", "")
			if err != nil {
				return false, err
			}
			next := *src
			next.Referer = referer
			if err := mediaget.ValidateSource(next); err != nil {
				fmt.Fprintln(inv.IO.Err, "Informe uma URL HTTP(S) válida.")
				continue
			}
			changed := next.Referer != src.Referer
			*src = next
			fmt.Fprintln(inv.IO.Err, "Referer atualizado (valor oculto).")
			return changed, nil
		}
	case 1:
		for {
			fragments, err := ask(inv, fmt.Sprintf("Fragmentos paralelos (1 a %d)", mediaget.MaxConcurrentFragments), strconv.Itoa(src.FragmentConcurrency()))
			if err != nil {
				return false, err
			}
			n, err := parseFragments(fragments)
			if err != nil {
				fmt.Fprintln(inv.IO.Err, err)
				continue
			}
			src.ConcurrentFragments = n
			fmt.Fprintf(inv.IO.Err, "Fragmentos paralelos: %d (somente protocolos compatíveis).\n", n)
			return false, nil // Download concurrency does not change metadata or cached sizes.
		}
	}
	return false, nil
}

func inspectInitial(inv *core.Invocation, service mediaget.Service, src *mediaget.Source, automatic bool, prefetch **estimateSession) (mediaget.Info, error) {
	for {
		if !automatic {
			*prefetch = newEarlyEstimateSession(inv.Context, service, *src)
		}
		info, err := withLoading(inv, "Consultando mídia", func() (mediaget.Info, error) { return service.Inspect(inv.Context, *src, mediaget.Selection{}) })
		if err == nil || automatic || inv.Context.Err() != nil || core.ExitCode(err) != 1 {
			return info, err
		}
		if *prefetch != nil {
			(*prefetch).close()
			*prefetch = nil
		}
		fmt.Fprintln(inv.IO.Err, err)
		index, promptErr := choose(inv, "Não foi possível consultar. Como continuar?", []string{"Adicionar configuração (ex.: Referer)", "Tentar novamente", "Cancelar"}, false)
		if promptErr != nil {
			return info, promptErr
		}
		if index == 2 {
			return info, errDeclined
		}
		if index == 0 {
			if _, err := configure(inv, src); err != nil {
				return info, err
			}
		}
	}
}

func parseFragments(raw string) (int, error) {
	n, err := strconv.Atoi(raw)
	if err != nil || n < 1 || n > mediaget.MaxConcurrentFragments {
		return 0, fmt.Errorf("fragmentos paralelos devem estar entre 1 e %d", mediaget.MaxConcurrentFragments)
	}
	return n, nil
}

func fragmentsValue() core.Value {
	return core.TypedValue(core.CodecFuncs[int]{
		Name:      "fragment_concurrency",
		ParseFunc: parseFragments, FormatFunc: strconv.Itoa,
	})
}

func printDownloadConfiguration(inv *core.Invocation, src mediaget.Source, kind mediaget.Kind) {
	if src.Referer != "" {
		fmt.Fprintln(inv.IO.Err, "Configuração: Referer definido (valor oculto)")
	}
	if kind == mediaget.Video || kind == mediaget.Audio {
		fmt.Fprintf(inv.IO.Err, "Configuração: %d fragmentos paralelos (HLS/DASH)\n", src.FragmentConcurrency())
	}
}
