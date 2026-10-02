package mediacli

import (
	"fmt"

	core "github.com/matheusvcouto/cli-tools/cli"
	"github.com/matheusvcouto/cli-tools/internal/mediaget"
)

type downloadReview struct {
	Name, Dir string
	Selection mediaget.Selection
	Estimate  transferEstimate
}

// Final review edits invocation-local options, including those seeded by flags
// or environment. Nothing is saved globally and no transfer starts in this loop.
func reviewDownload(inv *core.Invocation, service mediaget.Service, src *mediaget.Source, info *mediaget.Info, review *downloadReview, prefetch **estimateSession) error {
	for {
		review.Estimate = (*prefetch).get(review.Selection).estimate
		if err := printDownloadSummary(inv, *info, *src, review.Selection, review.Estimate, review.Name, review.Dir); err != nil {
			return err
		}
		action, err := choose(inv, "O que fazer com este download?", []string{"Baixar com estas opções", "Editar opções", "Cancelar"}, false)
		if err != nil {
			return err
		}
		if action == 0 {
			return nil
		}
		if action == 2 {
			return errDeclined
		}
		field, err := choose(inv, "O que deseja editar?", []string{"Tipo, qualidade ou legenda", "Nome do arquivo", "Diretório de download", "Referer ou fragmentos paralelos"}, true)
		if err != nil {
			return err
		}
		switch field {
		case 0:
			review.Selection, err = wizard(inv, service, src, info, mediaget.Selection{}, &review.Estimate, prefetch, false)
		case 1:
			review.Name, err = ask(inv, "Nome do arquivo", mediaget.SafeName(review.Name))
		case 2:
			for {
				var dir string
				dir, err = ask(inv, "Diretório de download (deve existir)", review.Dir)
				if err != nil {
					break
				}
				if validationErr := service.ValidateDestination(dir); validationErr != nil {
					fmt.Fprintln(inv.IO.Err, validationErr)
					continue
				}
				review.Dir = dir
				break
			}
		case 3:
			var changed bool
			changed, err = configure(inv, src)
			if err == nil && changed {
				(*prefetch).close()
				*info, err = withLoading(inv, "Atualizando mídia", func() (mediaget.Info, error) { return service.Inspect(inv.Context, *src, mediaget.Selection{}) })
				if err != nil {
					return err
				}
				*prefetch = newEarlyEstimateSession(inv.Context, service, *src)
				review.Selection, err = wizard(inv, service, src, info, mediaget.Selection{}, &review.Estimate, prefetch, false)
			}
		}
		if err != nil {
			return err
		}
	}
}
