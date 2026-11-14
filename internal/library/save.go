package library

import (
	"crypto/sha256"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/samling/command-snippets/internal/models"
	"github.com/samling/command-snippets/internal/templating"
)

func (l *Library) hook(stage, path string) error {
	if l.saveHook != nil {
		return l.saveHook(stage, path)
	}
	return nil
}
func checkUnchanged(source *Source, path string) error {
	parent := filepath.Dir(path)
	canonicalParent, err := filepath.EvalSymlinks(parent)
	if err != nil {
		return err
	}
	if canonicalParent != parent {
		return fmt.Errorf("refusing a source beneath a symlink: %s", path)
	}
	info, err := os.Lstat(path)
	if source == nil || source.Info == nil {
		if os.IsNotExist(err) {
			return nil
		}
		if err != nil {
			return err
		}
		return fmt.Errorf("destination appeared after loading: %s; reload before saving", path)
	}
	if err != nil {
		return fmt.Errorf("source changed since loading: %s: %w", path, err)
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() || !os.SameFile(source.Info, info) || info.Mode() != source.Info.Mode() {
		return fmt.Errorf("source identity/permissions changed: %s; reload before saving", path)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	if sha256.Sum256(data) != source.Hash {
		return fmt.Errorf("source changed since loading: %s; reload before saving", path)
	}
	return nil
}

// commit replaces only the owned source. A post-replacement error is explicitly
// distinguished so callers refresh their snapshot and never blindly retry.
func (l *Library) commit(source *Source, path string, data []byte) (committed bool, err error) {
	if source != nil && source.Symlink {
		return false, fmt.Errorf("symlink sources are read-only: %s", source.DisplayPath)
	}
	if source != nil && source.Info != nil && source.Info.Mode().Perm()&0222 == 0 {
		return false, fmt.Errorf("source is read-only: %s", path)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return false, err
	}
	if err := checkUnchanged(source, path); err != nil {
		return false, err
	}
	lockPath := path + ".cs.lock"
	lock, err := os.OpenFile(lockPath, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		return false, fmt.Errorf("cannot acquire source lock %s: %w; inspect stale locks manually", lockPath, err)
	}
	defer func() {
		if cleanupErr := os.Remove(lockPath); cleanupErr != nil {
			err = errors.Join(err, fmt.Errorf("remove owned lock: %w", cleanupErr))
		}
	}()
	_, writeErr := fmt.Fprintf(lock, "pid=%d\n", os.Getpid())
	closeErr := lock.Close()
	if writeErr != nil || closeErr != nil {
		return false, errors.Join(writeErr, closeErr)
	}
	if err := checkUnchanged(source, path); err != nil {
		return false, err
	}
	temp, err := os.CreateTemp(filepath.Dir(path), ".cs-save-*")
	if err != nil {
		return false, err
	}
	tempPath := temp.Name()
	defer func() {
		if cleanupErr := os.Remove(tempPath); cleanupErr != nil && !os.IsNotExist(cleanupErr) {
			err = errors.Join(err, fmt.Errorf("remove save temp: %w", cleanupErr))
		}
	}()
	closed := false
	defer func() {
		if !closed {
			err = errors.Join(err, temp.Close())
		}
	}()
	if err := l.hook("temp-write", path); err != nil {
		return false, err
	}
	if _, err := temp.Write(data); err != nil {
		return false, err
	}
	mode := os.FileMode(0600)
	if source != nil && source.Info != nil {
		mode = source.Info.Mode().Perm()
	}
	if err := temp.Chmod(mode); err != nil {
		return false, err
	}
	if err := l.hook("temp-sync", path); err != nil {
		return false, err
	}
	if err := temp.Sync(); err != nil {
		return false, err
	}
	if err := l.hook("temp-close", path); err != nil {
		return false, err
	}
	closeErr = temp.Close()
	closed = true
	if closeErr != nil {
		return false, closeErr
	}
	if err := l.hook("before-replace", path); err != nil {
		return false, err
	}
	if err := checkUnchanged(source, path); err != nil {
		return false, err
	}
	if err := l.hook("replace", path); err != nil {
		return false, err
	}
	if source == nil || source.Info == nil {
		// Link is an atomic, no-clobber publication on the same filesystem.
		if err := os.Link(tempPath, path); err != nil {
			return false, err
		}
	} else {
		if err := replaceFile(tempPath, path); err != nil {
			return false, err
		}
	}
	committed = true
	if err := l.hook("directory-sync", path); err != nil {
		return true, fmt.Errorf("saved, but directory durability is uncertain: %w", err)
	}
	if err := syncDirectory(filepath.Dir(path)); err != nil {
		return true, fmt.Errorf("saved, but directory durability is uncertain: %w", err)
	}
	return true, nil
}
func (l *Library) Save(entry *Entry, draft models.Snippet, destination string) (*Library, *Entry, error) {
	if err := l.Error(); err != nil {
		return nil, nil, fmt.Errorf("repair/reload the library before saving: %w", err)
	}
	ordinal := -1
	if entry != nil {
		if destination != "" && ExpandPath(destination, filepath.Dir(l.ConfigPath)) != entry.Source.Path {
			return nil, nil, fmt.Errorf("editing cannot move a command to another source")
		}
		destination = entry.Source.Path
		ordinal = entry.Ordinal
		draft.ID = entry.Snippet.ID
	}
	if destination == "" {
		destination = l.Settings.DefaultSource
		if destination == "" {
			destination = l.ConfigPath
		}
	}
	destination = ExpandPath(destination, filepath.Dir(l.ConfigPath))
	if !l.AllowedDestination(destination) {
		return nil, nil, fmt.Errorf("destination is not configured: %s", destination)
	}
	if draft.ID == "" {
		id, err := models.NewID()
		if err != nil {
			return nil, nil, err
		}
		draft.ID = id
	}
	if _, err := templating.Compile(draft); err != nil {
		return nil, nil, err
	}
	source := l.Source(destination)
	original := source
	if source == nil {
		kind := "included"
		if destination == l.ConfigPath {
			kind = "main"
		}
		if destination == filepath.Join(l.CWD, ".csnippets") {
			kind = "project"
		}
		var err error
		source, err = parseSource(destination, kind, []byte("snippets: []\n"), nil)
		if err != nil {
			return nil, nil, err
		}
	}
	data, err := entryBytes(source, ordinal, draft)
	if err != nil {
		return nil, nil, err
	}
	candidate := load(l.ConfigPath, l.CWD, map[string][]byte{destination: data})
	if err := candidate.Error(); err != nil {
		return nil, nil, fmt.Errorf("candidate library is invalid: %w", err)
	}
	if _, err := candidate.Find("", draft.ID); err != nil {
		return nil, nil, fmt.Errorf("candidate is not discoverable: %w", err)
	}
	committed, err := l.commit(original, destination, data)
	if !committed {
		return nil, nil, err
	}
	next := Load(l.ConfigPath, l.CWD)
	saved, findErr := next.Find("", draft.ID)
	if findErr != nil {
		return next, nil, errors.Join(err, fmt.Errorf("saved; reload failed: %w", findErr))
	}
	return next, saved, err
}
func (l *Library) SaveSettings(settings models.Settings) (*Library, error) {
	if err := settings.Validate(); err != nil {
		return nil, err
	}
	source := l.Source(l.ConfigPath)
	if source == nil || source.Root == nil {
		return nil, fmt.Errorf("main configuration is not parseable; repair it externally before editing settings")
	}
	data, err := settingsBytes(source, settings)
	if err != nil {
		return nil, err
	}
	candidate := load(l.ConfigPath, l.CWD, map[string][]byte{source.Path: data})
	if err := candidate.Error(); err != nil {
		return nil, fmt.Errorf("candidate settings leave the library invalid: %w", err)
	}
	committed, err := l.commit(source, source.Path, data)
	if !committed {
		return nil, err
	}
	return Load(l.ConfigPath, l.CWD), err
}
