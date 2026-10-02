package mediacli

import (
	"context"
	"errors"
	"testing"
)

func TestTextEditingMovesInsertsDeletesAndHandlesUnicode(t *testing.T) {
	model := textModel{}
	for _, key := range []string{"a", "ç", "c", "left", "left", "b", "right", "delete", "home", "delete", "end", "!"} {
		model.apply(key)
	}
	if string(model.text) != "bç!" || model.cursor != 3 {
		t.Fatalf("%q cursor %d", string(model.text), model.cursor)
	}
	model.apply("backspace")
	model.apply("left")
	model.apply("X")
	if string(model.text) != "bXç" {
		t.Fatal(string(model.text))
	}
	for _, key := range []string{"up", "down", "left", "left", "left", "left", "backspace", "\x1b", "\u202e"} {
		model.apply(key)
	}
	if string(model.text) != "bXç" || model.cursor != 0 {
		t.Fatalf("%q %d", string(model.text), model.cursor)
	}
}
func TestCancellationMessagePreservesCauseAndOutcome(t *testing.T) {
	cause := errors.Join(errors.New("synthetic backend diagnostic"), context.Canceled)
	err := cancellationOutcome(cause, "Cancelado. Arquivos incompletos descartados.")
	if !errors.Is(err, context.Canceled) || CancellationMessage(err) != "Cancelado. Arquivos incompletos descartados." {
		t.Fatal(err)
	}
	if CancellationMessage(context.Canceled) != "Cancelado." {
		t.Fatal("technical cancellation exposed")
	}
}
