package main

import (
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"strings"

	"github.com/sreenivas-sadhu-prabhakara/udin-register-attestation-guard/internal/udin"
)

// maxBodyBytes caps every request body. A pasted list of two thousand
// documents is well inside this.
const maxBodyBytes = 2 << 20 // 2 MiB

// Server serves the embedded interface and the JSON API over one store.
// Everything is same-origin: the pages come out of the binary and talk to /api
// on the same loopback address. No request leaves the machine.
type Server struct {
	store *Store
	ui    fs.FS
	mux   *http.ServeMux
}

// NewServer builds the router.
func NewServer(store *Store, ui fs.FS) *Server {
	s := &Server{store: store, ui: ui, mux: http.NewServeMux()}
	s.routes()
	return s
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// A local tool still sets the headers that stop a page open in another tab
	// from reading a firm's register.
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Referrer-Policy", "no-referrer")
	w.Header().Set("Cache-Control", "no-store")
	s.mux.ServeHTTP(w, r)
}

func (s *Server) routes() {
	s.mux.Handle("GET /", http.FileServerFS(s.ui))

	s.mux.HandleFunc("GET /api/state", s.handleState)
	s.mux.HandleFunc("GET /api/board", s.handleBoard)
	s.mux.HandleFunc("GET /api/entry", s.handleGetEntry)
	s.mux.HandleFunc("GET /api/check", s.handleCheck)
	s.mux.HandleFunc("GET /api/export.csv", s.handleExportCSV)

	s.mux.HandleFunc("PUT /api/entry", s.handlePutEntry)
	s.mux.HandleFunc("DELETE /api/entry", s.handleDeleteEntry)
	s.mux.HandleFunc("POST /api/revoke", s.handleRevoke)
	s.mux.HandleFunc("POST /api/recall", s.handleRecall)
	s.mux.HandleFunc("PUT /api/settings", s.handlePutSettings)
	s.mux.HandleFunc("POST /api/paste", s.handlePaste)
}

// ---------- reads ----------

// reference is everything the interface needs in order to show a figure and,
// beside it, what that figure rests on. The interface never hard-codes a
// window or a source note of its own.
type reference struct {
	GenerationWindowDays  int    `json:"generationWindowDays"`
	EFilingWindowDays     int    `json:"eFilingWindowDays"`
	RevocationWindowHours int    `json:"revocationWindowHours"`
	WarnDays              int    `json:"warnDays"`
	GenerationSource      string `json:"generationSource"`
	EFilingSource         string `json:"eFilingSource"`
	RevocationSource      string `json:"revocationSource"`
	CountingSource        string `json:"countingSource"`
	MandateSource         string `json:"mandateSource"`
	FormatSource          string `json:"formatSource"`
	NoCheckDigit          string `json:"noCheckDigit"`
	NotAGenerator         string `json:"notAGenerator"`
	Archiving             string `json:"archiving"`
	Consequence           string `json:"consequence"`
	PortalURL             string `json:"portalUrl"`
	UDINLength            int    `json:"udinLength"`
	DescriptionMin        int    `json:"descriptionMin"`
	DescriptionMax        int    `json:"descriptionMax"`
}

func referenceValues() reference {
	defaults := udin.DefaultSettings()
	return reference{
		GenerationWindowDays:  defaults.GenerationWindowDays,
		EFilingWindowDays:     defaults.EFilingWindowDays,
		RevocationWindowHours: defaults.RevocationWindowHours,
		WarnDays:              defaults.WarnDays,
		GenerationSource:      udin.GenerationWindowSource,
		EFilingSource:         udin.EFilingWindowSource,
		RevocationSource:      udin.RevocationWindowSource,
		CountingSource:        udin.CountingSource,
		MandateSource:         udin.MandateSource,
		FormatSource:          udin.FormatSource,
		NoCheckDigit:          udin.NoCheckDigitNote,
		NotAGenerator:         udin.NotAGeneratorNote,
		Archiving:             udin.ArchivingNote,
		Consequence:           udin.ConsequenceNote,
		PortalURL:             udin.PortalURL,
		UDINLength:            udin.UDINLength,
		DescriptionMin:        udin.DescriptionMin,
		DescriptionMax:        udin.DescriptionMax,
	}
}

