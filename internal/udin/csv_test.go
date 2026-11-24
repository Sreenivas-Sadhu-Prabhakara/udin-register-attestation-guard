package udin

import (
	"encoding/csv"
	"strings"
	"testing"
)

func TestWriteCSVHoldsTheWholeRegister(t *testing.T) {
	r := New()
	if err := r.SetFirmName("Natarajan & Associates"); err != nil {
		t.Fatalf("SetFirmName: %v", err)
	}

	recorded := sampleEntry()
	recorded.UDIN = "26304576AKTSBN1359"
	recorded.GeneratedAt = DateStamp(MustDate(2026, 9, 12))
	if _, err := r.Put(recorded, today); err != nil {
		t.Fatalf("Put: %v", err)
	}

	missing := sampleEntry()
	missing.ClientName = "Anand Traders"
	missing.Category = CategoryGSTTaxAudit
	missing.DocumentType = "Form 3CB-3CD (tax audit report, section 44AB)"
	missing.SignedOn = MustDate(2026, 8, 1)
	missing.Description = ""
	missing.Figures = nil
	if _, err := r.Put(missing, today); err != nil {
		t.Fatalf("Put: %v", err)
	}

	revoked := sampleEntry()
	revoked.ClientName = "Zenith Mills Limited"
	revoked.UDIN = "26304576BKTSBN1360"
	revoked.GeneratedAt = DateStamp(MustDate(2026, 9, 12))
	revoked.InCirculation = true
	stored, err := r.Put(revoked, today)
	if err != nil {
		t.Fatalf("Put: %v", err)
	}
	if _, err := r.Revoke(stored.ID, Revocation{
		RevokedAt: DateStamp(MustDate(2026, 9, 13)),
		Reason:    "A figure was restated after the report was signed.",
	}, today); err != nil {
		t.Fatalf("Revoke: %v", err)
	}

	var out strings.Builder
	if err := WriteCSV(&out, r, today); err != nil {
		t.Fatalf("WriteCSV: %v", err)
	}

	records, err := csv.NewReader(strings.NewReader(out.String())).ReadAll()
	if err != nil {
		t.Fatalf("the export is not readable as CSV: %v", err)
	}
	if len(records) != 4 {
		t.Fatalf("the export has %d rows, want a header and three entries", len(records))
	}
	if strings.Join(records[0], "|") != strings.Join(csvHeader, "|") {
		t.Fatalf("the header row is %v", records[0])
	}

	column := func(name string) int {
		for i, heading := range csvHeader {
			if heading == name {
				return i
			}
		}
		t.Fatalf("there is no column called %q", name)
		return -1
	}
	cell := func(row int, name string) string { return records[row][column(name)] }

	// The register is kept in signing order, so the August entry comes first.
	if got := cell(1, "Client"); got != "Anand Traders" {
		t.Fatalf("the first exported row is for %q", got)
	}
	if got := cell(1, "Stage"); got != StageOverdue.Label() {
		t.Fatalf("the missing UDIN is exported as %q", got)
	}
	if got := cell(1, "UDIN"); got != "" {
		t.Fatalf("a missing UDIN was exported as %q", got)
	}
	if got := cell(1, "Deadline"); got != "29 Sep 2026" {
		t.Fatalf("the deadline is exported as %q", got)
	}
	if got := cell(1, "Days left"); got != "-56" {
		t.Fatalf("the days left are exported as %q", got)
	}
	if got := cell(1, "Window (days)"); got != "60" {
		t.Fatalf("the window is exported as %q", got)
	}
	if got := cell(1, "Category"); got != "GST and tax audit" {
		t.Fatalf("the category is exported as %q", got)
	}

	if got := cell(2, "UDIN"); got != "26304576AKTSBN1359" {
		t.Fatalf("the recorded UDIN is exported as %q", got)
	}
	if got := cell(2, "Days used (both ends)"); got != "3" {
		t.Fatalf("the days used are exported as %q", got)
	}
	if got := cell(2, "Financial year"); got != "2025-26" {
		t.Fatalf("the financial year is exported as %q", got)
	}
	if got := cell(2, "Assessment year"); got != "2026-27" {
		t.Fatalf("the assessment year is exported as %q", got)
	}
	if got := cell(2, "Figures the UDIN was generated against"); !strings.Contains(got, "Turnover = 48,20,15,000") {
		t.Fatalf("the figures are exported as %q", got)
	}
	if got := cell(2, "Signed on"); got != "10 Sep 2026" {
		t.Fatalf("the signing date is exported as %q", got)
	}
	if got := cell(2, "Membership number"); got != "304576" {
		t.Fatalf("the membership number is exported as %q", got)
	}

	if got := cell(3, "Stage"); got != StageRevoked.Label() {
		t.Fatalf("the revoked entry is exported as %q", got)
	}
	if got := cell(3, "Revoked on"); got != "13 Sep 2026" {
		t.Fatalf("the revocation date is exported as %q", got)
	}
	if got := cell(3, "Revocation reason"); got != "A figure was restated after the report was signed." {
		t.Fatalf("the reason is exported as %q", got)
	}
	if got := cell(3, "Document in circulation"); got != "yes" {
		t.Fatalf("the circulation column says %q", got)
	}
	if got := cell(3, "Document recalled"); got != "no" {
		t.Fatalf("the recall column says %q", got)
	}
	if got := cell(3, "Findings"); !strings.Contains(got, "revoked") {
		t.Fatalf("the findings column says %q", got)
	}
	if got := cell(3, "Assessed on"); got != "24 Nov 2026" {
		t.Fatalf("the assessment date is exported as %q", got)
	}
}

func TestWriteCSVOnAnEmptyRegisterIsJustTheHeader(t *testing.T) {
	var out strings.Builder
	if err := WriteCSV(&out, New(), today); err != nil {
		t.Fatalf("WriteCSV: %v", err)
	}
	records, err := csv.NewReader(strings.NewReader(out.String())).ReadAll()
	if err != nil {
		t.Fatalf("the export is not readable as CSV: %v", err)
	}
	if len(records) != 1 || len(records[0]) != len(csvHeader) {
		t.Fatalf("an empty register exported %d rows", len(records))
	}
}

func TestFiguresAndFindingsText(t *testing.T) {
	if got := FiguresText(nil); got != "" {
		t.Fatalf("FiguresText(nil) = %q", got)
	}
	figures := []Figure{{Keyword: "Turnover", Value: "1,00,000"}, {Keyword: "Net worth", Value: "50,000"}}
	if got := FiguresText(figures); got != "Turnover = 1,00,000; Net worth = 50,000" {
		t.Fatalf("FiguresText() = %q", got)
	}
	if got := FindingsText(nil); got != "" {
		t.Fatalf("FindingsText(nil) = %q", got)
	}
	findings := []Finding{{Code: "a", Severity: SeverityWarn, Message: "First."}, {Code: "b", Severity: SeverityNotice, Message: "Second."}}
	if got := FindingsText(findings); got != "[warn] First. [notice] Second." {
		t.Fatalf("FindingsText() = %q", got)
	}
}
