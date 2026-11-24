package udin

import (
	"strings"
	"testing"
)

func TestParsePasteInTheDefaultOrder(t *testing.T) {
	// What a spreadsheet puts on the clipboard: tabs, day-first dates, and a
	// UDIN column that is mostly empty.
	pasted := strings.Join([]string{
		"Sundaram Textiles Private Limited\tStatutory audit report\t10-09-2026\t26304576AKTSBN1359\t12-09-2026",
		"Anand Traders\tTurnover certificate for a bank\t01-10-2026",
		"",
		"Zenith Mills Limited\tForm 3CB-3CD\t30-09-2026\t\t",
	}, "\n")

	got, err := ParsePaste(pasted, CategoryAuditAssurance)
	if err != nil {
		t.Fatalf("ParsePaste: %v", err)
	}
	if len(got.Entries) != 3 {
		t.Fatalf("read %d documents, want 3 (warnings: %v)", len(got.Entries), got.Warnings)
	}
	if len(got.Warnings) != 0 {
		t.Fatalf("warnings on a clean paste: %v", got.Warnings)
	}

	first := got.Entries[0]
	if first.ClientName != "Sundaram Textiles Private Limited" || first.DocumentType != "Statutory audit report" {
		t.Fatalf("the first document came out as %+v", first)
	}
	if first.SignedOn.String() != "2026-09-10" {
		t.Fatalf("the signing date was read as %s", first.SignedOn)
	}
	if first.UDIN != "26304576AKTSBN1359" {
		t.Fatalf("the UDIN was read as %q", first.UDIN)
	}
	if first.GeneratedAt.String() != "2026-09-12" {
		t.Fatalf("the generation date was read as %s", first.GeneratedAt)
	}
	if first.Category != CategoryAuditAssurance {
		t.Fatalf("the category came out as %q", first.Category)
	}

	second := got.Entries[1]
	if second.HasUDIN() || !second.GeneratedAt.IsZero() {
		t.Fatalf("the second document was given a UDIN: %+v", second)
	}
	if second.SignedOn.String() != "2026-10-01" {
		t.Fatalf("the second signing date was read as %s", second.SignedOn)
	}
	if got.Columns[0] != "client" || got.Columns[2] != "signedOn" {
		t.Fatalf("the columns were taken as %v", got.Columns)
	}
}

func TestParsePasteWithAHeaderRow(t *testing.T) {
	pasted := strings.Join([]string{
		"Date of signing,Client,UDIN,Document,FY,MRN,PAN",
		`10-09-2026,"Sundaram Textiles, Private Limited",,Statutory audit report,2025-26,304576,AABCS1234K`,
		"2026/10/01,Anand Traders,,Turnover certificate,2025-2026,0219876,AAAPA1111B",
	}, "\n")

	got, err := ParsePaste(pasted, CategoryCertificate)
	if err != nil {
		t.Fatalf("ParsePaste: %v", err)
	}
	if len(got.Warnings) != 0 {
		t.Fatalf("warnings on a clean paste: %v", got.Warnings)
	}
	if len(got.Entries) != 2 {
		t.Fatalf("read %d documents, want 2", len(got.Entries))
	}

	first := got.Entries[0]
	if first.ClientName != "Sundaram Textiles, Private Limited" {
		t.Fatalf("the quoted client name came out as %q", first.ClientName)
	}
	if first.SignedOn.String() != "2026-09-10" || first.DocumentType != "Statutory audit report" {
		t.Fatalf("the first document came out as %+v", first)
	}
	if first.FinancialYear.Label() != "2025-26" {
		t.Fatalf("the financial year came out as %q", first.FinancialYear.Label())
	}
	if first.PartnerMRN != "304576" || first.ClientRef != "AABCS1234K" {
		t.Fatalf("the partner or reference came out as %q / %q", first.PartnerMRN, first.ClientRef)
	}

	second := got.Entries[1]
	if second.SignedOn.String() != "2026-10-01" || second.PartnerMRN != "219876" {
		t.Fatalf("the second document came out as %+v", second)
	}
}

