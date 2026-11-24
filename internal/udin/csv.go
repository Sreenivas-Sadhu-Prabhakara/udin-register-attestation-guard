package udin

import (
	"encoding/csv"
	"fmt"
	"io"
	"strings"
)

// csvHeader is the register as a flat table. Every column is either something
// the member entered or something the rules derived from it, and the derived
// columns carry the window they were measured against, so the file still means
// something a year later when the window may have changed.
var csvHeader = []string{
	"Entry",
	"Category",
	"Document type",
	"Document description",
	"Client",
	"Client reference",
	"Financial year",
	"Assessment year",
	"Signed on",
	"Engagement partner",
	"Membership number",
	"UDIN",
	"UDIN generated on",
	"Days used (both ends)",
	"Window (days)",
	"Stage",
	"Deadline",
	"Deadline set by",
	"Days left",
	"E-filing submitted on",
	"E-filing update deadline",
	"Revoked on",
	"Revocation reason",
	"Replacement UDIN",
	"Document in circulation",
	"Document recalled",
	"Figures the UDIN was generated against",
	"Findings",
	"Notes",
	"Assessed on",
}

// WriteCSV writes the whole register in the order it was kept — oldest signing
// date first — with each entry's assessment beside it.
//
// This file is the answer to "produce your UDIN register", so it holds
// everything, including the entries whose UDIN is missing or revoked. A clean
// export would be a useless one.
func WriteCSV(w io.Writer, r *Register, asOf Date) error {
	writer := csv.NewWriter(w)
	if err := writer.Write(csvHeader); err != nil {
		return err
	}
	for _, e := range r.Entries {
		if err := writer.Write(csvRow(e, Assess(e, r.Settings, asOf), asOf)); err != nil {
			return err
		}
	}
	writer.Flush()
	return writer.Error()
}

func csvRow(e Entry, a Assessment, asOf Date) []string {
	revokedOn, reason, replacement, recalled := "", "", "", ""
	if e.Revoked() {
		revokedOn = e.Revocation.RevokedAt.Pretty()
		reason = e.Revocation.Reason
		replacement = e.Revocation.ReplacementUDIN
		recalled = yesNo(e.Revocation.Recalled)
	}

	daysUsed := ""
	if a.DaysUsed > 0 {
		daysUsed = fmt.Sprintf("%d", a.DaysUsed)
	}
	daysLeft := ""
	if !a.Deadline.IsZero() {
		daysLeft = fmt.Sprintf("%d", a.DaysLeft)
	}

	return []string{
		e.ID,
		e.Category.Label(),
		e.DocumentType,
		e.Description,
		e.ClientName,
		e.ClientRef,
		e.FinancialYear.Label(),
		e.FinancialYear.AssessmentYear(),
		e.SignedOn.Pretty(),
		e.PartnerName,
		e.PartnerMRN,
		e.UDIN,
		e.GeneratedAt.Pretty(),
		daysUsed,
		fmt.Sprintf("%d", a.WindowDays),
		a.Stage.Label(),
		a.Deadline.Pretty(),
		a.DeadlineRule,
		daysLeft,
		e.EFilingSubmittedOn.Pretty(),
		a.EFilingDeadline.Pretty(),
		revokedOn,
		reason,
		replacement,
		yesNo(e.InCirculation),
		recalled,
		FiguresText(e.Figures),
		FindingsText(a.Flags),
		e.Notes,
		asOf.Pretty(),
	}
}

// FiguresText renders the keyword and value pairs on one line, for the export
// and for a printed reminder.
func FiguresText(figures []Figure) string {
	parts := make([]string, 0, len(figures))
	for _, f := range figures {
		parts = append(parts, fmt.Sprintf("%s = %s", f.Keyword, f.Value))
	}
	return strings.Join(parts, "; ")
}

// FindingsText renders the findings on one line.
func FindingsText(findings []Finding) string {
	parts := make([]string, 0, len(findings))
	for _, f := range findings {
		parts = append(parts, fmt.Sprintf("[%s] %s", f.Severity, f.Message))
	}
	return strings.Join(parts, " ")
}

func yesNo(b bool) string {
	if b {
		return "yes"
	}
	return "no"
}
