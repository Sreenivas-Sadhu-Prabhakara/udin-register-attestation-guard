package main

import (
	"bytes"
	"encoding/json"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sreenivas-sadhu-prabhakara/udin-register-attestation-guard/internal/udin"
)

func newTestServer(t *testing.T) *Server {
	t.Helper()
	store, err := OpenStore(filepath.Join(t.TempDir(), "register.json"))
	if err != nil {
		t.Fatalf("OpenStore: %v", err)
	}
	ui, err := fs.Sub(webFiles, "web")
	if err != nil {
		t.Fatalf("the interface is not embedded: %v", err)
	}
	return NewServer(store, ui)
}

func call(t *testing.T, server *Server, method, path string, payload any) *httptest.ResponseRecorder {
	t.Helper()
	var body *bytes.Reader
	if payload == nil {
		body = bytes.NewReader(nil)
	} else if raw, ok := payload.(string); ok {
		body = bytes.NewReader([]byte(raw))
	} else {
		encoded, err := json.Marshal(payload)
		if err != nil {
			t.Fatalf("encoding the request: %v", err)
		}
		body = bytes.NewReader(encoded)
	}
	request := httptest.NewRequest(method, path, body)
	if payload != nil {
		request.Header.Set("Content-Type", "application/json")
	}
	recorder := httptest.NewRecorder()
	server.ServeHTTP(recorder, request)
	return recorder
}

func decode(t *testing.T, recorder *httptest.ResponseRecorder, target any) {
	t.Helper()
	if err := json.Unmarshal(recorder.Body.Bytes(), target); err != nil {
		t.Fatalf("the response is not readable JSON (%d): %s", recorder.Code, recorder.Body.String())
	}
}

func errorMessage(t *testing.T, recorder *httptest.ResponseRecorder) string {
	t.Helper()
	var payload struct {
		Error string `json:"error"`
	}
	decode(t, recorder, &payload)
	return payload.Error
}

func newEntry() map[string]any {
	return map[string]any{
		"category":      string(udin.CategoryAuditAssurance),
		"documentType":  "Statutory audit report (Companies Act, section 143)",
		"description":   "Independent auditor's report on the financial statements for the year ended 31 March 2026",
		"clientName":    "Sundaram Textiles Private Limited",
		"clientRef":     "AABCS1234K",
		"financialYear": "2025-26",
		"signedOn":      "2026-09-10",
		"partnerName":   "R. Natarajan",
		"partnerMrn":    "304576",
		"figures": []map[string]string{
			{"keyword": "Turnover", "value": "48,20,15,000"},
			{"keyword": "Shareholders' funds", "value": "12,40,00,000"},
		},
	}
}

func TestStateCarriesTheReferenceFiguresAndTheirSources(t *testing.T) {
	server := newTestServer(t)
	recorder := call(t, server, "GET", "/api/state", nil)
	if recorder.Code != http.StatusOK {
		t.Fatalf("GET /api/state = %d: %s", recorder.Code, recorder.Body.String())
	}

	var out stateResponse
	decode(t, recorder, &out)

	if out.Reference.GenerationWindowDays != 60 || out.Reference.EFilingWindowDays != 15 ||
		out.Reference.RevocationWindowHours != 48 {
		t.Fatalf("the reference windows came back as %+v", out.Reference)
	}
	if out.Settings != udin.DefaultSettings() {
		t.Fatalf("the settings came back as %+v", out.Settings)
	}
	if len(out.Categories) != 3 || out.Categories[0].MandatoryFrom.String() != "2019-02-01" {
		t.Fatalf("the categories came back as %+v", out.Categories)
	}
	if len(out.Categories[0].DocumentTypes) == 0 {
		t.Fatal("no document type suggestions came back")
	}
	if len(out.Filters) == 0 || out.Today.IsZero() || out.DataFile == "" {
		t.Fatalf("the state is incomplete: %+v", out)
	}
	for name, note := range map[string]string{
		"generation source": out.Reference.GenerationSource,
		"counting source":   out.Reference.CountingSource,
		"revocation source": out.Reference.RevocationSource,
		"e-filing source":   out.Reference.EFilingSource,
		"mandate source":    out.Reference.MandateSource,
		"format source":     out.Reference.FormatSource,
		"check digit note":  out.Reference.NoCheckDigit,
		"not-a-generator":   out.Reference.NotAGenerator,
		"archiving note":    out.Reference.Archiving,
		"consequence note":  out.Reference.Consequence,
	} {
		if len(note) < 40 {
			t.Fatalf("the %s is missing or too short: %q", name, note)
		}
	}
	if !strings.Contains(out.Reference.NotAGenerator, "does not generate") {
		t.Fatalf("the interface is not told plainly that this tool generates nothing: %q", out.Reference.NotAGenerator)
	}
}

