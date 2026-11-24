package udin

import (
	"strings"
	"testing"
)

// today is the day every register test is run against, so nothing depends on
// when the suite happens to be run.
var today = MustDate(2026, 11, 24)

func sampleEntry() Entry {
	return Entry{
		Category:      CategoryAuditAssurance,
		DocumentType:  "Statutory audit report (Companies Act, section 143)",
		Description:   "Independent auditor's report on the financial statements for the year ended 31 March 2026",
		ClientName:    "Sundaram Textiles Private Limited",
		ClientRef:     "aabcs1234k",
		FinancialYear: FinYear{start: 2025},
		SignedOn:      MustDate(2026, 9, 10),
		PartnerName:   "R. Natarajan",
		PartnerMRN:    "M. No. 304576",
		Figures: []Figure{
			{Keyword: "Turnover", Value: "48,20,15,000"},
			{Keyword: "Shareholders' funds", Value: "12,40,00,000"},
		},
	}
}

func TestPutStoresATidiedEntry(t *testing.T) {
	r := New()
	in := sampleEntry()
	in.ClientName = "  Sundaram Textiles Private Limited  "
	in.UDIN = "26 304576 aktsbn1359"
	in.GeneratedAt = DateStamp(MustDate(2026, 9, 12))

	stored, err := r.Put(in, today)
	if err != nil {
		t.Fatalf("Put: %v", err)
	}
	if stored.ID != "E-0001" {
		t.Fatalf("the first entry was given the identifier %q", stored.ID)
	}
	if stored.ClientName != "Sundaram Textiles Private Limited" {
		t.Fatalf("the client name was stored as %q", stored.ClientName)
	}
	if stored.ClientRef != "AABCS1234K" {
		t.Fatalf("the client reference was stored as %q", stored.ClientRef)
	}
	if stored.PartnerMRN != "304576" {
		t.Fatalf("the membership number was stored as %q", stored.PartnerMRN)
	}
	if stored.UDIN != "26304576AKTSBN1359" {
		t.Fatalf("the UDIN was stored as %q", stored.UDIN)
	}
	if len(stored.Figures) != 2 || stored.Figures[0].Keyword != "Turnover" {
		t.Fatalf("the figures were stored as %+v", stored.Figures)
	}
	if !stored.HasUDIN() || stored.Revoked() {
		t.Fatalf("HasUDIN() = %v, Revoked() = %v", stored.HasUDIN(), stored.Revoked())
	}
	if r.NextID != 2 || len(r.Entries) != 1 {
		t.Fatalf("after one entry the register holds %d entries and next is %d", len(r.Entries), r.NextID)
	}

	second, err := r.Put(sampleEntry(), today)
	if err != nil {
		t.Fatalf("second Put: %v", err)
	}
	if second.ID != "E-0002" {
		t.Fatalf("the second entry was given the identifier %q", second.ID)
	}
}

