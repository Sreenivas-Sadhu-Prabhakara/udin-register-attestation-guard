package udin

import (
	"strings"
	"testing"
)

// assessEntry is the entry every assessment case starts from: a complete
// record, so that a case only has to change the one thing it is about.
func assessEntry() Entry {
	return Entry{
		ID:            "E-0001",
		Category:      CategoryAuditAssurance,
		DocumentType:  "Statutory audit report (Companies Act, section 143)",
		Description:   "Independent auditor's report on the financial statements for the year ended 31 March 2026",
		ClientName:    "Sundaram Textiles Private Limited",
		FinancialYear: FinYear{start: 2025},
		SignedOn:      MustDate(2026, 9, 10),
		PartnerName:   "R. Natarajan",
		PartnerMRN:    "304576",
		Figures: []Figure{
			{Keyword: "Turnover", Value: "48,20,15,000"},
			{Keyword: "Shareholders' funds", Value: "12,40,00,000"},
		},
	}
}

func withUDIN(e Entry, udin string, generated Date) Entry {
	e.UDIN = udin
	e.GeneratedAt = DateStamp(generated)
	return e
}

func TestAssessStages(t *testing.T) {
	cases := []struct {
		name         string
		entry        Entry
		wantStage    Stage
		wantSeverity Severity
		wantDeadline string
		wantDaysLeft int
		wantDaysUsed int
		wantRule     string
		wantFlags    []string
		wantNoFlags  bool
	}{
		{
			name: "signed today, the whole window ahead",
			entry: func() Entry {
				e := assessEntry()
				e.SignedOn = today
				return e
			}(),
			wantStage: StageAwaiting, wantSeverity: SeverityNotice,
			wantDeadline: "2027-01-22", wantDaysLeft: 59, wantRule: RuleGeneration, wantNoFlags: true,
		},
		{
			name: "five days left",
			entry: func() Entry {
				e := assessEntry()
				e.SignedOn = MustDate(2026, 10, 1)
				return e
			}(),
			wantStage: StageAwaiting, wantSeverity: SeverityWarn,
			wantDeadline: "2026-11-29", wantDaysLeft: 5, wantRule: RuleGeneration, wantNoFlags: true,
		},
		{
			name: "today is the last day of the window",
			entry: func() Entry {
				e := assessEntry()
				e.SignedOn = MustDate(2026, 9, 26)
				return e
			}(),
			wantStage: StageAwaiting, wantSeverity: SeverityUrgent,
			wantDeadline: "2026-11-24", wantDaysLeft: 0, wantRule: RuleGeneration, wantNoFlags: true,
		},
		{
			name: "the window closed and there is still no UDIN",
			entry: func() Entry {
				e := assessEntry()
				e.SignedOn = MustDate(2026, 8, 1)
				return e
			}(),
			wantStage: StageOverdue, wantSeverity: SeverityBreach,
			wantDeadline: "2026-09-29", wantDaysLeft: -56, wantRule: RuleGeneration, wantNoFlags: true,
		},
		{
			name:      "a UDIN generated two days after signing",
			entry:     withUDIN(assessEntry(), "26304576AKTSBN1359", MustDate(2026, 9, 12)),
			wantStage: StageRecorded, wantSeverity: SeverityOK,
			wantDeadline: "2026-11-08", wantDaysUsed: 3, wantDaysLeft: -16, wantRule: RuleGeneration, wantNoFlags: true,
		},
		{
			name:      "a UDIN generated on the very last day of the window",
			entry:     withUDIN(assessEntry(), "26304576AKTSBN1359", MustDate(2026, 11, 8)),
			wantStage: StageRecorded, wantSeverity: SeverityOK,
			wantDeadline: "2026-11-08", wantDaysUsed: 60, wantDaysLeft: -16, wantRule: RuleGeneration, wantNoFlags: true,
		},
		{
			name:      "a UDIN generated one day after the window closed",
			entry:     withUDIN(assessEntry(), "26304576AKTSBN1359", MustDate(2026, 11, 9)),
			wantStage: StageLate, wantSeverity: SeverityWarn,
			wantDeadline: "2026-11-08", wantDaysUsed: 61, wantDaysLeft: -16, wantRule: RuleGeneration,
			wantFlags: []string{"generated-late"},
		},
		{
			name: "a certificate signed before UDIN was mandatory",
			entry: func() Entry {
				e := assessEntry()
				e.Category = CategoryCertificate
				e.FinancialYear = FinYear{start: 2018}
				e.SignedOn = MustDate(2019, 1, 31)
				return e
			}(),
			wantStage: StageNotRequired, wantSeverity: SeverityOK,
			wantDeadline: "", wantRule: RuleNone, wantNoFlags: true,
		},
		{
			name: "a certificate signed on the first day of its mandate",
			entry: func() Entry {
				e := assessEntry()
				e.Category = CategoryCertificate
				e.FinancialYear = FinYear{start: 2018}
				e.SignedOn = MustDate(2019, 2, 1)
				return e
			}(),
			wantStage: StageOverdue, wantSeverity: SeverityBreach,
			wantDeadline: "2019-04-01", wantDaysLeft: -2794, wantRule: RuleGeneration, wantNoFlags: true,
		},
		{
			name: "the e-filing clock is the one that binds",
			entry: func() Entry {
				e := assessEntry()
				e.Category = CategoryGSTTaxAudit
				e.DocumentType = "Form 3CB-3CD (tax audit report, section 44AB)"
				e.SignedOn = MustDate(2026, 11, 20)
				e.EFilingSubmittedOn = MustDate(2026, 11, 20)
				return e
			}(),
			wantStage: StageAwaiting, wantSeverity: SeverityWarn,
			wantDeadline: "2026-12-04", wantDaysLeft: 10, wantRule: RuleEFiling, wantNoFlags: true,
		},
		{
			name: "a revoked UDIN on a document that is still out",
			entry: func() Entry {
				e := withUDIN(assessEntry(), "26304576AKTSBN1359", MustDate(2026, 9, 12))
				e.InCirculation = true
				e.Revocation = &Revocation{
					RevokedAt: DateStamp(MustDate(2026, 9, 13)),
					Reason:    "The client restated a figure after the report was signed.",
				}
				return e
			}(),
			wantStage: StageRevoked, wantSeverity: SeverityBreach, wantDaysUsed: 3,
			wantDeadline: "2026-11-08", wantRule: RuleGeneration, wantDaysLeft: -16,
			wantFlags: []string{"revoked-in-circulation", "no-replacement"},
		},
		{
			name: "a revoked UDIN on a document that has been taken back",
			entry: func() Entry {
				e := withUDIN(assessEntry(), "26304576AKTSBN1359", MustDate(2026, 9, 12))
				e.InCirculation = true
				e.Revocation = &Revocation{
					RevokedAt:       DateStamp(MustDate(2026, 9, 13)),
					Reason:          "The client restated a figure after the report was signed.",
					ReplacementUDIN: "26304576BKTSBN1360",
					Recalled:        true,
				}
				return e
			}(),
			wantStage: StageRevoked, wantSeverity: SeverityNotice, wantDaysUsed: 3,
			wantDeadline: "2026-11-08", wantRule: RuleGeneration, wantDaysLeft: -16, wantNoFlags: true,
		},
		{
			name: "a revocation recorded after the portal's window had shut",
			entry: func() Entry {
				e := withUDIN(assessEntry(), "26304576AKTSBN1359", MustDate(2026, 9, 12))
				e.Revocation = &Revocation{
					RevokedAt:       DateStamp(MustDate(2026, 9, 20)),
					Reason:          "Withdrawn from the client after a figure was restated.",
					ReplacementUDIN: "26304576BKTSBN1360",
				}
				return e
			}(),
			wantStage: StageRevoked, wantSeverity: SeverityWarn, wantDaysUsed: 3,
			wantDeadline: "2026-11-08", wantRule: RuleGeneration, wantDaysLeft: -16,
			wantFlags: []string{"revoked-after-portal-window"},
		},
		{
			name: "a UDIN carrying another member's number",
			entry: func() Entry {
				e := withUDIN(assessEntry(), "26999999AKTSBN1359", MustDate(2026, 9, 12))
				return e
			}(),
			wantStage: StageRecorded, wantSeverity: SeverityBreach, wantDaysUsed: 3,
			wantDeadline: "2026-11-08", wantRule: RuleGeneration, wantDaysLeft: -16,
			wantFlags: []string{"udin-mrn-mismatch"},
		},
		{
			name: "a record with no description, no figures and no partner",
			entry: func() Entry {
				e := withUDIN(assessEntry(), "26304576AKTSBN1359", MustDate(2026, 9, 12))
				e.Description = ""
				e.Figures = nil
				e.PartnerName = ""
				e.PartnerMRN = ""
				return e
			}(),
			wantStage: StageRecorded, wantSeverity: SeverityNotice, wantDaysUsed: 3,
			wantDeadline: "2026-11-08", wantRule: RuleGeneration, wantDaysLeft: -16,
			wantFlags: []string{"no-description", "few-figures", "no-partner"},
		},
		{
			name: "a document recorded against a financial year that had not begun",
			entry: func() Entry {
				e := assessEntry()
				e.SignedOn = MustDate(2026, 10, 1)
				e.FinancialYear = FinYear{start: 2027}
				return e
			}(),
			wantStage: StageAwaiting, wantSeverity: SeverityWarn,
			wantDeadline: "2026-11-29", wantDaysLeft: 5, wantRule: RuleGeneration,
			wantFlags: []string{"financial-year-ahead"},
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := Assess(c.entry, DefaultSettings(), today)

			if got.Stage != c.wantStage {
				t.Fatalf("stage = %q, want %q", got.Stage, c.wantStage)
			}
			if got.Severity != c.wantSeverity {
				t.Fatalf("severity = %q, want %q (flags %v)", got.Severity, c.wantSeverity, got.Flags)
			}
			if got.Deadline.String() != c.wantDeadline {
				t.Fatalf("deadline = %q, want %q", got.Deadline, c.wantDeadline)
			}
			if got.DeadlineRule != c.wantRule {
				t.Fatalf("deadline rule = %q, want %q", got.DeadlineRule, c.wantRule)
			}
			if got.DaysLeft != c.wantDaysLeft {
				t.Fatalf("days left = %d, want %d", got.DaysLeft, c.wantDaysLeft)
			}
			if got.DaysUsed != c.wantDaysUsed {
				t.Fatalf("days used = %d, want %d", got.DaysUsed, c.wantDaysUsed)
			}
			if got.WindowDays != DefaultGenerationWindowDays {
				t.Fatalf("window = %d days", got.WindowDays)
			}
			if c.wantNoFlags && len(got.Flags) != 0 {
				t.Fatalf("flags = %v, want none", got.Flags)
			}
			for _, code := range c.wantFlags {
				if !hasFinding(got.Flags, code) {
					t.Fatalf("flag %q missing; got %v", code, got.Flags)
				}
			}
			if strings.TrimSpace(got.Headline) == "" || strings.TrimSpace(got.NextStep) == "" {
				t.Fatalf("an assessment with no words: %q / %q", got.Headline, got.NextStep)
			}
			if got.Stage.Label() == "" || got.Stage.Label() == string(got.Stage) {
				t.Fatalf("stage %q has no readable label", got.Stage)
			}
		})
	}
}