func TestEntryLifecycle(t *testing.T) {
	server := newTestServer(t)

	// 1. A signed document with no UDIN.
	recorder := call(t, server, "PUT", "/api/entry", newEntry())
	if recorder.Code != http.StatusOK {
		t.Fatalf("PUT /api/entry = %d: %s", recorder.Code, recorder.Body.String())
	}
	var row udin.Row
	decode(t, recorder, &row)
	if row.Entry.ID != "E-0001" {
		t.Fatalf("the entry came back as %q", row.Entry.ID)
	}
	if row.Assessment.Stage != udin.StageOverdue {
		t.Fatalf("a document signed in September with no UDIN is %q", row.Assessment.Stage)
	}
	if row.Assessment.Deadline.String() != "2026-11-08" {
		t.Fatalf("the deadline came back as %s", row.Assessment.Deadline)
	}

	// 2. The board counts it as outstanding.
	recorder = call(t, server, "GET", "/api/board", nil)
	var board boardResponse
	decode(t, recorder, &board)
	if board.Counts.Total != 1 || board.Counts.NoUDIN != 1 || board.Counts.Overdue != 1 {
		t.Fatalf("the counts came back as %+v", board.Counts)
	}

	// 3. The UDIN is generated on the portal and recorded here.
	withUDIN := newEntry()
	withUDIN["id"] = "E-0001"
	withUDIN["udin"] = "26 304576 AKTSBN1359"
	withUDIN["generatedAt"] = "2026-09-12T14:30"
	recorder = call(t, server, "PUT", "/api/entry", withUDIN)
	if recorder.Code != http.StatusOK {
		t.Fatalf("recording the UDIN = %d: %s", recorder.Code, recorder.Body.String())
	}
	decode(t, recorder, &row)
	if row.Entry.UDIN != "26304576AKTSBN1359" {
		t.Fatalf("the UDIN was stored as %q", row.Entry.UDIN)
	}
	if row.Assessment.Stage != udin.StageRecorded || row.Assessment.DaysUsed != 3 {
		t.Fatalf("the entry is %q on day %d", row.Assessment.Stage, row.Assessment.DaysUsed)
	}
	if len(row.Assessment.Flags) != 0 {
		t.Fatalf("a complete record produced findings: %+v", row.Assessment.Flags)
	}

	// 4. It cannot be removed now that it carries a number.
	recorder = call(t, server, "DELETE", "/api/entry?id=E-0001", nil)
	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("deleting a numbered entry = %d", recorder.Code)
	}
	if !strings.Contains(errorMessage(t, recorder), "stays in the register") {
		t.Fatalf("the refusal reads %q", errorMessage(t, recorder))
	}

	// 5. The UDIN is revoked, and the register keeps it.
	recorder = call(t, server, "POST", "/api/revoke", map[string]any{
		"id":        "E-0001",
		"revokedAt": "2026-09-13T09:15",
		"reason":    "The client restated a figure after the report was signed.",
	})
	if recorder.Code != http.StatusOK {
		t.Fatalf("POST /api/revoke = %d: %s", recorder.Code, recorder.Body.String())
	}
	decode(t, recorder, &row)
	if row.Assessment.Stage != udin.StageRevoked || row.Entry.UDIN == "" {
		t.Fatalf("after revocation the entry is %q with UDIN %q", row.Assessment.Stage, row.Entry.UDIN)
	}
	if row.Entry.Revocation == nil || row.Entry.Revocation.Reason == "" {
		t.Fatalf("the revocation came back as %+v", row.Entry.Revocation)
	}

	// 6. A second revocation is refused.
	recorder = call(t, server, "POST", "/api/revoke", map[string]any{
		"id":        "E-0001",
		"revokedAt": "2026-09-14",
		"reason":    "Trying again for no good reason.",
	})
	if recorder.Code != http.StatusBadRequest || !strings.Contains(errorMessage(t, recorder), "already revoked") {
		t.Fatalf("a second revocation = %d: %s", recorder.Code, recorder.Body.String())
	}

	// 7. The document is taken back out of circulation.
	recorder = call(t, server, "POST", "/api/recall", map[string]any{"id": "E-0001", "recalled": true})
	if recorder.Code != http.StatusOK {
		t.Fatalf("POST /api/recall = %d: %s", recorder.Code, recorder.Body.String())
	}
	decode(t, recorder, &row)
	if row.Entry.Revocation == nil || !row.Entry.Revocation.Recalled {
		t.Fatalf("the recall was not recorded: %+v", row.Entry.Revocation)
	}

	// 8. One entry fetched on its own.
	recorder = call(t, server, "GET", "/api/entry?id=E-0001", nil)
	if recorder.Code != http.StatusOK {
		t.Fatalf("GET /api/entry = %d", recorder.Code)
	}
	decode(t, recorder, &row)
	if row.Entry.ID != "E-0001" {
		t.Fatalf("the entry came back as %+v", row.Entry)
	}
	if recorder := call(t, server, "GET", "/api/entry?id=E-0404", nil); recorder.Code != http.StatusNotFound {
		t.Fatalf("an entry that does not exist = %d", recorder.Code)
	}
	if recorder := call(t, server, "GET", "/api/entry", nil); recorder.Code != http.StatusBadRequest {
		t.Fatalf("no identifier = %d", recorder.Code)
	}

	// 9. The export holds the revoked number and its reason.
	recorder = call(t, server, "GET", "/api/export.csv", nil)
	if recorder.Code != http.StatusOK {
		t.Fatalf("GET /api/export.csv = %d", recorder.Code)
	}
	if got := recorder.Header().Get("Content-Type"); !strings.HasPrefix(got, "text/csv") {
		t.Fatalf("the export is served as %q", got)
	}
	if got := recorder.Header().Get("Content-Disposition"); !strings.Contains(got, "udin-register-") {
		t.Fatalf("the export is named %q", got)
	}
	exported := recorder.Body.String()
	for _, want := range []string{"26304576AKTSBN1359", "The client restated a figure", "Revoked", "Sundaram Textiles"} {
		if !strings.Contains(exported, want) {
			t.Fatalf("the export does not mention %q:\n%s", want, exported)
		}
	}
}

