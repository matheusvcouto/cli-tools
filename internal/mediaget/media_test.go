package mediaget

import (
	"context"
	"errors"
	"math"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

type fakeBackend struct {
	checkErr, errorOnDownload error
	write                     func(Request, string) error
	calls                     int
}

func (f *fakeBackend) Check(context.Context, Selection) error                   { return f.checkErr }
func (f *fakeBackend) Inspect(context.Context, Source, Selection) (Info, error) { return Info{}, nil }
func (f *fakeBackend) Download(_ context.Context, r Request, d string, _ func(Progress)) error {
	f.calls++
	if f.write != nil {
		if err := f.write(r, d); err != nil {
			return err
		}
	}
	return f.errorOnDownload
}
func request(dir string) Request {
	return Request{Source: Source{URL: "https://example.invalid/media"}, Selection: Selection{Kind: Video, Height: 1080}, OutputDir: dir, Name: "Título"}
}
func writeMedia(_ Request, dir string) error {
	return os.WriteFile(filepath.Join(dir, "media.mp4"), []byte("synthetic media"), 0600)
}

func TestMissingDestinationValidationIsReadOnlyAndDownloadCreatesIt(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "new", "nested")
	s := Service{Backend: &fakeBackend{write: writeMedia}}
	if err := s.ValidateDestination(dir); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(dir); !os.IsNotExist(err) {
		t.Fatal("validation mutated destination", err)
	}
	result, err := s.Download(t.Context(), request(dir), nil)
	if err != nil || filepath.Dir(result.Path) != dir {
		t.Fatal(result, err)
	}
	files, err := os.ReadDir(dir)
	if err != nil || len(files) != 1 {
		t.Fatal(files, err)
	}
}

func TestMissingDestinationIsNotCreatedForInvalidOrCanceledRequest(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "new", "nested")
	s := Service{Backend: &fakeBackend{write: writeMedia}}
	r := request(dir)
	r.Source.URL = "file:///synthetic"
	if _, err := s.Download(t.Context(), r, nil); err == nil {
		t.Fatal("invalid source accepted")
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if _, err := s.Download(ctx, request(dir), nil); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Dir(dir)); !os.IsNotExist(err) {
		t.Fatal("request created directories", err)
	}
}

func TestMissingDestinationRejectsFileAndSymlinkAncestors(t *testing.T) {
	parent := t.TempDir()
	file := filepath.Join(parent, "file")
	if err := os.WriteFile(file, []byte("keep"), 0600); err != nil {
		t.Fatal(err)
	}
	s := Service{Backend: &fakeBackend{write: writeMedia}}
	for _, path := range []string{file, filepath.Join(file, "child"), ""} {
		if err := s.ValidateDestination(path); err == nil {
			t.Fatal("invalid destination accepted", path)
		}
	}
	outside := t.TempDir()
	link := filepath.Join(parent, "link")
	if err := os.Symlink(outside, link); err != nil {
		t.Skip(err)
	}
	want := filepath.Join(link, "new", "nested")
	if err := s.ValidateDestination(want); err == nil {
		t.Fatal("symlink accepted")
	}
	if _, err := s.Download(t.Context(), request(want), nil); err == nil {
		t.Fatal("download followed symlink")
	}
	files, err := os.ReadDir(outside)
	if err != nil || len(files) != 0 {
		t.Fatal("outside modified", files, err)
	}
}

