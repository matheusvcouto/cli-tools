package repozip

import (
	"archive/zip"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/matheusvcouto/cli-tools/internal/safefs"
)

type Archiver struct {
	Git Git
}

// Create writes a complete ZIP to out. The caller owns out and is responsible
// for syncing/closing it. Paths read from repo are confined by safefs.Root.
func (a Archiver) Create(repo string, out io.Writer, files []string, gitMeta *GitSnapshotMetadata) error {
	root, err := safefs.Open(repo)
	if err != nil {
		return fmt.Errorf("open repository safety root: %w", err)
	}
	defer root.Close()

	zw := zip.NewWriter(out)
	sort.Strings(files)
	for _, rel := range files {
		if err := addPathToZip(zw, root, rel, rel); err != nil {
			_ = zw.Close()
			return err
		}
	}
	if gitMeta != nil {
		if err := addGitBundle(zw, repo, a.Git, *gitMeta); err != nil {
			_ = zw.Close()
			return err
		}
	}
	if err := zw.Close(); err != nil {
		return fmt.Errorf("finalize zip: %w", err)
	}
	return nil
}

func addPathToZip(zw *zip.Writer, root *safefs.Root, rel, archiveName string) error {
	rel = filepath.ToSlash(rel)
	before, err := root.Lstat(rel)
	if err != nil {
		return fmt.Errorf("inspect %q before archiving: %w", rel, err)
	}
	name := filepath.ToSlash(archiveName)
	if strings.HasPrefix(name, "/") || name == ".." || strings.HasPrefix(name, "../") {
		return fmt.Errorf("unsafe archive path: %q", name)
	}

	if before.Mode()&os.ModeSymlink != 0 {
		target, err := root.Readlink(rel)
		if err != nil {
			return fmt.Errorf("read symlink %q: %w", rel, err)
		}
		after, err := root.Lstat(rel)
		if err != nil {
			return err
		}
		if after.Mode()&os.ModeSymlink == 0 || !os.SameFile(before, after) {
			return fmt.Errorf("symlink changed while archiving: %s", rel)
		}
		h := &zip.FileHeader{Name: name, Method: zip.Store}
		h.SetMode(before.Mode())
		h.Modified = before.ModTime()
		w, err := zw.CreateHeader(h)
		if err != nil {
			return err
		}
		_, err = io.WriteString(w, target)
		return err
	}
	if !before.Mode().IsRegular() {
		return fmt.Errorf("unsupported file type: %s", rel)
	}
	f, err := root.Open(rel)
	if err != nil {
		return fmt.Errorf("open %q: %w", rel, err)
	}
	defer f.Close()
	afterOpen, err := f.Stat()
	if err != nil {
		return err
	}
	if !os.SameFile(before, afterOpen) {
		return fmt.Errorf("file changed identity while opening: %s", rel)
	}
	if !sameRegularSnapshot(before, afterOpen) {
		return fmt.Errorf("file changed metadata while opening: %s", rel)
	}
	h, err := zip.FileInfoHeader(before)
	if err != nil {
		return err
	}
	h.Name = name
	h.Method = zip.Deflate
	w, err := zw.CreateHeader(h)
	if err != nil {
		return err
	}
	if _, err := io.Copy(w, f); err != nil {
		return fmt.Errorf("archive %q: %w", rel, err)
	}
	afterRead, err := f.Stat()
	if err != nil {
		return err
	}
	afterPath, err := root.Lstat(rel)
	if err != nil {
		return err
	}
	if !os.SameFile(before, afterRead) || !os.SameFile(afterRead, afterPath) || !sameRegularSnapshot(before, afterRead) || !sameRegularSnapshot(afterRead, afterPath) {
		return fmt.Errorf("file changed while archiving: %s", rel)
	}
	return nil
}

func sameRegularSnapshot(a, b os.FileInfo) bool {
	return a.Mode() == b.Mode() && a.Size() == b.Size() && a.ModTime().Equal(b.ModTime())
}

