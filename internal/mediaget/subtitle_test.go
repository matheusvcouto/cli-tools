package mediaget

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSubtitleTextValidatesCuesAndKeepsSpokenNumbers(t *testing.T) {
	input := "\ufeff1\r\n00:00:01,000 --> 00:00:03,000\r\n<i>Hello &amp; welcome</i>\r\n\r\n2\r\n00:00:02,000 --> 00:00:04,000\r\nHello &amp; welcome everyone\r\n\r\n3\r\n00:00:05,000 --> 00:00:06,000\r\n123\r\n"
	got, err := SubtitleText([]byte(input))
	if err != nil || got != "Hello & welcome\neveryone\n123\n" {
		t.Fatalf("%q %v", got, err)
	}
	for _, bad := range []string{"", "1\nno timestamp\ntext", "1\n00:99:00,000 --> 00:99:01,000\ntext", "1\n00:00:03,000 --> 00:00:01,000\ntext", "1\n00:00:01,000 --> 00:00:02,000\n<i></i>", string([]byte{255})} {
		if _, err := SubtitleText([]byte(bad)); err == nil {
			t.Fatalf("accepted %q", bad)
		}
	}
	if _, err := SubtitleText([]byte(strings.Repeat("a", maxSubtitleBytes+1))); err == nil {
		t.Fatal("size limit")
	}
}
func TestTXTIsPublishedWithoutTimingAndDoesNotOverwrite(t *testing.T) {
	dir := t.TempDir()
	existing := filepath.Join(dir, "caption.txt")
	os.WriteFile(existing, []byte("keep"), 0600)
	b := &fakeBackend{write: func(_ Request, work string) error {
		return os.WriteFile(filepath.Join(work, "media.en.srt"), []byte("1\n00:00:01,000 --> 00:00:02,000\nHello\n"), 0600)
	}}
	req := request(dir)
	req.Name = "caption.txt"
	req.Selection = Selection{Kind: Subtitle, Track: Track{Lang: "en"}}
	result, err := (Service{Backend: b}).Download(context.Background(), req, nil)
	if err != nil || filepath.Base(result.Path) != "caption (1).txt" {
		t.Fatalf("%+v %v", result, err)
	}
	got, _ := os.ReadFile(result.Path)
	if string(got) != "Hello\n" {
		t.Fatalf("%q", got)
	}
	kept, _ := os.ReadFile(existing)
	if string(kept) != "keep" {
		t.Fatal("overwritten")
	}
}
func TestOutputRequestsFailClosed(t *testing.T) {
	for _, sel := range []Selection{{Kind: Video, VideoFormat: "avi"}, {Kind: Audio, VideoFormat: "mp4"}, {Kind: Subtitle, SubtitleFormat: "exe", Track: Track{Lang: "en"}}} {
		b := &fakeBackend{write: writeMedia}
		req := request(t.TempDir())
		req.Selection = sel
		if _, err := (Service{Backend: b}).Download(context.Background(), req, nil); err == nil || b.calls != 0 {
			t.Fatalf("%+v %v", sel, err)
		}
	}
	b := &fakeBackend{write: func(_ Request, d string) error {
		return os.WriteFile(filepath.Join(d, "media.mkv"), []byte("synthetic"), 0600)
	}}
	req := request(t.TempDir())
	req.Name = "example.MP4"
	req.KeepIncomplete = true
	result, err := (Service{Backend: b}).Download(context.Background(), req, nil)
	if err == nil || result.Path != "" || result.Incomplete == nil {
		t.Fatalf("%+v %v", result, err)
	}
}