func TestPutRejectsWhatTheRegisterCannotHold(t *testing.T) {
	cases := []struct {
		name    string
		change  func(*Entry)
		wantErr string
	}{
		{name: "no category", change: func(e *Entry) { e.Category = "" }, wantErr: "category"},
		{name: "an invented category", change: func(e *Entry) { e.Category = "peer-review" }, wantErr: "not a UDIN category"},
		{name: "no client", change: func(e *Entry) { e.ClientName = "   " }, wantErr: "client"},
		{name: "no document type", change: func(e *Entry) { e.DocumentType = "" }, wantErr: "what the document is"},
		{name: "no signing date", change: func(e *Entry) { e.SignedOn = Date{} }, wantErr: "signed"},
		{name: "signed in the future", change: func(e *Entry) { e.SignedOn = today.AddDays(1) }, wantErr: "future"},
		{
			name:    "a description shorter than the portal takes",
			change:  func(e *Entry) { e.Description = "FS" },
			wantErr: "5 to 250",
		},
		{
			name:    "a description longer than the portal takes",
			change:  func(e *Entry) { e.Description = strings.Repeat("a", 251) },
			wantErr: "5 to 250",
		},
		{
			name:    "a client name longer than the field",
			change:  func(e *Entry) { e.ClientName = strings.Repeat("a", 161) },
			wantErr: "longer than",
		},
		{name: "a membership number that is not one", change: func(e *Entry) { e.PartnerMRN = "ICAI" }, wantErr: "membership number"},
		{
			name:    "a figure with no keyword",
			change:  func(e *Entry) { e.Figures = []Figure{{Value: "48,20,15,000"}} },
			wantErr: "no keyword",
		},
		{
			name: "more figures than the portal asks for",
			change: func(e *Entry) {
				e.Figures = nil
				for i := 0; i < 9; i++ {
					e.Figures = append(e.Figures, Figure{Keyword: "Keyword", Value: "1"})
				}
			},
			wantErr: "not 9",
		},
		{
			name:    "a UDIN that is not one",
			change:  func(e *Entry) { e.UDIN = "NO-UDIN-HERE" },
			wantErr: "not a UDIN",
		},
		{
			name:    "a UDIN with no generation date",
			change:  func(e *Entry) { e.UDIN = "26304576AKTSBN1359" },
			wantErr: "date the UDIN was generated",
		},
		{
			name: "a UDIN generated before the document was signed",
			change: func(e *Entry) {
				e.UDIN = "26304576AKTSBN1359"
				e.GeneratedAt = DateStamp(e.SignedOn.AddDays(-1))
			},
			wantErr: "before the document was signed",
		},
		{
			name: "a UDIN generated in the future",
			change: func(e *Entry) {
				e.UDIN = "26304576AKTSBN1359"
				e.GeneratedAt = DateStamp(today.AddDays(1))
			},
			wantErr: "future",
		},
		{
			name:    "a generation date with no UDIN",
			change:  func(e *Entry) { e.GeneratedAt = DateStamp(MustDate(2026, 9, 12)) },
			wantErr: "no UDIN",
		},
		{
			name: "an e-filing submission before signing",
			change: func(e *Entry) {
				e.EFilingSubmittedOn = e.SignedOn.AddDays(-1)
			},
			wantErr: "before the document was signed",
		},
		{
			name: "an e-filing submission in the future",
			change: func(e *Entry) {
				e.EFilingSubmittedOn = today.AddDays(1)
			},
			wantErr: "future",
		},
		{
			name:    "an unknown entry identifier",
			change:  func(e *Entry) { e.ID = "E-9999" },
			wantErr: "no entry",
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			r := New()
			in := sampleEntry()
			c.change(&in)
			_, err := r.Put(in, today)
			if err == nil {
				t.Fatalf("Put accepted %s", c.name)
			}
			if !strings.Contains(err.Error(), c.wantErr) {
				t.Fatalf("Put(%s) said %q, which does not mention %q", c.name, err, c.wantErr)
			}
			if len(r.Entries) != 0 {
				t.Fatalf("a rejected entry was stored anyway")
			}
		})
	}
}

func TestPutAcceptsADocumentWithNoUDINYet(t *testing.T) {
	r := New()
	in := sampleEntry()
	in.UDIN = ""
	stored, err := r.Put(in, today)
	if err != nil {
		t.Fatalf("Put: %v", err)
	}
	if stored.HasUDIN() || !stored.GeneratedAt.IsZero() {
		t.Fatalf("an entry with no UDIN came back as %+v", stored)
	}
}

func TestPutUpdatesInPlaceAndKeepsTheRevocation(t *testing.T) {
	r := New()
	first := sampleEntry()
	first.UDIN = "26304576AKTSBN1359"
	first.GeneratedAt = DateStamp(MustDate(2026, 9, 12))
	stored, err := r.Put(first, today)
	if err != nil {
		t.Fatalf("Put: %v", err)
	}
	if _, err := r.Revoke(stored.ID, Revocation{
		RevokedAt: DateStamp(MustDate(2026, 9, 13)),
		Reason:    "A figure in the report was restated after signing.",
	}, today); err != nil {
		t.Fatalf("Revoke: %v", err)
	}

	update := sampleEntry()
	update.ID = stored.ID
	update.UDIN = "26304576AKTSBN1359"
	update.GeneratedAt = DateStamp(MustDate(2026, 9, 12))
	update.Notes = "Reissued under a fresh number."
	update.Revocation = nil // a stray edit must not wipe the revocation

	updated, err := r.Put(update, today)
	if err != nil {
		t.Fatalf("Put update: %v", err)
	}
	if len(r.Entries) != 1 {
		t.Fatalf("the update created a second entry: %d entries", len(r.Entries))
	}
	if !updated.Revoked() {
		t.Fatal("the update erased the revocation")
	}
	if updated.Notes != "Reissued under a fresh number." {
		t.Fatalf("the note was stored as %q", updated.Notes)
	}
}