func TestEstimatedSizeRequiresEveryPart(t *testing.T) {
	cases := []struct {
		name  string
		info  Info
		want  int64
		known bool
	}{
		{"exact", Info{Size: FormatSize{Bytes: 42}}, 42, true},
		{"approx", Info{Size: FormatSize{ApproxBytes: 1024}}, 1024, true},
		{"bitrate", Info{Duration: 8, Size: FormatSize{Bitrate: 1}}, 1000, true},
		{"sum", Info{Duration: 8, Parts: []FormatSize{{Bytes: 100}, {Bitrate: 1}}}, 1100, true},
		{"partial", Info{Size: FormatSize{Bytes: 42}, Parts: []FormatSize{{Bytes: 100}, {}}}, 0, false},
		{"empty", Info{}, 0, false},
		{"overflow", Info{Size: FormatSize{Bytes: float64(math.MaxInt64)}}, 0, false},
		{"infinity", Info{Duration: math.Inf(1), Parts: []FormatSize{{Bitrate: 1}}}, 0, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			n, known := EstimatedSize(c.info)
			if n != c.want || known != c.known {
				t.Fatalf("got %d %t", n, known)
			}
		})
	}
}
func TestSafeNameAndSourceValidation(t *testing.T) {
	for _, name := range []string{"../escape", "a/b\\c", "\x1b[31m unsafe", strings.Repeat("漢", 200), "...", " "} {
		got := SafeName(name)
		if got == "" || len(got) > 180 || strings.ContainsAny(got, "/\\\x1b") {
			t.Fatalf("unsafe %q", got)
		}
	}
	for _, raw := range []string{"file:///etc/passwd", "https://u:p@example.invalid", "https://example.invalid/\n", "--exec", ""} {
		if ValidateSource(Source{URL: raw}) == nil {
			t.Fatalf("accepted %q", raw)
		}
	}
	if err := ValidateSource(Source{URL: "https://example.invalid/?a=1&b=2", Referer: "https://origin.invalid/"}); err != nil {
		t.Fatal(err)
	}
}
func TestPublishDoesNotOverwriteAndCleansGeneratedWork(t *testing.T) {
	dir := t.TempDir()
	existing := filepath.Join(dir, "Título.mp4")
	if err := os.WriteFile(existing, []byte("important"), 0600); err != nil {
		t.Fatal(err)
	}
	service := Service{Backend: &fakeBackend{write: writeMedia}}
	result, err := service.Download(context.Background(), request(dir), nil)
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Base(result.Path) != "Título (1).mp4" || result.Bytes != 15 || result.CleanupWarning != nil {
		t.Fatalf("%+v", result)
	}
	contents, err := os.ReadFile(existing)
	if err != nil || string(contents) != "important" {
		t.Fatalf("existing changed %q %v", contents, err)
	}
	entries, err := os.ReadDir(dir)
	if err != nil || len(entries) != 2 {
		t.Fatalf("work leaked %v %v", entries, err)
	}
}
func TestConcurrentPublishCreatesDistinctFiles(t *testing.T) {
	dir := t.TempDir()
	var wg sync.WaitGroup
	paths := make(chan string, 6)
	errs := make(chan error, 6)
	for i := 0; i < 6; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			result, err := (Service{Backend: &fakeBackend{write: writeMedia}}).Download(context.Background(), request(dir), nil)
			paths <- result.Path
			errs <- err
		}()
	}
	wg.Wait()
	close(paths)
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatal(err)
		}
	}
	seen := map[string]bool{}
	for path := range paths {
		if seen[path] {
			t.Fatalf("duplicate %s", path)
		}
		seen[path] = true
	}
	entries, err := os.ReadDir(dir)
	if err != nil || len(entries) != 6 {
		t.Fatalf("entries %v %v", entries, err)
	}
}
func TestFailurePreservesPartialAndCause(t *testing.T) {
	dir := t.TempDir()
	sentinel := errors.New("transfer failed")
	f := &fakeBackend{write: writeMedia, errorOnDownload: sentinel}
	req := request(dir)
	req.KeepIncomplete = true
	result, err := (Service{Backend: f}).Download(context.Background(), req, nil)
	if result.Incomplete == nil {
		t.Fatal("missing partial handle")
	}
	if !errors.Is(err, sentinel) {
		t.Fatalf("%v", err)
	}
	entries, readErr := os.ReadDir(dir)
	if readErr != nil || len(entries) != 1 || !entries[0].IsDir() {
		t.Fatalf("lost partial %v %v", entries, readErr)
	}
}
func TestPreflightFailureHasNoWrites(t *testing.T) {
	dir := t.TempDir()
	f := &fakeBackend{checkErr: errors.New("missing dependency")}
	_, err := (Service{Backend: f}).Download(context.Background(), request(dir), nil)
	if err == nil || f.calls != 0 {
		t.Fatalf("%v calls %d", err, f.calls)
	}
	entries, err := os.ReadDir(dir)
	if err != nil || len(entries) != 0 {
		t.Fatal("preflight wrote files")
	}
}
func TestRejectsSymlinkAndIncompleteOutput(t *testing.T) {
	for _, mode := range []string{"symlink", "empty", "part", "multiple"} {
		t.Run(mode, func(t *testing.T) {
			dir := t.TempDir()
			outside := filepath.Join(t.TempDir(), "important.mp4")
			if err := os.WriteFile(outside, []byte("keep"), 0600); err != nil {
				t.Fatal(err)
			}
			f := &fakeBackend{write: func(_ Request, work string) error {
				switch mode {
				case "symlink":
					if err := os.Symlink(outside, filepath.Join(work, "media.mp4")); err != nil {
						t.Skipf("symlink unavailable: %v", err)
					}
					return nil
				case "empty":
					return os.WriteFile(filepath.Join(work, "media.mp4"), nil, 0600)
				case "part":
					return os.WriteFile(filepath.Join(work, "media.part"), []byte("partial"), 0600)
				default:
					if err := writeMedia(Request{}, work); err != nil {
						return err
					}
					return os.WriteFile(filepath.Join(work, "extra.mp4"), []byte("extra"), 0600)
				}
			}}
			result, err := (Service{Backend: f}).Download(context.Background(), request(dir), nil)
			if err == nil || result.Path != "" {
				t.Fatalf("published %v %v", result, err)
			}
			data, err := os.ReadFile(outside)
			if err != nil || string(data) != "keep" {
				t.Fatal("outside file changed")
			}
		})
	}
}
func TestCancelBeforePublication(t *testing.T) {
	dir := t.TempDir()
	ctx, cancel := context.WithCancel(context.Background())
	f := &fakeBackend{write: func(r Request, work string) error { cancel(); return writeMedia(r, work) }}
	result, err := (Service{Backend: f}).Download(ctx, request(dir), nil)
	if !errors.Is(err, context.Canceled) || result.Path != "" {
		t.Fatalf("%v %v", result, err)
	}
	entries, _ := os.ReadDir(dir)
	if len(entries) != 0 {
		t.Fatal("cancel left incomplete files")
	}
}
func TestExistingSymlinkIsNeverReplaced(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(t.TempDir(), "missing")
	if err := os.Symlink(target, filepath.Join(dir, "Título.mp4")); err != nil {
		t.Skipf("symlink unavailable: %v", err)
	}
	result, err := (Service{Backend: &fakeBackend{write: writeMedia}}).Download(context.Background(), request(dir), nil)
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Base(result.Path) != "Título (1).mp4" {
		t.Fatal(result.Path)
	}
	if link, err := os.Readlink(filepath.Join(dir, "Título.mp4")); err != nil || link != target {
		t.Fatal("symlink replaced")
	}
}

