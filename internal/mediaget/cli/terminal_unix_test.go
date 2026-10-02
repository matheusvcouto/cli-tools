//go:build darwin || linux

package mediacli

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"
)

func TestNativeKeyDecoderAndCancellationDoNotLeaveReader(t *testing.T) {
	in, out, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer in.Close()
	defer out.Close()
	for _, tc := range []struct{ input, key string }{{"\x1b[A", "up"}, {"\x1b[B", "down"}, {"\x1b[C", "right"}, {"\x1b[D", "left"}, {"\x1b[1;5D", "left"}, {"\x1b[200~", ""}, {"\x1b[201~", ""}, {"\x1b[H", "home"}, {"\x1b[F", "end"}, {"\x1b[3~", "delete"}, {"\x1b[1~", "home"}, {"\x1b[4~", "end"}, {"\x04", "eof"}, {"\r", "enter"}, {"\x7f", "backspace"}, {"á", "á"}, {"\x03", "interrupt"}, {"\x1b", "cancel"}} {
		if _, err := out.WriteString(tc.input); err != nil {
			t.Fatal(err)
		}
		key, err := readTerminalKey(context.Background(), in)
		if err != nil || key != tc.key {
			t.Fatalf("%q => %q %v", tc.input, key, err)
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { _, err := readTerminalKey(ctx, in); done <- err }()
	cancel()
	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Fatal(err)
		}
	case <-time.After(time.Second):
		t.Fatal("native reader blocked cancellation")
	}
	// A canceled selector must not consume input intended for the next prompt.
	out.WriteString("x")
	key, err := readTerminalKey(context.Background(), in)
	if err != nil || key != "x" {
		t.Fatal(key, err)
	}
}