func TestPutRefusesTheSameUDINTwice(t *testing.T) {
	r := New()
	first := sampleEntry()
	first.UDIN = "26304576AKTSBN1359"
	first.GeneratedAt = DateStamp(MustDate(2026, 9, 12))
	if _, err := r.Put(first, today); err != nil {
		t.Fatalf("Put: %v", err)
	}

	second := sampleEntry()
	second.ClientName = "Another Client Private Limited"
	second.UDIN = "26 304576 AKTSBN 1359" // the same number, spaced
	second.GeneratedAt = DateStamp(MustDate(2026, 9, 12))
	_, err := r.Put(second, today)
	if err == nil {
		t.Fatal("the same UDIN was accepted on two entries")
	}
	if !strings.Contains(err.Error(), "E-0001") {
		t.Fatalf("the error does not name the entry already holding it: %v", err)
	}

	// Re-saving the same entry with its own UDIN must still work.
	stored, _ := r.Entry("E-0001")
	again := stored
	again.Notes = "Checked against the signed copy."
	if _, err := r.Put(again, today); err != nil {
		t.Fatalf("re-saving an entry with its own UDIN failed: %v", err)
	}
}

func TestEntriesAreKeptInSigningOrder(t *testing.T) {
	r := New()
	dates := []Date{
		MustDate(2026, 9, 30),
		MustDate(2026, 3, 31),
		MustDate(2026, 9, 30),
		MustDate(2026, 4, 1),
	}
	for _, date := range dates {
		in := sampleEntry()
		in.SignedOn = date
		if _, err := r.Put(in, today); err != nil {
			t.Fatalf("Put(%s): %v", date, err)
		}
	}
	wantOrder := []string{"2026-03-31", "2026-04-01", "2026-09-30", "2026-09-30"}
	wantIDs := []string{"E-0002", "E-0004", "E-0001", "E-0003"}
	for i, e := range r.Entries {
		if e.SignedOn.String() != wantOrder[i] || e.ID != wantIDs[i] {
			t.Fatalf("entry %d is %s signed %s, want %s signed %s", i, e.ID, e.SignedOn, wantIDs[i], wantOrder[i])
		}
	}
}