func TestParsePastePipeSeparated(t *testing.T) {
	pasted := "Zenith Mills Limited | Form 15CB | 15.08.2026 | | \n"
	got, err := ParsePaste(pasted, CategoryCertificate)
	if err != nil {
		t.Fatalf("ParsePaste: %v", err)
	}
	if len(got.Entries) != 1 {
		t.Fatalf("read %d documents", len(got.Entries))
	}
	if got.Entries[0].ClientName != "Zenith Mills Limited" || got.Entries[0].SignedOn.String() != "2026-08-15" {
		t.Fatalf("the document came out as %+v", got.Entries[0])
	}
}

func TestParsePasteWarnsRatherThanGuesses(t *testing.T) {
	pasted := strings.Join([]string{
		"Client\tDocument\tSigned\tUDIN\tUDIN date\tMRN\tFY",
		"Good Client Private Limited\tStatutory audit report\t10-09-2026\t26304576AKTSBN1359\t12-09-2026\t304576\t2025-26",
		"Bad Date Traders\tTurnover certificate\tsometime in October\t\t\t\t",
		"\tTurnover certificate\t01-10-2026\t\t\t\t",
		"No Document Limited\t\t01-10-2026\t\t\t\t",
		"Bad UDIN Limited\tForm 15CB\t01-10-2026\tUDIN-PENDING\t\t\t",
		"Undated UDIN Limited\tForm 15CB\t01-10-2026\t26304576BKTSBN1360\t\t\t",
		"Bad Everything Limited\tForm 15CB\t01-10-2026\t\t\tICAI\tnext year",
		"Date But No UDIN Limited\tForm 15CB\t01-10-2026\t\t05-10-2026\t\t",
	}, "\n")

	got, err := ParsePaste(pasted, CategoryCertificate)
	if err != nil {
		t.Fatalf("ParsePaste: %v", err)
	}

	// Three lines could not be read at all; the rest came in, with notes.
	if len(got.Entries) != 5 {
		t.Fatalf("read %d documents, want 5: %+v", len(got.Entries), got.Entries)
	}
	wants := []string{
		"Line 3",             // unreadable date
		"Line 4",             // no client
		"Line 5",             // no document
		"not in the 18-char", // the pending UDIN
		"has no generation date",
		"not a membership number",
		"not a financial year",
		"generation date was given with no UDIN",
	}
	joined := strings.Join(got.Warnings, "\n")
	for _, want := range wants {
		if !strings.Contains(joined, want) {
			t.Fatalf("the warnings do not mention %q:\n%s", want, joined)
		}
	}

	byClient := map[string]Entry{}
	for _, e := range got.Entries {
		byClient[e.ClientName] = e
	}
	if e := byClient["Bad UDIN Limited"]; e.HasUDIN() || !strings.Contains(e.Notes, "UDIN-PENDING") {
		t.Fatalf("the unreadable UDIN was not kept as a note: %+v", e)
	}
	if e := byClient["Undated UDIN Limited"]; e.HasUDIN() || !strings.Contains(e.Notes, "26304576BKTSBN1360") {
		t.Fatalf("an undated UDIN was stored as if it were on record: %+v", e)
	}
	if e := byClient["Bad Everything Limited"]; e.PartnerMRN != "" || !e.FinancialYear.IsZero() {
		t.Fatalf("unreadable fields were stored anyway: %+v", e)
	}
	if e := byClient["Good Client Private Limited"]; e.UDIN != "26304576AKTSBN1359" || e.GeneratedAt.String() != "2026-09-12" {
		t.Fatalf("the good line was not read properly: %+v", e)
	}
}

func TestParsePasteWarnsAboutColumnsItCannotPlace(t *testing.T) {
	pasted := "Client Private Limited\tStatutory audit report\t10-09-2026\t\t\tR. Natarajan\t304576"
	got, err := ParsePaste(pasted, CategoryAuditAssurance)
	if err != nil {
		t.Fatalf("ParsePaste: %v", err)
	}
	if len(got.Entries) != 1 {
		t.Fatalf("read %d documents", len(got.Entries))
	}
	if len(got.Warnings) != 1 || !strings.Contains(got.Warnings[0], "header row") {
		t.Fatalf("the extra columns were not reported: %v", got.Warnings)
	}
	if got.Entries[0].PartnerName != "" {
		t.Fatalf("a column past the default order was used anyway: %+v", got.Entries[0])
	}
}