const (
	gitBundleEntry   = ".repo-zip/repository.bundle"
	gitMetadataEntry = ".repo-zip/metadata.json"
)

func addGitBundle(zw *zip.Writer, repo string, git Git, meta GitSnapshotMetadata) error {
	metaRaw, err := json.MarshalIndent(meta, "", "  ")
	if err != nil {
		return fmt.Errorf("encode Git snapshot metadata: %w", err)
	}
	metaRaw = append(metaRaw, '\n')
	mh := &zip.FileHeader{Name: gitMetadataEntry, Method: zip.Deflate}
	mh.SetMode(0o644)
	mw, err := zw.CreateHeader(mh)
	if err != nil {
		return fmt.Errorf("create Git metadata ZIP entry: %w", err)
	}
	if _, err := mw.Write(metaRaw); err != nil {
		return fmt.Errorf("write Git metadata ZIP entry: %w", err)
	}

	tmp, err := os.CreateTemp("", "repo-zip-*.bundle")
	if err != nil {
		return fmt.Errorf("create temporary Git bundle: %w", err)
	}
	tmpPath := tmp.Name()
	defer func() {
		_ = tmp.Close()
		_ = os.Remove(tmpPath)
	}()
	if err := tmp.Chmod(0o600); err != nil {
		return fmt.Errorf("secure temporary Git bundle: %w", err)
	}
	if err := git.WriteBundle(repo, tmp); err != nil {
		return err
	}
	if err := tmp.Sync(); err != nil {
		return fmt.Errorf("sync temporary Git bundle: %w", err)
	}
	if err := git.VerifyBundle(repo, tmpPath); err != nil {
		return err
	}
	if _, err := tmp.Seek(0, io.SeekStart); err != nil {
		return fmt.Errorf("rewind temporary Git bundle: %w", err)
	}
	bh := &zip.FileHeader{Name: gitBundleEntry, Method: zip.Store}
	bh.SetMode(0o600)
	bw, err := zw.CreateHeader(bh)
	if err != nil {
		return fmt.Errorf("create Git bundle ZIP entry: %w", err)
	}
	if _, err := io.Copy(bw, tmp); err != nil {
		return fmt.Errorf("write Git bundle ZIP entry: %w", err)
	}
	return nil
}

// Verify validates every entry from a ReaderAt. This lets the service verify
// the exact temporary file descriptor it created instead of reopening a path.
func (Archiver) Verify(src io.ReaderAt, size int64) error {
	zr, err := zip.NewReader(src, size)
	if err != nil {
		return fmt.Errorf("open generated zip: %w", err)
	}
	seen := make(map[string]struct{}, len(zr.File))
	for _, f := range zr.File {
		name := f.Name
		if name == "" || strings.HasPrefix(name, "/") || name == ".." || strings.HasPrefix(name, "../") || strings.Contains(name, "\\") {
			return fmt.Errorf("unsafe entry in generated zip: %q", name)
		}
		if _, ok := seen[name]; ok {
			return fmt.Errorf("duplicate entry in generated zip: %q", name)
		}
		seen[name] = struct{}{}
		mode := f.Mode()
		if !mode.IsRegular() && mode&os.ModeSymlink == 0 {
			return fmt.Errorf("unsupported entry type in generated zip: %q", name)
		}
		r, err := f.Open()
		if err != nil {
			return fmt.Errorf("open zip entry %q: %w", name, err)
		}
		_, copyErr := io.Copy(io.Discard, r)
		closeErr := r.Close()
		if copyErr != nil {
			return fmt.Errorf("verify zip entry %q: %w", name, copyErr)
		}
		if closeErr != nil {
			return fmt.Errorf("close zip entry %q: %w", name, closeErr)
		}
	}
	if len(zr.File) == 0 {
		return errors.New("generated zip is empty")
	}
	return nil
}

// VerifyPath is a test/tooling convenience. Production publication uses
// Verify on the already-open temporary file descriptor.
func (a Archiver) VerifyPath(path string) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		return err
	}
	return a.Verify(f, info.Size())
}
