package mediacli

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	core "github.com/matheusvcouto/cli-tools/cli"
	"github.com/matheusvcouto/cli-tools/internal/mediaget"
)

func finishIncomplete(inv *core.Invocation, partial *mediaget.Incomplete, cause error) error {
	if partial == nil {
		return cause
	}
	size := "tamanho indisponível"
	if bytes, err := partial.Size(); err == nil {
		size = mediaget.HumanSize(bytes)
	}
	fmt.Fprintf(inv.IO.Err, "Download interrompido. Arquivos incompletos: %s (%s)\n", partial.Path, size)
	// The transfer's context may already be canceled. A new signal-aware context
	// permits one bounded cleanup decision; another interrupt/Esc/EOF discards.
	ctx, stop := core.SignalContext(context.WithoutCancel(inv.Context))
	defer stop()
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	cleanup := *inv
	cleanup.Context = ctx
	choice, err := choose(&cleanup, "O que fazer com os arquivos incompletos? (padrão: descartar; 30 s)", []string{"Descartar download incompleto", "Manter arquivos"}, false)
	if err == nil && choice == 1 {
		return cancellationOutcome(cause, fmt.Sprintf("Cancelado. Arquivos incompletos mantidos em %s (%s)", partial.Path, size))
	}
	fmt.Fprintln(inv.IO.Err, "Descartando somente a pasta deste download:", partial.Path)
	if err := partial.Discard(); err != nil {
		return cancellationOutcome(cause, fmt.Sprintf("Cancelado. Não foi possível descartar %s: %v", partial.Path, err))
	}
	return cancellationOutcome(cause, "Cancelado. Arquivos incompletos descartados.")
}

// Keep cancellation recognizable to callers while displaying its cleanup outcome.
type cancellationNotice struct {
	cause   error
	message string
}

func (n *cancellationNotice) Error() string { return n.message }
func (n *cancellationNotice) Unwrap() error { return n.cause }
func cancellationOutcome(cause error, message string) error {
	if errors.Is(cause, context.Canceled) {
		return &cancellationNotice{cause: cause, message: message}
	}
	// Ordinary failures still include the original safe diagnostic.
	return fmt.Errorf("%w; %s", cause, strings.TrimPrefix(message, "Cancelado. "))
}
func CancellationMessage(err error) string {
	var notice *cancellationNotice
	if errors.As(err, &notice) {
		return notice.message
	}
	return "Cancelado."
}