func TestPutEntryRejectsBadInputWithAReadableMessage(t *testing.T) {
	server := newTestServer(t)
	cases := []struct {
		name    string
		change  func(map[string]any)
		wantErr string
	}{
		{name: "no client", change: func(e map[string]any) { delete(e, "clientName") }, wantErr: "client"},
		{name: "no signing date", change: func(e map[string]any) { delete(e, "signedOn") }, wantErr: "signed"},
		{name: "a bad date", change: func(e map[string]any) { e["signedOn"] = "10-09-2026" }, wantErr: "YYYY-MM-DD"},
		{name: "a bad financial year", change: func(e map[string]any) { e["financialYear"] = "2025-27" }, wantErr: "consecutive"},
		{name: "a UDIN that is not one", change: func(e map[string]any) { e["udin"] = "PENDING" }, wantErr: "not a UDIN"},
		{
			name:    "a UDIN with no generation date",
			change:  func(e map[string]any) { e["udin"] = "26304576AKTSBN1359" },
			wantErr: "date the UDIN was generated",
		},
		{name: "an unknown entry", change: func(e map[string]any) { e["id"] = "E-9999" }, wantErr: "no entry"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			entry := newEntry()
			c.change(entry)
			recorder := call(t, server, "PUT", "/api/entry", entry)
			if recorder.Code != http.StatusBadRequest {
				t.Fatalf("PUT /api/entry = %d, want 400: %s", recorder.Code, recorder.Body.String())
			}
			if got := errorMessage(t, recorder); !strings.Contains(got, c.wantErr) {
				t.Fatalf("the message is %q, which does not mention %q", got, c.wantErr)
			}
		})
	}

	// A field the API does not know is refused rather than ignored, so a typo
	// in a hand-made request cannot silently drop data.
	recorder := call(t, server, "PUT", "/api/entry", `{"clientName":"X","udinNumber":"26304576AKTSBN1359"}`)
	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("an unknown field = %d", recorder.Code)
	}
	recorder = call(t, server, "PUT", "/api/entry", `{"clientName":`)
	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("a broken body = %d", recorder.Code)
	}
}

