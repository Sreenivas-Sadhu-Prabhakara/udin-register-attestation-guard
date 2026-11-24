package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/sreenivas-sadhu-prabhakara/udin-register-attestation-guard/internal/udin"
)

func sampleEntry() udin.Entry {
	return udin.Entry{
		Category:     udin.CategoryAuditAssurance,
		DocumentType: "Statutory audit report (Companies Act, section 143)",
		ClientName:   "Sundaram Textiles Private Limited",
		SignedOn:     udin.MustDate(2026, 9, 10),
		PartnerName:  "R. Natarajan",
		PartnerMRN:   "304576",
	}
}

func TestOpenStoreStartsEmptyWithoutTouchingTheDisk(t *testing.T) {
	path := filepath.Join(t.TempDir(), "register.json")

	store, err := OpenStore(path)
	if err != nil {
		t.Fatalf("OpenStore: %v", err)
	}
	if store.Path() != path {
		t.Fatalf("Path() = %q, want %q", store.Path(), path)
	}
	if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("opening a register that does not exist created a file")
	}

	store.Read(func(register *udin.Register) {
		if len(register.Entries) != 0 {
			t.Fatalf("a new register holds %d entries", len(register.Entries))
		}
		if register.Settings != udin.DefaultSettings() {
			t.Fatalf("a new register has settings %+v", register.Settings)
		}
	})
}

func TestWriteSavesAtomicallyAndReloads(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "register.json")

	store, err := OpenStore(path)
	if err != nil {
		t.Fatalf("OpenStore: %v", err)
	}
	if err := store.Write(func(register *udin.Register) error {
		if err := register.SetFirmName("Natarajan & Associates"); err != nil {
			return err
		}
		_, err := register.Put(sampleEntry(), udin.MustDate(2026, 11, 24))
		return err
	}); err != nil {
		t.Fatalf("Write: %v", err)
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("the register was not written: %v", err)
	}
	if mode := info.Mode().Perm(); mode != 0o600 {
		t.Fatalf("the register is saved with mode %o, want 600: it holds client names", mode)
	}

	names, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("ReadDir: %v", err)
	}
	for _, name := range names {
		if strings.HasSuffix(name.Name(), ".tmp") {
			t.Fatalf("a temporary file was left behind: %s", name.Name())
		}
	}

	contents, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	var file dataFile
	if err := json.Unmarshal(contents, &file); err != nil {
		t.Fatalf("the saved file is not readable: %v", err)
	}
	if file.Format != dataFileFormat || file.Version != dataFileVersion {
		t.Fatalf("the saved file says format %q version %d", file.Format, file.Version)
	}
	if file.Register == nil || len(file.Register.Entries) != 1 {
		t.Fatalf("the saved file holds %+v", file.Register)
	}

	reopened, err := OpenStore(path)
	if err != nil {
		t.Fatalf("reopening: %v", err)
	}
	reopened.Read(func(register *udin.Register) {
		if register.FirmName != "Natarajan & Associates" {
			t.Fatalf("the firm name came back as %q", register.FirmName)
		}
		if len(register.Entries) != 1 || register.Entries[0].ClientName != "Sundaram Textiles Private Limited" {
			t.Fatalf("the entries came back as %+v", register.Entries)
		}
		if register.NextID != 2 {
			t.Fatalf("the next identifier came back as %d", register.NextID)
		}
	})
}