type categoryInfo struct {
	Key           udin.Category `json:"key"`
	Label         string        `json:"label"`
	MandatoryFrom udin.Date     `json:"mandatoryFrom"`
	DocumentTypes []string      `json:"documentTypes"`
}

type stateResponse struct {
	Today      udin.Date      `json:"today"`
	DataFile   string         `json:"dataFile"`
	FirmName   string         `json:"firmName"`
	Settings   udin.Settings  `json:"settings"`
	Reference  reference      `json:"reference"`
	Categories []categoryInfo `json:"categories"`
	Filters    []string       `json:"filters"`
	Clients    []string       `json:"clients"`
	Partners   []udin.Partner `json:"partners"`
	Entries    int            `json:"entries"`
}

func (s *Server) handleState(w http.ResponseWriter, r *http.Request) {
	out := stateResponse{
		Today:     udin.Today(),
		DataFile:  s.store.Path(),
		Reference: referenceValues(),
		Filters:   udin.AllFilters(),
		Clients:   []string{},
		Partners:  []udin.Partner{},
	}
	for _, category := range udin.AllCategories() {
		out.Categories = append(out.Categories, categoryInfo{
			Key:           category,
			Label:         category.Label(),
			MandatoryFrom: category.MandatoryFrom(),
			DocumentTypes: udin.CommonDocumentTypes(category),
		})
	}
	s.store.Read(func(register *udin.Register) {
		out.FirmName = register.FirmName
		out.Settings = register.Settings
		out.Entries = len(register.Entries)
		out.Clients = append(out.Clients, register.DistinctClients()...)
		out.Partners = append(out.Partners, register.DistinctPartners()...)
	})
	writeJSON(w, http.StatusOK, out)
}

type boardResponse struct {
	AsOf     udin.Date     `json:"asOf"`
	Filter   string        `json:"filter"`
	Query    string        `json:"query"`
	Rows     []udin.Row    `json:"rows"`
	Counts   udin.Counts   `json:"counts"`
	Shown    int           `json:"shown"`
	Settings udin.Settings `json:"settings"`
}