func TestRevoke(t *testing.T) {
	generated := DateStamp(MustDate(2026, 9, 12))
	cases := []struct {
		name       string
		withUDIN   bool
		revocation Revocation
		preRevoke  bool
		wantErr    string
	}{
		{
			name:     "a revocation with a reason",
			withUDIN: true,
			revocation: Revocation{
				RevokedAt: DateStamp(MustDate(2026, 9, 13)),
				Reason:    "The client restated a figure after the report was signed.",
			},
		},
		{
			name:     "a revocation naming the replacement number",
			withUDIN: true,
			revocation: Revocation{
				RevokedAt:       DateStamp(MustDate(2026, 9, 13)),
				Reason:          "Signed copy carried the wrong date.",
				ReplacementUDIN: "26304576BKTSBN1360",
			},
		},
		{
			name:       "nothing to revoke",
			withUDIN:   false,
			revocation: Revocation{RevokedAt: DateStamp(MustDate(2026, 9, 13)), Reason: "Wrong figures."},
			wantErr:    "no UDIN to revoke",
		},
		{
			name:       "no reason",
			withUDIN:   true,
			revocation: Revocation{RevokedAt: DateStamp(MustDate(2026, 9, 13)), Reason: "oops"},
			wantErr:    "reason",
		},
		{
			name:       "no revocation date",
			withUDIN:   true,
			revocation: Revocation{Reason: "The client restated a figure."},
			wantErr:    "when the UDIN was revoked",
		},
		{
			name:       "revoked in the future",
			withUDIN:   true,
			revocation: Revocation{RevokedAt: DateStamp(today.AddDays(1)), Reason: "The client restated a figure."},
			wantErr:    "future",
		},
		{
			name:       "revoked before the UDIN existed",
			withUDIN:   true,
			revocation: Revocation{RevokedAt: DateStamp(MustDate(2026, 9, 11)), Reason: "The client restated a figure."},
			wantErr:    "cannot be revoked before it exists",
		},
		{
			name:     "a replacement that is not a UDIN",
			withUDIN: true,
			revocation: Revocation{
				RevokedAt:       DateStamp(MustDate(2026, 9, 13)),
				Reason:          "The client restated a figure.",
				ReplacementUDIN: "PENDING",
			},
			wantErr: "replacement UDIN is not usable",
		},
		{
			name:     "a replacement that is the same number",
			withUDIN: true,
			revocation: Revocation{
				RevokedAt:       DateStamp(MustDate(2026, 9, 13)),
				Reason:          "The client restated a figure.",
				ReplacementUDIN: "26304576AKTSBN1359",
			},
			wantErr: "same number",
		},
		{
			name:      "revoked twice",
			withUDIN:  true,
			preRevoke: true,
			revocation: Revocation{
				RevokedAt: DateStamp(MustDate(2026, 9, 14)),
				Reason:    "Changed my mind about the reason.",
			},
			wantErr: "already revoked",
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			r := New()
			in := sampleEntry()
			if c.withUDIN {
				in.UDIN = "26304576AKTSBN1359"
				in.GeneratedAt = generated
			}
			stored, err := r.Put(in, today)
			if err != nil {
				t.Fatalf("Put: %v", err)
			}
			if c.preRevoke {
				if _, err := r.Revoke(stored.ID, Revocation{
					RevokedAt: DateStamp(MustDate(2026, 9, 13)),
					Reason:    "The client restated a figure after signing.",
				}, today); err != nil {
					t.Fatalf("first Revoke: %v", err)
				}
			}

			got, err := r.Revoke(stored.ID, c.revocation, today)
			if c.wantErr != "" {
				if err == nil {
					t.Fatalf("Revoke accepted %s", c.name)
				}
				if !strings.Contains(err.Error(), c.wantErr) {
					t.Fatalf("Revoke said %q, which does not mention %q", err, c.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("Revoke: %v", err)
			}
			if !got.Revoked() {
				t.Fatal("the entry does not report as revoked")
			}
			if got.UDIN == "" {
				t.Fatal("revoking removed the UDIN; a revoked number stays on the record")
			}
			if len(r.Entries) != 1 {
				t.Fatalf("revoking changed the number of entries to %d", len(r.Entries))
			}
			if got.Revocation.Reason != strings.TrimSpace(c.revocation.Reason) {
				t.Fatalf("the reason was stored as %q", got.Revocation.Reason)
			}
		})
	}

	r := New()
	if _, err := r.Revoke("E-0404", Revocation{RevokedAt: DateStamp(today), Reason: "Not there at all."}, today); err == nil {
		t.Fatal("an entry that does not exist was revoked")
	}
}

func TestRemoveRefusesAnEntryThatCarriesAUDIN(t *testing.T) {
	r := New()
	withUDIN := sampleEntry()
	withUDIN.UDIN = "26304576AKTSBN1359"
	withUDIN.GeneratedAt = DateStamp(MustDate(2026, 9, 12))
	numbered, err := r.Put(withUDIN, today)
	if err != nil {
		t.Fatalf("Put: %v", err)
	}

	draft := sampleEntry()
	draft.ClientName = "Typed By Mistake Private Limited"
	mistake, err := r.Put(draft, today)
	if err != nil {
		t.Fatalf("Put: %v", err)
	}

	if err := r.Remove(numbered.ID); err == nil {
		t.Fatal("an entry carrying a UDIN was removed from the register")
	}
	if err := r.Remove(mistake.ID); err != nil {
		t.Fatalf("a mistyped entry with no UDIN could not be removed: %v", err)
	}
	if len(r.Entries) != 1 || r.Entries[0].ID != numbered.ID {
		t.Fatalf("the register holds %d entries after the removal", len(r.Entries))
	}
	if err := r.Remove("E-0404"); err == nil {
		t.Fatal("removing an entry that does not exist succeeded")
	}
}

func TestRecallMarksTheDocumentAsTakenBack(t *testing.T) {
	r := New()
	in := sampleEntry()
	in.UDIN = "26304576AKTSBN1359"
	in.GeneratedAt = DateStamp(MustDate(2026, 9, 12))
	in.InCirculation = true
	stored, err := r.Put(in, today)
	if err != nil {
		t.Fatalf("Put: %v", err)
	}

	if _, err := r.Recall(stored.ID, true); err == nil {
		t.Fatal("a document with no revoked UDIN was marked as recalled")
	}

	if _, err := r.Revoke(stored.ID, Revocation{
		RevokedAt: DateStamp(MustDate(2026, 9, 13)),
		Reason:    "The report was signed against draft accounts.",
	}, today); err != nil {
		t.Fatalf("Revoke: %v", err)
	}

	recalled, err := r.Recall(stored.ID, true)
	if err != nil {
		t.Fatalf("Recall: %v", err)
	}
	if !recalled.Revocation.Recalled {
		t.Fatal("the recall was not recorded")
	}
	if back, err := r.Recall(stored.ID, false); err != nil || back.Revocation.Recalled {
		t.Fatalf("the recall could not be undone: %+v %v", back.Revocation, err)
	}
	if _, err := r.Recall("E-0404", true); err == nil {
		t.Fatal("an entry that does not exist was recalled")
	}
}

func TestNormaliseRepairsAHandEditedFile(t *testing.T) {
	r := &Register{
		Entries: []Entry{
			{ID: "E-0007", ClientName: "Later", SignedOn: MustDate(2026, 5, 1)},
			{ID: "E-0003", ClientName: "Earlier", SignedOn: MustDate(2026, 1, 1)},
		},
		NextID: 1,
	}
	r.Normalise()

	if r.NextID != 8 {
		t.Fatalf("the next identifier is %d, which would overwrite an existing entry", r.NextID)
	}
	if r.Entries[0].ID != "E-0003" {
		t.Fatalf("the entries were not put back in signing order: %s first", r.Entries[0].ID)
	}
	if r.Settings != DefaultSettings() {
		t.Fatalf("the settings were not filled in: %+v", r.Settings)
	}
	for _, e := range r.Entries {
		if e.Figures == nil {
			t.Fatalf("entry %s has a nil figure list, which would encode as null", e.ID)
		}
	}

	empty := &Register{NextID: -5}
	empty.Normalise()
	if empty.NextID != 1 || empty.Entries == nil {
		t.Fatalf("an empty register came back with next=%d entries=%v", empty.NextID, empty.Entries)
	}
}

func TestSetSettingsAndFirmName(t *testing.T) {
	r := New()
	if err := r.SetSettings(Settings{GenerationWindowDays: 0}); err == nil {
		t.Fatal("a window of zero days was accepted")
	}
	if r.Settings != DefaultSettings() {
		t.Fatalf("a rejected change was applied anyway: %+v", r.Settings)
	}
	want := Settings{GenerationWindowDays: 15, EFilingWindowDays: 15, RevocationWindowHours: 48, WarnDays: 3}
	if err := r.SetSettings(want); err != nil {
		t.Fatalf("SetSettings: %v", err)
	}
	if r.Settings != want {
		t.Fatalf("the settings were stored as %+v", r.Settings)
	}

	if err := r.SetFirmName("  Natarajan & Associates  "); err != nil {
		t.Fatalf("SetFirmName: %v", err)
	}
	if r.FirmName != "Natarajan & Associates" {
		t.Fatalf("the firm name was stored as %q", r.FirmName)
	}
	if err := r.SetFirmName(strings.Repeat("a", 161)); err == nil {
		t.Fatal("a firm name longer than the field was accepted")
	}
}

func TestDistinctClientsAndPartners(t *testing.T) {
	r := New()
	for _, name := range []string{"Zenith Mills Limited", "Anand Traders", "Zenith Mills Limited"} {
		in := sampleEntry()
		in.ClientName = name
		if _, err := r.Put(in, today); err != nil {
			t.Fatalf("Put(%q): %v", name, err)
		}
	}
	other := sampleEntry()
	other.PartnerName = "S. Iyer"
	other.PartnerMRN = "219876"
	if _, err := r.Put(other, today); err != nil {
		t.Fatalf("Put: %v", err)
	}

	clients := r.DistinctClients()
	if len(clients) != 3 || clients[0] != "Anand Traders" {
		t.Fatalf("DistinctClients() = %v", clients)
	}
	partners := r.DistinctPartners()
	if len(partners) != 2 {
		t.Fatalf("DistinctPartners() = %+v", partners)
	}
	if partners[0].Name != "R. Natarajan" || partners[0].MRN != "304576" {
		t.Fatalf("the first partner is %+v", partners[0])
	}
	if partners[1].Name != "S. Iyer" || partners[1].MRN != "219876" {
		t.Fatalf("the second partner is %+v", partners[1])
	}

	empty := New()
	if len(empty.DistinctClients()) != 0 || len(empty.DistinctPartners()) != 0 {
		t.Fatal("an empty register listed clients or partners")
	}
	if _, ok := empty.Entry("E-0001"); ok {
		t.Fatal("an empty register found an entry")
	}
}