func TestParsePasteRejectsWhatItCannotUse(t *testing.T) {
	cases := []struct {
		name     string
		pasted   string
		category Category
		wantErr  string
	}{
		{name: "nothing at all", pasted: "   \n\n", category: CategoryCertificate, wantErr: "nothing in the pasted text"},
		{
			name:     "a header and no documents",
			pasted:   "Client\tDocument\tSigned",
			category: CategoryCertificate,
			wantErr:  "no documents under it",
		},
		{
			name:     "nothing readable",
			pasted:   "a\nb\nc",
			category: CategoryCertificate,
			wantErr:  "could be read",
		},
		{
			name:     "no category chosen",
			pasted:   "Client Private Limited\tStatutory audit report\t10-09-2026",
			category: "",
			wantErr:  "category",
		},
		{
			name:     "an invented category",
			pasted:   "Client Private Limited\tStatutory audit report\t10-09-2026",
			category: "peer-review",
			wantErr:  "category",
		},
		{
			name:     "more lines than the tool takes",
			pasted:   strings.Repeat("Client\tDocument\t10-09-2026\n", maxPasteLines+1),
			category: CategoryCertificate,
			wantErr:  "at a time",
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, err := ParsePaste(c.pasted, c.category)
			if err == nil {
				t.Fatalf("ParsePaste accepted %s", c.name)
			}
			if !strings.Contains(err.Error(), c.wantErr) {
				t.Fatalf("ParsePaste said %q, which does not mention %q", err, c.wantErr)
			}
		})
	}
}

func TestParsePasteReadsAGenerationTime(t *testing.T) {
	pasted := strings.Join([]string{
		"Client\tDocument\tSigned\tUDIN\tUDIN date",
		"Client Private Limited\tForm 15CB\t10-09-2026\t26304576AKTSBN1359\t12-09-2026 14:30",
		"Other Private Limited\tForm 15CB\t10-09-2026\t26304576BKTSBN1360\t2026-09-12T09:05",
	}, "\n")

	got, err := ParsePaste(pasted, CategoryCertificate)
	if err != nil {
		t.Fatalf("ParsePaste: %v", err)
	}
	if len(got.Warnings) != 0 {
		t.Fatalf("warnings: %v", got.Warnings)
	}
	if got.Entries[0].GeneratedAt.String() != "2026-09-12T14:30" || !got.Entries[0].GeneratedAt.Exact() {
		t.Fatalf("the first generation time came out as %s", got.Entries[0].GeneratedAt)
	}
	if got.Entries[1].GeneratedAt.String() != "2026-09-12T09:05" {
		t.Fatalf("the second generation time came out as %s", got.Entries[1].GeneratedAt)
	}
}

// The wedge, end to end: a pasted list becomes a register that immediately
// says what is outstanding and how long is left.
func TestPastedListBecomesAnAnsweredRegister(t *testing.T) {
	pasted := strings.Join([]string{
		"Client\tDocument\tSigned\tUDIN\tUDIN date",
		"On Time Private Limited\tStatutory audit report\t10-09-2026\t26304576AKTSBN1359\t12-09-2026",
		"Pending Private Limited\tStatutory audit report\t01-10-2026\t\t",
		"Missed Private Limited\tStatutory audit report\t01-08-2026\t\t",
	}, "\n")

	parsed, err := ParsePaste(pasted, CategoryAuditAssurance)
	if err != nil {
		t.Fatalf("ParsePaste: %v", err)
	}

	r := New()
	for _, entry := range parsed.Entries {
		entry.PartnerName = "R. Natarajan"
		entry.PartnerMRN = "304576"
		if _, err := r.Put(entry, today); err != nil {
			t.Fatalf("Put(%s): %v", entry.ClientName, err)
		}
	}

	rows := Board(r, today)
	counts := Summarise(rows)
	if counts.Total != 3 || counts.NoUDIN != 2 || counts.Overdue != 1 || counts.Recorded != 1 {
		t.Fatalf("Summarise() = %+v", counts)
	}
	if rows[0].Entry.ClientName != "Missed Private Limited" {
		t.Fatalf("the register did not put the missed document first: %q", rows[0].Entry.ClientName)
	}
	pending := rows[1]
	if pending.Entry.ClientName != "Pending Private Limited" {
		t.Fatalf("the second row is %q", pending.Entry.ClientName)
	}
	if pending.Assessment.DaysLeft != 5 || pending.Assessment.Deadline.String() != "2026-11-29" {
		t.Fatalf("the pending row says %d days to %s", pending.Assessment.DaysLeft, pending.Assessment.Deadline)
	}
}
