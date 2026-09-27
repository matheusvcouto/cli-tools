package repozip

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/matheusvcouto/cli-tools/internal/safefs"
)

type ArchiveBackend interface {
	Create(context.Context, string, io.Writer, []string, *GitSnapshotMetadata, Git) error
	Verify(context.Context, io.ReaderAt, int64) error
}

type Service struct {
	Git      Git
	Archiver ArchiveBackend
}

type Result struct {
	Output string
}

func (s Service) Run(ctx context.Context, opts Options) (Result, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if err := ctx.Err(); err != nil {
		return Result{}, err
	}
	if err := ensurePublicationSupported(); err != nil {
		return Result{}, err
	}
	git := s.Git.withContext(ctx)

	sourceAbs, err := filepath.Abs(opts.Source)
	if err != nil {
		return Result{}, err
	}
	info, err := os.Stat(sourceAbs)
	if err != nil {
		return Result{}, fmt.Errorf("source does not exist or cannot be read: %w", err)
	}
	if !info.IsDir() {
		return Result{}, fmt.Errorf("source must be a directory: %s", sourceAbs)
	}

	repo, err := git.RepoRoot(sourceAbs)
	if err != nil {
		return Result{}, err
	}
	if err := git.AssertNoSubmodules(repo); err != nil {
		return Result{}, err
	}
	if err := git.AssertNoSparse(repo); err != nil {
		return Result{}, err
	}

	suffix := ""
	if opts.Suffix != "" {
		suffix, err = validateSuffix(opts.Suffix, "--suffix")
	}
	if err != nil {
		return Result{}, err
	}

	base := filepath.Base(repo)
	if opts.Name != "" {
		base, err = normalizeBaseName(opts.Name)
		if err != nil {
			return Result{}, err
		}
	}

	var outputPath, outputParent string
	if opts.Output != "" {
		outputPath, err = filepath.Abs(opts.Output)
		if err != nil {
			return Result{}, err
		}
		if !strings.EqualFold(filepath.Ext(outputPath), ".zip") {
			return Result{}, fmt.Errorf("--output must end in .zip")
		}
		outputParent, err = ensureExplicitParent(outputPath, repo)
		if err != nil {
			return Result{}, err
		}
		outputPath = filepath.Join(outputParent, filepath.Base(outputPath))
	} else {
		outputParent, err = ensureDefaultOutputParent(repo)
		if err != nil {
			return Result{}, err
		}
	}

	outputRel := ""
	if opts.Output != "" {
		outputRel = relativeIfInside(outputPath, repo)
	}

	var head string
	var dirty bool
	if opts.Git {
		head, err = git.HeadLabel(repo)
		if err != nil {
			return Result{}, err
		}
		if opts.Output == "" {
			dirty, err = git.IsDirty(repo, nil)
			if err != nil {
				return Result{}, err
			}
		}
	}

	if opts.Output == "" {
		parts := []string{base}
		if opts.Git {
			parts = append(parts, head)
			if dirty {
				parts = append(parts, "dirty")
			}
		}
		if suffix != "" {
			parts = append(parts, suffix)
		}
		outputPath = filepath.Join(outputParent, strings.Join(parts, "-")+".zip")
		outputRel = relativeIfInside(outputPath, repo)
	}
	if err := git.TrackedGuard(repo, outputRel); err != nil {
		return Result{}, err
	}

	outputRoot, err := safefs.Open(outputParent)
	if err != nil {
		return Result{}, fmt.Errorf("open output safety root: %w", err)
	}
	defer outputRoot.Close()
	finalName := filepath.Base(outputPath)
	if err := validateDestinationRoot(outputRoot, finalName, opts.Force); err != nil {
		return Result{}, err
	}

	var initialToken []byte
	var gitMeta *GitSnapshotMetadata
	if opts.Git {
		initialToken, err = git.StatusToken(repo, []string{outputRel})
		if err != nil {
			return Result{}, err
		}
		meta, metaErr := git.SnapshotMetadata(repo, []string{outputRel})
		if metaErr != nil {
			return Result{}, metaErr
		}
		gitMeta = &meta
	}

	files, err := git.ListFiles(repo, outputRel)
	if err != nil {
		return Result{}, err
	}
	if opts.Git {
		for _, rel := range files {
			slash := filepath.ToSlash(rel)
			if slash == ".repo-zip" || strings.HasPrefix(slash, ".repo-zip/") {
				return Result{}, fmt.Errorf("--git reserves .repo-zip/ inside the archive; repository path conflicts with that namespace: %s", rel)
			}
		}
	}
	if len(files) == 0 && !opts.Git {
		return Result{}, fmt.Errorf("no eligible files to archive")
	}

	random, err := randomToken(12)
	if err != nil {
		return Result{}, err
	}
	tempName := "." + finalName + ".repo-zip-" + random + ".tmp.zip"
	tempPath := filepath.Join(outputParent, tempName)
	tempRel := relativeIfInside(tempPath, repo)
	temp, err := outputRoot.OpenFile(tempName, os.O_CREATE|os.O_EXCL|os.O_RDWR, 0o600)
	if err != nil {
		return Result{}, fmt.Errorf("create temporary archive: %w", err)
	}
	tempPublished := false
	defer func() {
		_ = temp.Close()
		if !tempPublished {
			_ = outputRoot.Remove(tempName)
		}
	}()

	if err := s.Archiver.Create(ctx, repo, temp, files, gitMeta, git); err != nil {
		return Result{}, err
	}
	if err := temp.Sync(); err != nil {
		return Result{}, fmt.Errorf("sync temporary ZIP: %w", err)
	}
	stat, err := temp.Stat()
	if err != nil {
		return Result{}, fmt.Errorf("inspect temporary ZIP: %w", err)
	}
	if err := s.Archiver.Verify(ctx, temp, stat.Size()); err != nil {
		return Result{}, fmt.Errorf("generated ZIP failed verification: %w", err)
	}

	if opts.Git {
		finalToken, err := git.StatusToken(repo, []string{outputRel, tempRel})
		if err != nil {
			return Result{}, err
		}
		if !bytes.Equal(initialToken, finalToken) {
			return Result{}, fmt.Errorf("repository state changed during --git snapshot; temporary ZIP discarded")
		}
	}

	finalFiles, err := git.ListFiles(repo, outputRel, tempRel)
	if err != nil {
		return Result{}, err
	}
	if !slices.Equal(files, finalFiles) {
		return Result{}, fmt.Errorf("repository file set changed during snapshot; temporary ZIP discarded")
	}

	if err := git.TrackedGuard(repo, outputRel); err != nil {
		return Result{}, err
	}
	if err := ctx.Err(); err != nil {
		return Result{}, err
	}
	if err := validateDestinationRoot(outputRoot, finalName, opts.Force); err != nil {
		return Result{}, err
	}
	if err := publishArchive(outputRoot, tempName, finalName, opts.Force); err != nil {
		return Result{}, err
	}
	tempPublished = true

	published, err := outputRoot.Lstat(finalName)
	if err != nil {
		return Result{}, fmt.Errorf("published ZIP is missing: %w", err)
	}
	if !published.Mode().IsRegular() || published.Mode()&os.ModeSymlink != 0 {
		return Result{}, fmt.Errorf("published ZIP is not a regular file")
	}
	if d, err := outputRoot.Open("."); err == nil {
		_ = d.Sync()
		_ = d.Close()
	}
	return Result{Output: outputPath}, nil
}

func validateDestinationRoot(root *safefs.Root, name string, force bool) error {
	info, err := root.Lstat(name)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	if !info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("destination exists and is not a regular file: %s", name)
	}
	if !force {
		return fmt.Errorf("destination already exists: %s (use --force to replace)", name)
	}
	return nil
}

func randomToken(n int) (string, error) {
	buf := make([]byte, n)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return hex.EncodeToString(buf), nil
}