func TestChangedWorkIdentityDoesNotDeleteReplacement(t *testing.T) {
	dir := t.TempDir()
	var replacement string
	f := &fakeBackend{write: func(_ Request, work string) error {
		if err := os.Rename(work, work+"-original"); err != nil {
			return err
		}
		if err := os.Mkdir(work, 0700); err != nil {
			return err
		}
		replacement = filepath.Join(work, "important.txt")
		return os.WriteFile(replacement, []byte("keep"), 0600)
	}}
	_, err := (Service{Backend: f}).Download(context.Background(), request(dir), nil)
	if err == nil || !strings.Contains(err.Error(), "mudou") {
		t.Fatalf("%v", err)
	}
	if data, err := os.ReadFile(replacement); err != nil || string(data) != "keep" {
		t.Fatal("replacement was removed")
	}
}

func TestUnsafeSubtitleIdentifierFailsBeforeAnyWrite(t *testing.T) {
	dir := t.TempDir()
	f := &fakeBackend{write: writeMedia}
	service := Service{Backend: f}
	for _, lang := range []string{"../outside", "pt,all", "pt/../../outside", "%(title)s", "\x1bpt", ".."} {
		req := request(dir)
		req.Selection = Selection{Kind: Subtitle, Track: Track{Lang: lang}}
		if _, err := service.Download(context.Background(), req, nil); err == nil {
			t.Fatalf("accepted %q", lang)
		}
	}
	if f.calls != 0 {
		t.Fatal("invalid language spawned backend")
	}
	entries, err := os.ReadDir(dir)
	if err != nil || len(entries) != 0 {
		t.Fatal("invalid language wrote files")
	}
}

func TestDefaultFailureDiscardsOnlyCurrentDownload(t *testing.T) {
	dir := t.TempDir()
	existing := filepath.Join(dir, ".media-get-existing")
	if err := os.Mkdir(existing, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(existing, "keep"), []byte("important"), 0600); err != nil {
		t.Fatal(err)
	}
	cause := errors.New("failed")
	result, err := (Service{Backend: &fakeBackend{write: writeMedia, errorOnDownload: cause}}).Download(context.Background(), request(dir), nil)
	if !errors.Is(err, cause) || result.Incomplete != nil || !strings.Contains(err.Error(), "descartados") {
		t.Fatalf("%+v %v", result, err)
	}
	entries, err := os.ReadDir(dir)
	if err != nil || len(entries) != 1 || entries[0].Name() != ".media-get-existing" {
		t.Fatalf("%v %v", entries, err)
	}
}

func TestRetainedIncompleteSizeAndSafeDiscard(t *testing.T) {
	dir := t.TempDir()
	req := request(dir)
	req.KeepIncomplete = true
	result, err := (Service{Backend: &fakeBackend{write: writeMedia, errorOnDownload: context.Canceled}}).Download(context.Background(), req, nil)
	if !errors.Is(err, context.Canceled) || result.Incomplete == nil {
		t.Fatalf("%+v %v", result, err)
	}
	partial := result.Incomplete
	if strings.HasPrefix(filepath.Base(partial.Path), ".") {
		t.Fatal("hidden incomplete")
	}
	if n, err := partial.Size(); err != nil || n != 15 {
		t.Fatalf("%d %v", n, err)
	}
	if err := partial.Discard(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Lstat(partial.Path); !errors.Is(err, os.ErrNotExist) {
		t.Fatal(err)
	}
}

func TestIncompleteDiscardRejectsReplacement(t *testing.T) {
	req := request(t.TempDir())
	req.KeepIncomplete = true
	result, _ := (Service{Backend: &fakeBackend{write: writeMedia, errorOnDownload: context.Canceled}}).Download(context.Background(), req, nil)
	partial := result.Incomplete
	if partial == nil {
		t.Fatal("missing partial")
	}
	if err := os.Rename(partial.Path, partial.Path+"-original"); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(partial.Path, 0700); err != nil {
		t.Fatal(err)
	}
	keep := filepath.Join(partial.Path, "important")
	if err := os.WriteFile(keep, []byte("keep"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := partial.Discard(); err == nil {
		t.Fatal("deleted replacement")
	}
	if data, err := os.ReadFile(keep); err != nil || string(data) != "keep" {
		t.Fatal("replacement changed")
	}
}
