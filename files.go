package main

import (
	"crypto/rand"
	"errors"
	"fmt"
	"io/fs"
	"os"
)

// openProjectDir confines names and symlinks beneath the configured root.
func openProjectDir(base, name string, create bool) (*os.Root, error) {
	if create {
		if err := os.MkdirAll(base, 0o700); err != nil {
			return nil, fmt.Errorf("create directory %q: %w", base, err)
		}
	}
	root, err := os.OpenRoot(base)
	if err != nil {
		return nil, fmt.Errorf("open directory %q: %w", base, err)
	}
	defer root.Close()
	if create {
		if err := root.Mkdir(name, 0o700); err != nil && !errors.Is(err, fs.ErrExist) {
			return nil, fmt.Errorf("create project directory %q: %w", name, err)
		}
	}
	dir, err := root.OpenRoot(name)
	if err != nil {
		return nil, fmt.Errorf("open project directory %q: %w", name, err)
	}
	return dir, nil
}

// writeFile publishes only complete, synced files. Temporary files live in the
// destination directory, so the final operation never crosses filesystems.
// Hard links provide exclusive publication for memories; snapshots replace the
// previous file with rename. A failed publication preserves the previous file.
func writeFile(dir *os.Root, name string, data []byte, replace bool) error {
	tmp := ".recall-" + rand.Text() + ".tmp"
	f, err := dir.OpenFile(tmp, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return err
	}
	defer dir.Remove(tmp)
	_, writeErr := f.Write(data)
	if writeErr == nil {
		writeErr = f.Sync()
	}
	if err := errors.Join(writeErr, f.Close()); err != nil {
		return err
	}
	if replace {
		return dir.Rename(tmp, name)
	}
	return dir.Link(tmp, name)
}