func TestBoardFilterAndSearch(t *testing.T) {
	server := newTestServer(t)

	first := newEntry()
	first["udin"] = "26304576AKTSBN1359"
	first["generatedAt"] = "2026-09-12"
	if recorder := call(t, server, "PUT", "/api/entry", first); recorder.Code != http.StatusOK {
		t.Fatalf("PUT = %d: %s", recorder.Code, recorder.Body.String())
	}

	second := newEntry()
	second["clientName"] = "Anand Traders"
	second["documentType"] = "Turnover certificate for a bank"
	second["category"] = string(udin.CategoryCertificate)
	if recorder := call(t, server, "PUT", "/api/entry", second); recorder.Code != http.StatusOK {
		t.Fatalf("PUT = %d: %s", recorder.Code, recorder.Body.String())
	}

	cases := []struct {
		path      string
		wantShown int
	}{
		{path: "/api/board", wantShown: 2},
		{path: "/api/board?filter=no-udin", wantShown: 1},
		{path: "/api/board?filter=recorded", wantShown: 1},
		{path: "/api/board?filter=needs-attention", wantShown: 1},
		{path: "/api/board?filter=revoked", wantShown: 0},
		{path: "/api/board?q=anand", wantShown: 1},
		{path: "/api/board?q=26+304576+AKTSBN1359", wantShown: 1},
		{path: "/api/board?q=nobody", wantShown: 0},
		{path: "/api/board?filter=recorded&q=anand", wantShown: 0},
	}
	for _, c := range cases {
		t.Run(c.path, func(t *testing.T) {
			recorder := call(t, server, "GET", c.path, nil)
			if recorder.Code != http.StatusOK {
				t.Fatalf("GET %s = %d", c.path, recorder.Code)
			}
			var board boardResponse
			decode(t, recorder, &board)
			if board.Shown != c.wantShown || len(board.Rows) != c.wantShown {
				t.Fatalf("GET %s showed %d rows, want %d", c.path, board.Shown, c.wantShown)
			}
			// The counts always describe the whole register.
			if board.Counts.Total != 2 {
				t.Fatalf("GET %s counted %d documents in all", c.path, board.Counts.Total)
			}
		})
	}

	// Looking at a day of your choosing.
	recorder := call(t, server, "GET", "/api/board?asOf=2026-09-15", nil)
	var board boardResponse
	decode(t, recorder, &board)
	if board.Counts.Overdue != 0 || board.Counts.NoUDIN != 1 {
		t.Fatalf("as at 15 September the counts are %+v", board.Counts)
	}
	if recorder := call(t, server, "GET", "/api/board?asOf=15-09-2026", nil); recorder.Code != http.StatusBadRequest {
		t.Fatalf("a bad asOf = %d", recorder.Code)
	}
}