func TestAssessShowsBothClocksWhenTheyDisagree(t *testing.T) {
	e := assessEntry()
	e.Category = CategoryGSTTaxAudit
	e.SignedOn = MustDate(2026, 9, 10)
	e.EFilingSubmittedOn = MustDate(2026, 9, 15)

	got := Assess(e, DefaultSettings(), today)
	if got.GenerationDeadline.String() != "2026-11-08" {
		t.Fatalf("generation deadline = %s", got.GenerationDeadline)
	}
	if got.EFilingDeadline.String() != "2026-09-29" {
		t.Fatalf("e-filing deadline = %s", got.EFilingDeadline)
	}
	if got.Deadline.String() != "2026-09-29" || got.DeadlineRule != RuleEFiling {
		t.Fatalf("the binding deadline is %s by %q", got.Deadline, got.DeadlineRule)
	}
	if got.Stage != StageOverdue {
		t.Fatalf("stage = %q, want %q: the e-filing window shut in September", got.Stage, StageOverdue)
	}

	// Without a submission on the e-filing portal, only the ICAI clock runs.
	e.EFilingSubmittedOn = Date{}
	got = Assess(e, DefaultSettings(), today)
	if !got.EFilingDeadline.IsZero() || got.DeadlineRule != RuleGeneration {
		t.Fatalf("an e-filing deadline appeared from nowhere: %s / %q", got.EFilingDeadline, got.DeadlineRule)
	}
}

