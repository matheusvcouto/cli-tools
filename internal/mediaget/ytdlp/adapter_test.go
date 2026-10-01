package ytdlp

import (
	"bytes"
	"github.com/matheusvcouto/cli-tools/internal/mediaget"
	"strings"
	"testing"
)

func TestDecodeFormatsAndTracks(t *testing.T) {
	data := []byte(`{"title":"Synthetic","duration":8,"filesize":900,"formats":[{"height":1080},{"height":720},{"height":1080}],"requested_formats":[{"filesize":100},{"tbr":1}],"subtitles":{"pt":[{"name":"Português"}]},"automatic_captions":{"pt":[{"name":"Auto"}],"en-orig":[{"name":"English"}]}}`)
	info, err := decodeInfo(data)
	if err != nil {
		t.Fatal(err)
	}
	if info.Title != "Synthetic" || info.Size.Bytes != 900 || len(info.Parts) != 2 || len(info.Heights) != 2 || info.Heights[0] != 1080 || len(info.Tracks) != 2 {
		t.Fatalf("%+v", info)
	}
	if !info.Tracks[0].Auto || info.Tracks[1].Auto {
		t.Fatalf("%+v", info.Tracks)
	}
}
func TestInvalidJSONDoesNotEchoSensitiveChildData(t *testing.T) {
	_, err := decodeInfo([]byte("https://secret.invalid?token=SYNTHETIC"))
	if err == nil || strings.Contains(err.Error(), "SYNTHETIC") {
		t.Fatalf("%v", err)
	}
}
func TestBoundedMetadataWriter(t *testing.T) {
	var b bytes.Buffer
	w := boundedWriter{w: &b, remaining: 4}
	if n, err := w.Write([]byte("1234")); n != 4 || err != nil {
		t.Fatal(n, err)
	}
	if _, err := w.Write([]byte("5")); err == nil || b.Len() != 4 {
		t.Fatal("metadata limit not enforced")
	}
}
func TestProgressOnlyNumericalTemplateReachesUI(t *testing.T) {
	var events []mediaget.Progress
	p := progressWriter{notify: func(event mediaget.Progress) { events = append(events, event) }}
	p.Write([]byte("ERROR: https://secret.invalid?token=SYNTHETIC\n[download] malicious title\nMEDIA_GET_PROGRESS:100|NA|200|12.5|8\n"))
	p.Write([]byte("MEDIA_GET_PROGRESS:"))
	p.Write([]byte("200|200|NA|NA|NA\n"))
	if len(events) != 2 || events[0].Total != 200 || events[0].Downloaded != 100 || events[1].Downloaded != 200 {
		t.Fatalf("%+v", events)
	}
	for _, line := range []string{"MEDIA_GET_PROGRESS:-1|1|1|1|1", "MEDIA_GET_PROGRESS:secret", "[download] title"} {
		if _, ok := parseProgress(line); ok {
			t.Fatalf("accepted %q", line)
		}
	}
}

func TestPlaylistsAreExplicitlyRejected(t *testing.T) {
	for _, data := range []string{`{"_type":"playlist","entries":[]}`, `{"_type":"multi_video"}`} {
		if _, err := decodeInfo([]byte(data)); err == nil {
			t.Fatalf("accepted %s", data)
		}
	}
}

func TestProcessingMarkerNeverExposesChildText(t *testing.T) {
	var events []mediaget.Progress
	p := progressWriter{notify: func(event mediaget.Progress) { events = append(events, event) }}
	p.Write([]byte("MEDIA_GET_PROCESSING:secret.invalid\nMEDIA_GET_PROCESSING\n"))
	if len(events) != 1 || events[0].Stage != mediaget.Processing {
		t.Fatalf("%+v", events)
	}
	event, ok := parseProgress("MEDIA_GET_PROGRESS:100|200|NA|NaN|9223372036854775807")
	if !ok || event.Speed != 0 || event.ETA != 0 {
		t.Fatalf("%+v %t", event, ok)
	}
}