func TestCheckEndpointSaysOnlyWhatItCanTell(t *testing.T) {
	server := newTestServer(t)

	recorder := call(t, server, "GET", "/api/check?udin=19304576AKTSBN1359", nil)
	if recorder.Code != http.StatusOK {
		t.Fatalf("GET /api/check = %d", recorder.Code)
	}
	var out checkResponse
	decode(t, recorder, &out)
	if !out.Valid || out.UDIN.MembershipNumber != "304576" || len(out.Findings) != 0 {
		t.Fatalf("a well-formed UDIN came back as %+v", out)
	}
	if !strings.Contains(out.Note, "Only verification at") {
		t.Fatalf("the answer does not say what it cannot tell: %q", out.Note)
	}

	recorder = call(t, server, "GET", "/api/check?udin=PENDING", nil)
	decode(t, recorder, &out)
	if out.Valid || len(out.Findings) == 0 {
		t.Fatalf("a string that is not a UDIN came back as %+v", out)
	}

	// With the document's details, the membership number is cross-checked.
	recorder = call(t, server, "GET", "/api/check?udin=26999999AKTSBN1359&signedOn=2026-09-10&mrn=304576", nil)
	decode(t, recorder, &out)
	found := false
	for _, finding := range out.Findings {
		if finding.Code == "udin-mrn-mismatch" {
			found = true
		}
	}
	if !found {
		t.Fatalf("the membership number was not cross-checked: %+v", out.Findings)
	}

	if recorder := call(t, server, "GET", "/api/check", nil); recorder.Code != http.StatusBadRequest {
		t.Fatalf("no UDIN = %d", recorder.Code)
	}
	if recorder := call(t, server, "GET", "/api/check?udin=26304576AKTSBN1359&signedOn=nonsense", nil); recorder.Code != http.StatusBadRequest {
		t.Fatalf("a bad signing date = %d", recorder.Code)
	}
}

func TestPasteIsPreviewedBeforeItIsKept(t *testing.T) {
	server := newTestServer(t)
	list := strings.Join([]string{
		"Sundaram Textiles Private Limited\tStatutory audit report\t10-09-2026\t26304576AKTSBN1359\t12-09-2026",
		"Anand Traders\tTurnover certificate\t01-10-2026",
		"Nothing Doing\t\t01-10-2026",
	}, "\n")

	recorder := call(t, server, "POST", "/api/paste", map[string]any{
		"text":     list,
		"category": string(udin.CategoryAuditAssurance),
		"commit":   false,
	})
	if recorder.Code != http.StatusOK {
		t.Fatalf("POST /api/paste = %d: %s", recorder.Code, recorder.Body.String())
	}
	var preview pasteResponse
	decode(t, recorder, &preview)
	if preview.Committed || preview.Read != 2 || preview.Imported != 0 {
		t.Fatalf("the preview came back as %+v", preview)
	}
	if len(preview.Warnings) == 0 {
		t.Fatal("the line with no document type produced no warning")
	}

	var board boardResponse
	decode(t, call(t, server, "GET", "/api/board", nil), &board)
	if board.Counts.Total != 0 {
		t.Fatalf("a preview put %d documents in the register", board.Counts.Total)
	}

	recorder = call(t, server, "POST", "/api/paste", map[string]any{
		"text":     list,
		"category": string(udin.CategoryAuditAssurance),
		"commit":   true,
	})
	var committed pasteResponse
	decode(t, recorder, &committed)
	if !committed.Committed || committed.Imported != 2 {
		t.Fatalf("the import came back as %+v", committed)
	}
	if committed.Counts.Total != 2 || committed.Counts.NoUDIN != 1 {
		t.Fatalf("after the import the counts are %+v", committed.Counts)
	}

	// A second import of the same list is refused entry by entry: the UDIN is
	// already on the register, and the register does not hold it twice.
	recorder = call(t, server, "POST", "/api/paste", map[string]any{
		"text":     list,
		"category": string(udin.CategoryAuditAssurance),
		"commit":   true,
	})
	decode(t, recorder, &committed)
	if committed.Imported != 1 {
		t.Fatalf("the repeat import added %d entries", committed.Imported)
	}
	joined := strings.Join(committed.Warnings, " ")
	if !strings.Contains(joined, "already recorded") {
		t.Fatalf("the duplicate UDIN was not reported: %s", joined)
	}

	if recorder := call(t, server, "POST", "/api/paste", map[string]any{"text": "", "category": "certificate"}); recorder.Code != http.StatusBadRequest {
		t.Fatalf("an empty paste = %d", recorder.Code)
	}
	if recorder := call(t, server, "POST", "/api/paste", map[string]any{"text": "a\tb\tc", "category": "nonsense"}); recorder.Code != http.StatusBadRequest {
		t.Fatalf("a bad category = %d", recorder.Code)
	}
}