func TestAssessUsesTheOverriddenWindow(t *testing.T) {
	e := assessEntry()
	e.SignedOn = MustDate(2026, 10, 1)

	old := Settings{GenerationWindowDays: 15, EFilingWindowDays: 15, RevocationWindowHours: 48, WarnDays: 3}
	got := Assess(e, old, today)
	if got.Deadline.String() != "2026-10-15" {
		t.Fatalf("under a 15-day window the deadline is %s, want 2026-10-15", got.Deadline)
	}
	if got.Stage != StageOverdue || got.WindowDays != 15 {
		t.Fatalf("stage = %q under a %d-day window", got.Stage, got.WindowDays)
	}

	// The same entry is comfortably inside the 60-day window.
	if got := Assess(e, DefaultSettings(), today); got.Stage != StageAwaiting {
		t.Fatalf("stage = %q under the 60-day window", got.Stage)
	}

	// An empty settings block falls back to the reference values rather than
	// producing a deadline before the signing date.
	if got := Assess(e, Settings{}, today); got.Deadline.String() != "2026-11-29" {
		t.Fatalf("with empty settings the deadline is %s", got.Deadline)
	}
}

func TestAssessReportsTheRevocationWindow(t *testing.T) {
	cases := []struct {
		name      string
		generated Stamp
		want      RevokeWindow
		wantExact bool
	}{
		{name: "generated yesterday", generated: DateStamp(today.AddDays(-1)), want: RevokeOpen},
		{name: "generated the day before", generated: DateStamp(today.AddDays(-2)), want: RevokeClosingToday},
		{name: "generated last week", generated: DateStamp(today.AddDays(-7)), want: RevokeClosed},
		{
			name:      "generated yesterday at a known time",
			generated: mustStamp(t, today.AddDays(-1).String()+"T14:30"),
			want:      RevokeOpen, wantExact: true,
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			e := assessEntry()
			e.SignedOn = c.generated.Date()
			e.UDIN = "26304576AKTSBN1359"
			e.GeneratedAt = c.generated

			got := Assess(e, DefaultSettings(), today)
			if got.RevokeWindow != c.want {
				t.Fatalf("revocation window = %q, want %q (by %s)", got.RevokeWindow, c.want, got.RevokeBy)
			}
			if got.RevokeExact != c.wantExact {
				t.Fatalf("RevokeExact = %v, want %v", got.RevokeExact, c.wantExact)
			}
			if got.RevokeBy.IsZero() {
				t.Fatal("no revocation deadline was worked out")
			}
		})
	}

	// A document with no UDIN has no revocation window at all.
	if got := Assess(assessEntry(), DefaultSettings(), today); !got.RevokeBy.IsZero() || got.RevokeWindow != "" {
		t.Fatalf("an entry with no UDIN reported a revocation window: %s / %q", got.RevokeBy, got.RevokeWindow)
	}
}

