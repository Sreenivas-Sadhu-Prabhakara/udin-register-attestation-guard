package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/sreenivas-sadhu-prabhakara/udin-register-attestation-guard/internal/udin"
)

// dataFileFormat names the file so that a register opened in a text editor in
// three years' time says what wrote it.
const dataFileFormat = "udin-register"

// dataFileVersion is bumped only if the shape of the file changes in a way an
// older build could not read.
const dataFileVersion = 1

// dataFile is what sits on disk: a header and the register.
type dataFile struct {
	Format   string         `json:"format"`
	Version  int            `json:"version"`
	SavedOn  udin.Date      `json:"savedOn"`
	Register *udin.Register `json:"register"`
	Note     string         `json:"note"`
}

// Store keeps one register in memory and on disk. Every write replaces the file
// atomically, so a crash or a full disk leaves either the previous register or
// the new one, never half of either.
type Store struct {
	mu       sync.RWMutex
	path     string
	register *udin.Register
}

// OpenStore reads the register at path, or starts an empty one if there is no
// file there yet. A file that exists but cannot be read is an error rather than
// something to overwrite: a practitioner's register is not a cache.
func OpenStore(path string) (*Store, error) {
	absolute, err := filepath.Abs(path)
	if err != nil {
		return nil, fmt.Errorf("could not work out where %s is: %w", path, err)
	}

	store := &Store{path: absolute, register: udin.New()}

	contents, err := os.ReadFile(absolute)
	switch {
	case errors.Is(err, os.ErrNotExist):
		return store, nil
	case err != nil:
		return nil, fmt.Errorf("could not read %s: %w", absolute, err)
	}
	if len(contents) == 0 {
		return store, nil
	}

	var file dataFile
	if err := json.Unmarshal(contents, &file); err != nil {
		return nil, fmt.Errorf("%s is not a register this tool wrote: %w", absolute, err)
	}
	if file.Register != nil {
		store.register = file.Register
	} else {
		// A file holding a bare register, hand-written or produced by an
		// earlier layout, is still readable.
		var bare udin.Register
		if err := json.Unmarshal(contents, &bare); err != nil {
			return nil, fmt.Errorf("%s has no register in it", absolute)
		}
		store.register = &bare
	}
	store.register.Normalise()
	return store, nil
}

// Path is where the register is kept, shown at startup and in the interface so
// nobody has to guess which file holds their data.
func (s *Store) Path() string { return s.path }

// Read runs fn against the register under a read lock. fn must not keep the
// pointer.
func (s *Store) Read(fn func(*udin.Register)) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	fn(s.register)
}

// Write applies a change and saves it.
//
// The change is made against a copy. If fn returns an error, or the file cannot
// be written, the register in memory is exactly as it was: a rejected entry
// cannot leave the register half-edited.
func (s *Store) Write(fn func(*udin.Register) error) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	working, err := cloneRegister(s.register)
	if err != nil {
		return err
	}
	if err := fn(working); err != nil {
		return err
	}
	working.Normalise()
	if err := s.save(working); err != nil {
		return err
	}
	s.register = working
	return nil
}

func (s *Store) save(register *udin.Register) error {
	file := dataFile{
		Format:   dataFileFormat,
		Version:  dataFileVersion,
		SavedOn:  udin.Today(),
		Register: register,
		Note:     "Kept by the firm that signed these documents. This file never leaves this machine. UDINs are generated only on ICAI's portal.",
	}
	encoded, err := json.MarshalIndent(file, "", "  ")
	if err != nil {
		return fmt.Errorf("could not encode the register: %w", err)
	}
	encoded = append(encoded, '\n')

	dir := filepath.Dir(s.path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("could not create %s: %w", dir, err)
	}

	temp, err := os.CreateTemp(dir, ".udin-register-*.tmp")
	if err != nil {
		return fmt.Errorf("could not open a temporary file next to %s: %w", s.path, err)
	}
	tempName := temp.Name()
	defer func() {
		// Nothing to clean up on the happy path: the rename has moved it.
		_ = os.Remove(tempName)
	}()

	if _, err := temp.Write(encoded); err != nil {
		temp.Close()
		return fmt.Errorf("could not write the register: %w", err)
	}
	if err := temp.Sync(); err != nil {
		temp.Close()
		return fmt.Errorf("could not flush the register to disk: %w", err)
	}
	if err := temp.Close(); err != nil {
		return fmt.Errorf("could not close the register: %w", err)
	}
	// A register holds client names and figures, so it is readable by its
	// owner and nobody else.
	if err := os.Chmod(tempName, 0o600); err != nil {
		return fmt.Errorf("could not set the permissions on the register: %w", err)
	}
	if err := os.Rename(tempName, s.path); err != nil {
		return fmt.Errorf("could not replace %s: %w", s.path, err)
	}
	return nil
}

func cloneRegister(register *udin.Register) (*udin.Register, error) {
	encoded, err := json.Marshal(register)
	if err != nil {
		return nil, fmt.Errorf("could not copy the register: %w", err)
	}
	var clone udin.Register
	if err := json.Unmarshal(encoded, &clone); err != nil {
		return nil, fmt.Errorf("could not copy the register: %w", err)
	}
	return &clone, nil
}