func TestWriteLeavesTheRegisterAloneWhenTheChangeFails(t *testing.T) {
	path := filepath.Join(t.TempDir(), "register.json")
	store, err := OpenStore(path)
	if err != nil {
		t.Fatalf("OpenStore: %v", err)
	}
	if err := store.Write(func(register *udin.Register) error {
		_, err := register.Put(sampleEntry(), udin.MustDate(2026, 11, 24))
		return err
	}); err != nil {
		t.Fatalf("Write: %v", err)
	}

	// A change that adds one entry and then fails must leave neither behind.
	failure := errors.New("the second entry was no good")
	err = store.Write(func(register *udin.Register) error {
		second := sampleEntry()
		second.ClientName = "Half Saved Private Limited"
		if _, err := register.Put(second, udin.MustDate(2026, 11, 24)); err != nil {
			return err
		}
		return failure
	})
	if !errors.Is(err, failure) {
		t.Fatalf("Write returned %v, want the failure from the change", err)
	}

	store.Read(func(register *udin.Register) {
		if len(register.Entries) != 1 {
			t.Fatalf("the register holds %d entries after a failed change", len(register.Entries))
		}
	})

	reopened, err := OpenStore(path)
	if err != nil {
		t.Fatalf("reopening: %v", err)
	}
	reopened.Read(func(register *udin.Register) {
		if len(register.Entries) != 1 {
			t.Fatalf("the file holds %d entries after a failed change", len(register.Entries))
		}
	})
}

func TestOpenStoreRefusesToOverwriteSomethingItCannotRead(t *testing.T) {
	path := filepath.Join(t.TempDir(), "register.json")
	if err := os.WriteFile(path, []byte("this is not a register"), 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	if _, err := OpenStore(path); err == nil {
		t.Fatal("a file that is not a register was opened anyway; it would have been overwritten")
	}
}

func TestOpenStoreReadsABareRegister(t *testing.T) {
	path := filepath.Join(t.TempDir(), "register.json")
	bare := `{"firmName":"Hand Written & Co","entries":[{"id":"E-0004","category":"certificate",` +
		`"documentType":"Net worth certificate","clientName":"Anand Traders","signedOn":"2026-10-01"}],"nextId":1}`
	if err := os.WriteFile(path, []byte(bare), 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	store, err := OpenStore(path)
	if err != nil {
		t.Fatalf("OpenStore: %v", err)
	}
	store.Read(func(register *udin.Register) {
		if register.FirmName != "Hand Written & Co" || len(register.Entries) != 1 {
			t.Fatalf("the bare register came back as %+v", register)
		}
		if register.NextID != 5 {
			t.Fatalf("the identifier counter was not repaired: %d", register.NextID)
		}
		if register.Settings != udin.DefaultSettings() {
			t.Fatalf("the settings were not filled in: %+v", register.Settings)
		}
	})
}

func TestOpenStoreTreatsAnEmptyFileAsAnEmptyRegister(t *testing.T) {
	path := filepath.Join(t.TempDir(), "register.json")
	if err := os.WriteFile(path, nil, 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	store, err := OpenStore(path)
	if err != nil {
		t.Fatalf("OpenStore: %v", err)
	}
	store.Read(func(register *udin.Register) {
		if len(register.Entries) != 0 {
			t.Fatalf("an empty file produced %d entries", len(register.Entries))
		}
	})
}

func TestWritesFromSeveralPlacesAtOnceAllLand(t *testing.T) {
	path := filepath.Join(t.TempDir(), "register.json")
	store, err := OpenStore(path)
	if err != nil {
		t.Fatalf("OpenStore: %v", err)
	}

	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			entry := sampleEntry()
			entry.ClientName = fmt.Sprintf("Client %02d Private Limited", n)
			if err := store.Write(func(register *udin.Register) error {
				_, err := register.Put(entry, udin.MustDate(2026, 11, 24))
				return err
			}); err != nil {
				t.Errorf("Write %d: %v", n, err)
			}
		}(i)
	}
	wg.Wait()

	store.Read(func(register *udin.Register) {
		if len(register.Entries) != 10 {
			t.Fatalf("the register holds %d of 10 entries", len(register.Entries))
		}
		seen := map[string]bool{}
		for _, entry := range register.Entries {
			if seen[entry.ID] {
				t.Fatalf("two entries share the identifier %s", entry.ID)
			}
			seen[entry.ID] = true
		}
	})
}