func TestAssessHandlesAMalformedUDINInTheDataFile(t *testing.T) {
	// Put would never store this, but a hand-edited file can hold anything and
	// the register still has to say something sensible about it.
	e := withUDIN(assessEntry(), "NOT-A-UDIN", MustDate(2026, 9, 12))
	got := Assess(e, DefaultSettings(), today)
	if got.Severity != SeverityBreach {
		t.Fatalf("severity = %q for a malformed UDIN", got.Severity)
	}
	if !hasFinding(got.Flags, "udin-length") {
		t.Fatalf("flags = %v", got.Flags)
	}
	if !hasIntegrityFlag(got.Flags) {
		t.Fatal("a malformed UDIN was not counted as a problem with the record")
	}
}

func TestBoardOrdersByUrgencyAndSummarises(t *testing.T) {
	r := New()
	add := func(change func(*Entry)) {
		in := assessEntry()
		in.ID = ""
		change(&in)
		if _, err := r.Put(in, today); err != nil {
			t.Fatalf("Put: %v", err)
		}
	}

	add(func(e *Entry) { e.SignedOn = today })                                             // awaiting, notice
	add(func(e *Entry) { e.SignedOn = MustDate(2026, 10, 1) })                             // awaiting, warn
	add(func(e *Entry) { e.SignedOn = MustDate(2026, 8, 1) })                              // overdue, breach
	add(func(e *Entry) { *e = withUDIN(*e, "26304576AKTSBN1359", MustDate(2026, 9, 12)) }) // recorded
	add(func(e *Entry) { *e = withUDIN(*e, "26304576CKTSBN1361", MustDate(2026, 11, 9)) }) // late, warn
	add(func(e *Entry) {
		e.Category = CategoryCertificate
		e.FinancialYear = FinYear{start: 2018}
		e.SignedOn = MustDate(2019, 1, 1)
	}) // before the mandate

	revoked := assessEntry()
	revoked.ID = ""
	revoked.UDIN = "26304576DKTSBN1362"
	revoked.GeneratedAt = DateStamp(MustDate(2026, 9, 12))
	revoked.InCirculation = true
	stored, err := r.Put(revoked, today)
	if err != nil {
		t.Fatalf("Put: %v", err)
	}
	if _, err := r.Revoke(stored.ID, Revocation{
		RevokedAt: DateStamp(MustDate(2026, 9, 13)),
		Reason:    "The client restated a figure after the report was signed.",
	}, today); err != nil {
		t.Fatalf("Revoke: %v", err)
	}

	rows := Board(r, today)
	if len(rows) != 7 {
		t.Fatalf("the board has %d rows, want 7", len(rows))
	}
	for i := 1; i < len(rows); i++ {
		if rows[i-1].Assessment.Severity.Rank() < rows[i].Assessment.Severity.Rank() {
			t.Fatalf("row %d (%q) is louder than row %d (%q)",
				i, rows[i].Assessment.Severity, i-1, rows[i-1].Assessment.Severity)
		}
	}
	if rows[0].Assessment.Severity != SeverityBreach {
		t.Fatalf("the loudest row is %q", rows[0].Assessment.Severity)
	}

	counts := Summarise(rows)
	want := Counts{
		Total: 7, NoUDIN: 3, DueSoon: 1, DueToday: 0, Overdue: 1,
		Recorded: 1, Late: 1, Revoked: 1, RevokedInCirculation: 1,
		NotRequired: 1, Integrity: 1, Breaches: 2,
	}
	if counts != want {
		t.Fatalf("Summarise() = %+v, want %+v", counts, want)
	}

	filters := map[string]int{
		FilterAll:            7,
		FilterNoUDIN:         3,
		FilterDueSoon:        1,
		FilterOverdue:        1,
		FilterRecorded:       1,
		FilterLate:           1,
		FilterRevoked:        1,
		FilterInCirculation:  1,
		FilterIntegrity:      1,
		FilterNeedsAttention: 4,
		"something else":     7,
	}
	for filter, wantCount := range filters {
		if got := len(Filter(rows, filter)); got != wantCount {
			t.Fatalf("Filter(%q) returned %d rows, want %d", filter, got, wantCount)
		}
	}
	if len(AllFilters()) != 10 {
		t.Fatalf("AllFilters() lists %d filters", len(AllFilters()))
	}

	if got := Summarise(nil); got != (Counts{}) {
		t.Fatalf("Summarise(nil) = %+v", got)
	}
	if got := Board(New(), today); len(got) != 0 {
		t.Fatalf("Board on an empty register returned %d rows", len(got))
	}
}

func mustStamp(t *testing.T, raw string) Stamp {
	t.Helper()
	stamp, err := ParseStamp(raw)
	if err != nil {
		t.Fatalf("ParseStamp(%q): %v", raw, err)
	}
	return stamp
}