func TestSettingsCanBeOverriddenAndAreChecked(t *testing.T) {
	server := newTestServer(t)

	recorder := call(t, server, "PUT", "/api/settings", map[string]any{
		"firmName": "Natarajan & Associates",
		"settings": map[string]int{
			"generationWindowDays":  15,
			"eFilingWindowDays":     15,
			"revocationWindowHours": 48,
			"warnDays":              3,
		},
	})
	if recorder.Code != http.StatusOK {
		t.Fatalf("PUT /api/settings = %d: %s", recorder.Code, recorder.Body.String())
	}
	var state stateResponse
	decode(t, recorder, &state)
	if state.Settings.GenerationWindowDays != 15 || state.FirmName != "Natarajan & Associates" {
		t.Fatalf("the settings came back as %+v for %q", state.Settings, state.FirmName)
	}
	// The reference figures are unchanged by an override: they are what the
	// announcement says, not what this firm has chosen.
	if state.Reference.GenerationWindowDays != 60 {
		t.Fatalf("the reference window moved to %d", state.Reference.GenerationWindowDays)
	}

	// A document signed in September is measured against the firm's window.
	if recorder := call(t, server, "PUT", "/api/entry", newEntry()); recorder.Code != http.StatusOK {
		t.Fatalf("PUT = %d", recorder.Code)
	}
	var row udin.Row
	decode(t, call(t, server, "GET", "/api/entry?id=E-0001", nil), &row)
	if row.Assessment.WindowDays != 15 || row.Assessment.Deadline.String() != "2026-09-24" {
		t.Fatalf("the entry was measured against %d days to %s", row.Assessment.WindowDays, row.Assessment.Deadline)
	}

	for _, bad := range []map[string]int{
		{"generationWindowDays": 0, "eFilingWindowDays": 15, "revocationWindowHours": 48, "warnDays": 3},
		{"generationWindowDays": 60, "eFilingWindowDays": 0, "revocationWindowHours": 48, "warnDays": 3},
		{"generationWindowDays": 60, "eFilingWindowDays": 15, "revocationWindowHours": 0, "warnDays": 3},
		{"generationWindowDays": 60, "eFilingWindowDays": 15, "revocationWindowHours": 48, "warnDays": 60},
	} {
		recorder := call(t, server, "PUT", "/api/settings", map[string]any{"settings": bad})
		if recorder.Code != http.StatusBadRequest {
			t.Fatalf("settings %+v were accepted", bad)
		}
	}
}

func TestTheInterfaceIsServedFromTheBinary(t *testing.T) {
	server := newTestServer(t)

	recorder := call(t, server, "GET", "/", nil)
	if recorder.Code != http.StatusOK {
		t.Fatalf("GET / = %d", recorder.Code)
	}
	page := recorder.Body.String()
	for _, want := range []string{
		"<h1>UDIN register",
		"default-src 'self'; base-uri 'self'; img-src 'self' data:; style-src 'self'; script-src 'self'; font-src 'self'; form-action 'self'",
		"assets/app.js",
		"· Open MSME</title>",
	} {
		if !strings.Contains(page, want) {
			t.Fatalf("the page does not contain %q", want)
		}
	}
	if strings.Contains(page, "<style") || strings.Contains(page, "style=\"") || strings.Contains(page, "onclick=") {
		t.Fatal("the page carries an inline style or handler")
	}

	for _, path := range []string{"/assets/app.js", "/assets/app.css", "/assets/style.css", "/assets/accent.css", "/assets/theme-init.js", "/assets/favicon.svg"} {
		if recorder := call(t, server, "GET", path, nil); recorder.Code != http.StatusOK {
			t.Fatalf("GET %s = %d", path, recorder.Code)
		}
	}

	if got := recorder.Header().Get("X-Content-Type-Options"); got != "nosniff" {
		t.Fatalf("X-Content-Type-Options = %q", got)
	}
	if got := recorder.Header().Get("Cache-Control"); got != "no-store" {
		t.Fatalf("Cache-Control = %q", got)
	}
	if got := recorder.Header().Get("Referrer-Policy"); got != "no-referrer" {
		t.Fatalf("Referrer-Policy = %q", got)
	}
}
