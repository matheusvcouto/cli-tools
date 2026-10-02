package mediacli

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	core "github.com/matheusvcouto/cli-tools/cli"
	"github.com/matheusvcouto/cli-tools/internal/mediaget"
)

func TestPrefetchStartsBothKindsAndQualitiesWithoutSelection(t *testing.T) {
	backend := &previewBackend{started: make(chan struct{}, 16), release: make(chan struct{})}
	session := newEarlyEstimateSession(context.Background(), mediaget.Service{Backend: backend}, mediaget.Source{URL: "https://example.invalid"})
	defer session.close()
	// Both begin before any quality menu/metadata completion.
	for range 2 {
		select {
		case <-backend.started:
		case <-time.After(time.Second):
			t.Fatal("no early prefetch")
		}
	}
	selections := []mediaget.Selection{{Kind: mediaget.Video}, {Kind: mediaget.Audio}}
	labels := session.labels(selections, []string{"Video", "Audio"})
	if !strings.Contains(labels[0], "calculando") || !strings.Contains(labels[1], "calculando") {
		t.Fatal(labels)
	}
	inv := &core.Invocation{Context: context.Background()}
	scheduleQualities(inv, session, mediaget.Info{Heights: []int{360, 720, 1080}})
	// A quality starts while the first two inspections are still blocked.
	select {
	case <-backend.started:
	case <-time.After(time.Second):
		t.Fatal("quality waits for type selection")
	}
	close(backend.release)
	deadline := time.Now().Add(time.Second)
	allReady := func() bool {
		for _, sel := range []mediaget.Selection{{Kind: mediaget.Video}, {Kind: mediaget.Audio}, {Kind: mediaget.Video, Height: 720}, {Kind: mediaget.Video, Height: 480}, {Kind: mediaget.Video, Height: 360}} {
			if !session.get(sel).ready {
				return false
			}
		}
		return true
	}
	for !allReady() && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	if !session.get(selections[1]).estimate.known {
		t.Fatal("audio cache missing")
	}
	if !session.get(mediaget.Selection{Kind: mediaget.Video, Height: 360}).ready {
		t.Fatal("quality not prefetched")
	}
	scheduleQualities(inv, session, mediaget.Info{Heights: []int{360, 720, 1080}})
	session.close()
	backend.mu.Lock()
	defer backend.mu.Unlock()
	if backend.peak > 3 || len(backend.calls) != 5 {
		t.Fatalf("peak %d calls %v", backend.peak, backend.calls)
	}
	for sel, calls := range backend.calls {
		if calls != 1 {
			t.Fatalf("duplicate %+v: %d", sel, calls)
		}
	}
	if !strings.Contains(session.labels(selections, []string{"Video", "Audio"})[1], "4.0 KiB") {
		t.Fatal("completed cache lost")
	}
}

type immediateLiveMenu struct {
	selected int
	seen     []string
}

func (*immediateLiveMenu) Text(context.Context, core.Prompt) (string, error) {
	return "", errors.New("unexpected text")
}
func (*immediateLiveMenu) Secret(context.Context, core.Prompt) (string, error) {
	return "", errors.New("unexpected secret")
}
func (*immediateLiveMenu) Confirm(context.Context, core.Prompt) (bool, error) {
	return false, errors.New("unexpected confirm")
}
func (m *immediateLiveMenu) SelectLive(_ context.Context, _ string, _ []string, _ bool, snapshot func() []string) (int, error) {
	m.seen = snapshot()
	return m.selected, nil
}

func TestMenuRemainsSelectableWhileEstimatesAreBlocked(t *testing.T) {
	backend := &previewBackend{started: make(chan struct{}, 16), release: make(chan struct{})}
	session := newEarlyEstimateSession(context.Background(), mediaget.Service{Backend: backend}, mediaget.Source{URL: "https://example.invalid"})
	defer session.close()
	menu := &immediateLiveMenu{selected: 1}
	inv := &core.Invocation{Context: context.Background(), Interaction: menu}
	done := make(chan error, 1)
	go func() {
		index, err := chooseLive(inv, "Kind", func() []string {
			return session.labels([]mediaget.Selection{{Kind: mediaget.Video}, {Kind: mediaget.Audio}}, []string{"Video", "Audio"})
		}, false)
		if index != 1 {
			err = errors.New("wrong selection")
		}
		done <- err
	}()
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(time.Second):
		t.Fatal("menu blocked on estimates")
	}
	if !strings.Contains(menu.seen[1], "calculando") {
		t.Fatal(menu.seen)
	}
	session.close()
	backend.mu.Lock()
	defer backend.mu.Unlock()
	if backend.active != 0 {
		t.Fatal("prefetch survived cancellation")
	}
}