func (s *Server) handleBoard(w http.ResponseWriter, r *http.Request) {
	asOf, err := asOfParam(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	filter := strings.TrimSpace(r.URL.Query().Get("filter"))
	if filter == "" {
		filter = udin.FilterAll
	}
	query := r.URL.Query().Get("q")

	out := boardResponse{AsOf: asOf, Filter: filter, Query: query, Rows: []udin.Row{}}
	s.store.Read(func(register *udin.Register) {
		all := udin.Board(register, asOf)
		// The counts always describe the whole register: a filtered view must
		// not make the outstanding UDINs look fewer than they are.
		out.Counts = udin.Summarise(all)
		out.Settings = register.Settings
		out.Rows = append(out.Rows, udin.Search(udin.Filter(all, filter), query)...)
	})
	out.Shown = len(out.Rows)
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) handleGetEntry(w http.ResponseWriter, r *http.Request) {
	asOf, err := asOfParam(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	id := strings.TrimSpace(r.URL.Query().Get("id"))
	if id == "" {
		writeError(w, http.StatusBadRequest, fmt.Errorf("which entry? pass id"))
		return
	}

	var row udin.Row
	found := false
	s.store.Read(func(register *udin.Register) {
		entry, ok := register.Entry(id)
		if !ok {
			return
		}
		found = true
		row = udin.Row{Entry: entry, Assessment: udin.Assess(entry, register.Settings, asOf)}
	})
	if !found {
		writeError(w, http.StatusNotFound, fmt.Errorf("there is no entry %q in this register", id))
		return
	}
	writeJSON(w, http.StatusOK, row)
}

type checkResponse struct {
	Input    string         `json:"input"`
	UDIN     udin.UDIN      `json:"udin"`
	Valid    bool           `json:"valid"`
	Findings []udin.Finding `json:"findings"`
	Note     string         `json:"note"`
}

// handleCheck is the shape check on its own, for the interface to run as a
// UDIN is typed. It says what the syntax allows and no more: see the note it
// returns with every answer.
func (s *Server) handleCheck(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()
	raw := query.Get("udin")
	if strings.TrimSpace(raw) == "" {
		writeError(w, http.StatusBadRequest, fmt.Errorf("pass the UDIN to check"))
		return
	}

	parsed, findings := udin.ParseUDIN(raw)
	out := checkResponse{
		Input:    raw,
		UDIN:     parsed,
		Valid:    udin.ValidUDIN(raw),
		Findings: findings,
		Note:     udin.NoCheckDigitNote,
	}

	if signed := strings.TrimSpace(query.Get("signedOn")); signed != "" && out.Valid {
		signedOn, err := udin.ParseDate(signed)
		if err != nil {
			writeError(w, http.StatusBadRequest, err)
			return
		}
		window := udin.DefaultGenerationWindowDays
		s.store.Read(func(register *udin.Register) { window = register.Settings.GenerationWindowDays })
		out.Findings = append(out.Findings, udin.CrossCheck(parsed, signedOn, query.Get("mrn"), window)...)
	}
	if out.Findings == nil {
		out.Findings = []udin.Finding{}
	}
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) handleExportCSV(w http.ResponseWriter, r *http.Request) {
	asOf, err := asOfParam(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", "udin-register-"+asOf.String()+".csv"))
	// The status line has gone out with the first byte, so a write that fails
	// part-way can only stop. Nothing is lost: an export does not touch the
	// data file, and it can simply be taken again.
	s.store.Read(func(register *udin.Register) {
		_ = udin.WriteCSV(w, register, asOf)
	})
}

// ---------- writes ----------

func (s *Server) handlePutEntry(w http.ResponseWriter, r *http.Request) {
	var in udin.Entry
	if err := decodeBody(r, &in); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	today := udin.Today()

	var saved udin.Entry
	if err := s.store.Write(func(register *udin.Register) error {
		var err error
		saved, err = register.Put(in, today)
		return err
	}); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	writeRow(w, s, saved, today)
}

func (s *Server) handleDeleteEntry(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimSpace(r.URL.Query().Get("id"))
	if id == "" {
		writeError(w, http.StatusBadRequest, fmt.Errorf("which entry? pass id"))
		return
	}
	if err := s.store.Write(func(register *udin.Register) error {
		return register.Remove(id)
	}); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"removed": id})
}

type revokeRequest struct {
	ID              string     `json:"id"`
	RevokedAt       udin.Stamp `json:"revokedAt"`
	Reason          string     `json:"reason"`
	ReplacementUDIN string     `json:"replacementUdin"`
	Recalled        bool       `json:"recalled"`
}

func (s *Server) handleRevoke(w http.ResponseWriter, r *http.Request) {
	var in revokeRequest
	if err := decodeBody(r, &in); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	today := udin.Today()

	var saved udin.Entry
	if err := s.store.Write(func(register *udin.Register) error {
		var err error
		saved, err = register.Revoke(strings.TrimSpace(in.ID), udin.Revocation{
			RevokedAt:       in.RevokedAt,
			Reason:          in.Reason,
			ReplacementUDIN: in.ReplacementUDIN,
			Recalled:        in.Recalled,
		}, today)
		return err
	}); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	writeRow(w, s, saved, today)
}

type recallRequest struct {
	ID       string `json:"id"`
	Recalled bool   `json:"recalled"`
}

func (s *Server) handleRecall(w http.ResponseWriter, r *http.Request) {
	var in recallRequest
	if err := decodeBody(r, &in); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	today := udin.Today()

	var saved udin.Entry
	if err := s.store.Write(func(register *udin.Register) error {
		var err error
		saved, err = register.Recall(strings.TrimSpace(in.ID), in.Recalled)
		return err
	}); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	writeRow(w, s, saved, today)
}

type settingsRequest struct {
	FirmName string        `json:"firmName"`
	Settings udin.Settings `json:"settings"`
}

func (s *Server) handlePutSettings(w http.ResponseWriter, r *http.Request) {
	var in settingsRequest
	if err := decodeBody(r, &in); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	if err := s.store.Write(func(register *udin.Register) error {
		if err := register.SetFirmName(in.FirmName); err != nil {
			return err
		}
		return register.SetSettings(in.Settings)
	}); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	s.handleState(w, r)
}

type pasteRequest struct {
	Text     string        `json:"text"`
	Category udin.Category `json:"category"`
	Commit   bool          `json:"commit"`
}

type pasteResponse struct {
	Committed bool          `json:"committed"`
	Category  udin.Category `json:"category"`
	Read      int           `json:"read"`
	Imported  int           `json:"imported"`
	Columns   []string      `json:"columns"`
	Entries   []udin.Entry  `json:"entries"`
	Warnings  []string      `json:"warnings"`
	Counts    udin.Counts   `json:"counts"`
}

// handlePaste reads a pasted list. Without commit it only reports what it
// understood, so nothing enters the register before somebody has looked at it.
func (s *Server) handlePaste(w http.ResponseWriter, r *http.Request) {
	var in pasteRequest
	if err := decodeBody(r, &in); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}

	parsed, err := udin.ParsePaste(in.Text, in.Category)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}

	out := pasteResponse{
		Committed: in.Commit,
		Category:  in.Category,
		Read:      len(parsed.Entries),
		Columns:   parsed.Columns,
		Entries:   parsed.Entries,
		Warnings:  parsed.Warnings,
	}
	if !in.Commit {
		writeJSON(w, http.StatusOK, out)
		return
	}

	today := udin.Today()
	if err := s.store.Write(func(register *udin.Register) error {
		for _, entry := range parsed.Entries {
			if _, err := register.Put(entry, today); err != nil {
				out.Warnings = append(out.Warnings, fmt.Sprintf("%s (%s) was not added: %s",
					entry.ClientName, entry.DocumentType, err))
				continue
			}
			out.Imported++
		}
		return nil
	}); err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}

	s.store.Read(func(register *udin.Register) {
		out.Counts = udin.Summarise(udin.Board(register, today))
	})
	writeJSON(w, http.StatusOK, out)
}

// ---------- plumbing ----------

// asOfParam reads the day being asked about. The interface normally leaves it
// off and gets today, but it can be set to look ahead ("what will be overdue by
// the end of the month?") or back.
func asOfParam(r *http.Request) (udin.Date, error) {
	raw := strings.TrimSpace(r.URL.Query().Get("asOf"))
	if raw == "" {
		return udin.Today(), nil
	}
	return udin.ParseDate(raw)
}

func writeRow(w http.ResponseWriter, s *Server, entry udin.Entry, asOf udin.Date) {
	var row udin.Row
	s.store.Read(func(register *udin.Register) {
		row = udin.Row{Entry: entry, Assessment: udin.Assess(entry, register.Settings, asOf)}
	})
	writeJSON(w, http.StatusOK, row)
}

func decodeBody(r *http.Request, target any) error {
	decoder := json.NewDecoder(io.LimitReader(r.Body, maxBodyBytes))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return fmt.Errorf("could not read the request: %w", err)
	}
	return nil
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	encoder := json.NewEncoder(w)
	encoder.SetEscapeHTML(true)
	_ = encoder.Encode(payload)
}

func writeError(w http.ResponseWriter, status int, err error) {
	writeJSON(w, status, map[string]string{"error": err.Error()})
}
