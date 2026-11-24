package udin

import (
	"fmt"
	"sort"
	"strings"
)

// Stage is where one document has got to.
type Stage string

const (
	// StageAwaiting is signed, no UDIN yet, window still open.
	StageAwaiting Stage = "awaiting"
	// StageOverdue is signed, no UDIN, window closed.
	StageOverdue Stage = "overdue"
	// StageRecorded is a UDIN generated inside the window.
	StageRecorded Stage = "recorded"
	// StageLate is a UDIN generated after the window had closed — a fact the
	// register keeps rather than rounds off.
	StageLate Stage = "late"
	// StageRevoked is a UDIN that has been withdrawn.
	StageRevoked Stage = "revoked"
	// StageNotRequired is a document signed before a UDIN was mandatory for
	// its category.
	StageNotRequired Stage = "not-required"
)

// Label is the stage as the interface prints it.
func (s Stage) Label() string {
	switch s {
	case StageAwaiting:
		return "UDIN pending"
	case StageOverdue:
		return "Window closed"
	case StageRecorded:
		return "UDIN on record"
	case StageLate:
		return "Generated late"
	case StageRevoked:
		return "Revoked"
	case StageNotRequired:
		return "Before the mandate"
	}
	return string(s)
}

// Names of the clocks, used in Assessment.DeadlineRule so the interface can say
// which rule produced the date on screen.
const (
	RuleGeneration = "ICAI generation window"
	RuleEFiling    = "e-filing UDIN update"
	RuleNone       = ""
)

// RevokeWindow says whether the portal would still accept a revocation.
type RevokeWindow string

const (
	// RevokeOpen means the 48 hours have not run out.
	RevokeOpen RevokeWindow = "open"
	// RevokeClosingToday means they run out at some point today.
	RevokeClosingToday RevokeWindow = "closing-today"
	// RevokeClosed means the portal will no longer revoke this UDIN.
	RevokeClosed RevokeWindow = "closed"
)

// Assessment is everything the rules have to say about one entry on one day.
// It holds no text the interface has to parse apart from the two sentences
// meant to be read.
type Assessment struct {
	ID       string   `json:"id"`
	Stage    Stage    `json:"stage"`
	Severity Severity `json:"severity"`
	Headline string   `json:"headline"`
	NextStep string   `json:"nextStep"`

	// Deadline is the last day a UDIN may be generated for this document, and
	// DeadlineRule names the clock that set it.
	Deadline     Date   `json:"deadline"`
	DeadlineRule string `json:"deadlineRule"`
	// DaysLeft counts from the day being assessed to the deadline: 0 is the
	// last day, negative is past it.
	DaysLeft int `json:"daysLeft"`
	// WindowDays is the window that was applied, so a register exported today
	// still says what rule it was measured against.
	WindowDays int `json:"windowDays"`
	// DaysUsed is how much of the window a generated UDIN consumed, counting
	// the signing day and the generation day.
	DaysUsed int `json:"daysUsed"`

	// GenerationDeadline and EFilingDeadline are both shown when the two
	// clocks disagree, which is the case the firm needs to see.
	GenerationDeadline Date `json:"generationDeadline"`
	EFilingDeadline    Date `json:"eFilingDeadline"`

	// RevokeBy is when the portal's revocation window shuts, RevokeExact is
	// false when only the generation date was recorded and the moment is
	// therefore the earliest it could be.
	RevokeBy     Stamp        `json:"revokeBy"`
	RevokeExact  bool         `json:"revokeExact"`
	RevokeWindow RevokeWindow `json:"revokeWindow"`

	Flags []Finding `json:"flags"`
}

// Assess applies every rule in this package to one entry as at asOf.
//
// It is a pure function: same entry, same settings, same day, same answer. The
// day is a parameter rather than a call to the clock so that a reminder printed
// this morning can be reproduced exactly.
func Assess(e Entry, settings Settings, asOf Date) Assessment {
	s := settings.withDefaults()
	a := Assessment{
		ID:         e.ID,
		WindowDays: s.GenerationWindowDays,
		Flags:      []Finding{},
	}

	a.GenerationDeadline = GenerationDeadline(e.SignedOn, s.GenerationWindowDays)
	if !e.EFilingSubmittedOn.IsZero() {
		a.EFilingDeadline = EFilingDeadline(e.EFilingSubmittedOn, s.EFilingWindowDays)
	}

	a.Deadline, a.DeadlineRule = effectiveDeadline(a.GenerationDeadline, a.EFilingDeadline)
	if !a.Deadline.IsZero() && !asOf.IsZero() {
		a.DaysLeft = DaysBetween(asOf, a.Deadline)
	}
	if e.HasUDIN() && !e.GeneratedAt.IsZero() {
		a.DaysUsed = DaysUsed(e.SignedOn, e.GeneratedAt.Date())
	}

	a.Flags = append(a.Flags, recordFlags(e, s)...)

	switch {
	case e.Revoked():
		a.Stage = StageRevoked
		a.Flags = append(a.Flags, revocationFlags(e, s)...)
		if e.InCirculation && !e.Revocation.Recalled {
			a.Headline = "Revoked, and the document is still out"
			a.NextStep = "Recall the document from whoever holds it, reissue it under a fresh UDIN, and mark it recalled here."
		} else {
			a.Headline = fmt.Sprintf("UDIN revoked on %s", e.Revocation.RevokedAt.Date().Pretty())
			a.NextStep = "Nothing further, unless the document is reissued — then record the fresh UDIN as a new entry."
		}

	case e.HasUDIN():
		withinWindow := WithinWindow(e.SignedOn, e.GeneratedAt.Date(), s.GenerationWindowDays)
		if withinWindow {
			a.Stage = StageRecorded
			a.Headline = fmt.Sprintf("UDIN on record, day %d of %d", a.DaysUsed, s.GenerationWindowDays)
			a.NextStep = "Nothing to do. Keep the figures and the signed copy together in the file."
		} else {
			a.Stage = StageLate
			a.Headline = fmt.Sprintf("UDIN generated on day %d of a %d-day window", a.DaysUsed, s.GenerationWindowDays)
			a.NextStep = "Leave the record as it stands. Note on the entry why the number was late, in case it is ever asked about."
			a.Flags = append(a.Flags, Finding{
				Code:     "generated-late",
				Severity: SeverityWarn,
				Message: fmt.Sprintf("Signed on %s, UDIN generated on %s: %d days counting both ends, against a window of %d.",
					e.SignedOn.Pretty(), e.GeneratedAt.Date().Pretty(), a.DaysUsed, s.GenerationWindowDays),
			})
		}
		a.RevokeBy, a.RevokeExact = RevocationDeadline(e.GeneratedAt, s.RevocationWindowHours)
		a.RevokeWindow = revokeWindowState(a.RevokeBy, asOf)

	case !e.Category.WasMandatory(e.SignedOn):
		a.Stage = StageNotRequired
		a.Deadline, a.DeadlineRule, a.DaysLeft = Date{}, RuleNone, 0
		a.Headline = fmt.Sprintf("Signed before UDIN was mandatory for a %s", lowerFirst(e.Category.Label()))
		a.NextStep = fmt.Sprintf("Nothing to do: a UDIN became mandatory for this category from %s.", e.Category.MandatoryFrom().Pretty())

	case a.DaysLeft < 0:
		a.Stage = StageOverdue
		a.Headline = fmt.Sprintf("No UDIN, and the window shut %s ago", days(-a.DaysLeft))
		a.NextStep = "Take this to the UDIN Directorate: a UDIN cannot be generated for this signing date any more. Record here what was done."

	default:
		a.Stage = StageAwaiting
		switch {
		case a.DaysLeft == 0:
			a.Headline = "No UDIN, and today is the last day"
		case a.DaysLeft == 1:
			a.Headline = "No UDIN, one day left"
		default:
			a.Headline = fmt.Sprintf("No UDIN, %d days left", a.DaysLeft)
		}
		a.NextStep = fmt.Sprintf("Generate the UDIN on the ICAI portal by %s and enter it here.", a.Deadline.Pretty())
	}

	a.Severity = severityFor(a, e, s)
	return a
}

// effectiveDeadline picks the clock that binds. A tax audit report uploaded to
// the e-filing portal without its UDIN has fifteen days from submission, which
// almost always runs out well before the sixty days from signing — and the
// consequence there is harder than a reprimand: the uploaded report is treated
// as an invalid submission.
func effectiveDeadline(generation, efiling Date) (Date, string) {
	switch {
	case generation.IsZero() && efiling.IsZero():
		return Date{}, RuleNone
	case efiling.IsZero():
		return generation, RuleGeneration
	case generation.IsZero():
		return efiling, RuleEFiling
	case efiling.Before(generation):
		return efiling, RuleEFiling
	default:
		return generation, RuleGeneration
	}
}

// recordFlags are the findings that do not depend on which stage the document
// has reached: a UDIN that is not shaped right, a membership number that is not
// the partner's, a description or figures the register never captured.
func recordFlags(e Entry, s Settings) []Finding {
	var flags []Finding
	if e.HasUDIN() {
		parsed, findings := ParseUDIN(e.UDIN)
		flags = append(flags, findings...)
		flags = append(flags, CrossCheck(parsed, e.SignedOn, e.PartnerMRN, s.GenerationWindowDays)...)

		if e.Description == "" {
			flags = append(flags, Finding{
				Code:     "no-description",
				Severity: SeverityNotice,
				Message:  "No document description on record. The portal asks for 5 to 250 characters from the document, and it is what ties this UDIN to this version of it.",
			})
		}
		if len(e.Figures) < figuresExpected {
			flags = append(flags, Finding{
				Code:     "few-figures",
				Severity: SeverityNotice,
				Message:  fmt.Sprintf("%s on record. The portal is given at least two figures with their keywords, and nothing is uploaded, so those figures are the only link between the number and the document.", figureCount(len(e.Figures))),
			})
		}
	}
	if e.PartnerName == "" && e.PartnerMRN == "" {
		flags = append(flags, Finding{
			Code:     "no-partner",
			Severity: SeverityNotice,
			Message:  "No engagement partner on record. A UDIN is generated under one member's number, and the register should say whose.",
		})
	}
	if !e.FinancialYear.IsZero() && !e.SignedOn.IsZero() && e.FinancialYear.Start().After(e.SignedOn) {
		flags = append(flags, Finding{
			Code:     "financial-year-ahead",
			Severity: SeverityWarn,
			Message: fmt.Sprintf("The document is recorded against FY %s, which only begins on %s — after it was signed on %s.",
				e.FinancialYear.Label(), e.FinancialYear.Start().Pretty(), e.SignedOn.Pretty()),
		})
	}
	return flags
}

// revocationFlags cover the two things that make a revocation worse than it
// looks: a document still in somebody's hands, and a revocation made after the
// portal's 48-hour window, which means the number is still live for anyone who
// verifies it.
func revocationFlags(e Entry, s Settings) []Finding {
	var flags []Finding
	if e.Revocation == nil {
		return nil
	}
	if e.InCirculation && !e.Revocation.Recalled {
		flags = append(flags, Finding{
			Code:     "revoked-in-circulation",
			Severity: SeverityBreach,
			Message:  "This document is recorded as being in circulation under a revoked UDIN. Whoever holds it is relying on a number that has been withdrawn.",
		})
	}
	if deadline, _ := RevocationDeadline(e.GeneratedAt, s.RevocationWindowHours); !deadline.IsZero() {
		if deadline.Before(e.Revocation.RevokedAt) {
			flags = append(flags, Finding{
				Code:     "revoked-after-portal-window",
				Severity: SeverityWarn,
				Message: fmt.Sprintf("The revocation is dated %s, after the portal's %d-hour window shut on %s. If it was not revoked on the portal in time, the UDIN is still live there and anyone verifying it will be told the document is genuine.",
					e.Revocation.RevokedAt.Pretty(), s.RevocationWindowHours, deadline.Pretty()),
			})
		}
	}
	if e.Revocation.ReplacementUDIN == "" {
		flags = append(flags, Finding{
			Code:     "no-replacement",
			Severity: SeverityNotice,
			Message:  "No replacement UDIN on record. If the document was reissued, the fresh number belongs in the register as its own entry.",
		})
	}
	return flags
}

// severityFor turns a stage and its findings into one level for the row. The
// loudest finding always wins, so an entry whose UDIN carries another member's
// number cannot sit quietly in the register because its deadline was met.
func severityFor(a Assessment, e Entry, s Settings) Severity {
	base := SeverityOK
	switch a.Stage {
	case StageAwaiting:
		switch {
		case a.DaysLeft == 0:
			base = SeverityUrgent
		case a.DaysLeft <= s.WarnDays:
			base = SeverityWarn
		default:
			base = SeverityNotice
		}
	case StageOverdue:
		base = SeverityBreach
	case StageLate:
		base = SeverityWarn
	case StageRevoked:
		base = SeverityNotice
		if e.InCirculation && e.Revocation != nil && !e.Revocation.Recalled {
			base = SeverityBreach
		}
	case StageRecorded, StageNotRequired:
		base = SeverityOK
	}
	return Worse(base, worstOf(a.Flags))
}

func revokeWindowState(deadline Stamp, asOf Date) RevokeWindow {
	if deadline.IsZero() || asOf.IsZero() {
		return RevokeClosed
	}
	switch {
	case deadline.Date().After(asOf):
		return RevokeOpen
	case deadline.Date().Equal(asOf):
		return RevokeClosingToday
	default:
		return RevokeClosed
	}
}

// Row is one entry with its assessment, which is what every screen and every
// export is built from.
type Row struct {
	Entry      Entry      `json:"entry"`
	Assessment Assessment `json:"assessment"`
}

// Board assesses the whole register as at asOf and puts the rows in the order
// they need attention: loudest first, then by deadline, then by entry.
func Board(r *Register, asOf Date) []Row {
	rows := make([]Row, 0, len(r.Entries))
	for _, e := range r.Entries {
		rows = append(rows, Row{Entry: e, Assessment: Assess(e, r.Settings, asOf)})
	}
	sort.SliceStable(rows, func(i, j int) bool {
		a, b := rows[i].Assessment, rows[j].Assessment
		if a.Severity.Rank() != b.Severity.Rank() {
			return a.Severity.Rank() > b.Severity.Rank()
		}
		switch {
		case a.Deadline.IsZero() && !b.Deadline.IsZero():
			return false
		case !a.Deadline.IsZero() && b.Deadline.IsZero():
			return true
		case !a.Deadline.Equal(b.Deadline):
			return a.Deadline.Before(b.Deadline)
		}
		return idNumber(a.ID) < idNumber(b.ID)
	})
	return rows
}

// Counts is the headline summary: the numbers a partner wants before opening
// anything.
type Counts struct {
	Total int `json:"total"`
	// NoUDIN is every signed document with no UDIN recorded, whether or not
	// its window has closed. This is the number that matters.
	NoUDIN               int `json:"noUdin"`
	DueSoon              int `json:"dueSoon"`
	DueToday             int `json:"dueToday"`
	Overdue              int `json:"overdue"`
	Recorded             int `json:"recorded"`
	Late                 int `json:"late"`
	Revoked              int `json:"revoked"`
	RevokedInCirculation int `json:"revokedInCirculation"`
	NotRequired          int `json:"notRequired"`
	// Integrity counts rows carrying a finding about the record itself: a
	// malformed UDIN, a membership number that is not the signing partner's.
	Integrity int `json:"integrity"`
	// Breaches counts rows at the loudest severity, whatever caused it.
	Breaches int `json:"breaches"`
}

// Summarise counts a board.
func Summarise(rows []Row) Counts {
	var c Counts
	c.Total = len(rows)
	for _, row := range rows {
		a := row.Assessment
		switch a.Stage {
		case StageAwaiting:
			c.NoUDIN++
			if a.DaysLeft == 0 {
				c.DueToday++
			} else if a.Severity == SeverityWarn || a.Severity == SeverityUrgent {
				c.DueSoon++
			}
		case StageOverdue:
			c.NoUDIN++
			c.Overdue++
		case StageRecorded:
			c.Recorded++
		case StageLate:
			c.Late++
		case StageRevoked:
			c.Revoked++
			if hasFlag(a.Flags, "revoked-in-circulation") {
				c.RevokedInCirculation++
			}
		case StageNotRequired:
			c.NotRequired++
		}
		if hasIntegrityFlag(a.Flags) {
			c.Integrity++
		}
		if a.Severity == SeverityBreach {
			c.Breaches++
		}
	}
	return c
}

// Filters are the views the interface offers. Each one answers a question
// somebody actually asks.
const (
	FilterAll            = "all"
	FilterNoUDIN         = "no-udin"
	FilterDueSoon        = "due-soon"
	FilterOverdue        = "overdue"
	FilterRecorded       = "recorded"
	FilterLate           = "late"
	FilterRevoked        = "revoked"
	FilterInCirculation  = "in-circulation"
	FilterIntegrity      = "integrity"
	FilterNeedsAttention = "needs-attention"
)

// AllFilters lists the filters in the order the interface shows them.
func AllFilters() []string {
	return []string{
		FilterNeedsAttention, FilterAll, FilterNoUDIN, FilterDueSoon, FilterOverdue,
		FilterRecorded, FilterLate, FilterRevoked, FilterInCirculation, FilterIntegrity,
	}
}

// Filter narrows a board. An unknown filter returns everything rather than
// nothing, so a stale bookmark shows the register instead of an empty page.
func Filter(rows []Row, filter string) []Row {
	out := make([]Row, 0, len(rows))
	for _, row := range rows {
		if keepRow(row, filter) {
			out = append(out, row)
		}
	}
	return out
}

func keepRow(row Row, filter string) bool {
	a := row.Assessment
	switch filter {
	case FilterNoUDIN:
		return a.Stage == StageAwaiting || a.Stage == StageOverdue
	case FilterDueSoon:
		return a.Stage == StageAwaiting && (a.Severity == SeverityWarn || a.Severity == SeverityUrgent)
	case FilterOverdue:
		return a.Stage == StageOverdue
	case FilterRecorded:
		return a.Stage == StageRecorded
	case FilterLate:
		return a.Stage == StageLate
	case FilterRevoked:
		return a.Stage == StageRevoked
	case FilterInCirculation:
		return hasFlag(a.Flags, "revoked-in-circulation")
	case FilterIntegrity:
		return hasIntegrityFlag(a.Flags)
	case FilterNeedsAttention:
		return a.Severity.Rank() >= SeverityWarn.Rank()
	default:
		return true
	}
}

// Search narrows a board to the rows that mention the query, across the fields
// somebody would actually search a register by: the client, the document, the
// UDIN, the partner, the financial year and the client's reference.
//
// A query that looks like a UDIN is normalised first, so searching for a number
// copied out of an email with its spaces intact still finds the row.
func Search(rows []Row, query string) []Row {
	needle := strings.ToLower(strings.TrimSpace(query))
	if needle == "" {
		return rows
	}
	normalised := strings.ToLower(NormaliseUDIN(query))

	out := make([]Row, 0, len(rows))
	for _, row := range rows {
		e := row.Entry
		haystack := strings.ToLower(strings.Join([]string{
			e.ID, e.ClientName, e.ClientRef, e.DocumentType, e.Description,
			e.PartnerName, e.PartnerMRN, e.UDIN, e.FinancialYear.Label(),
			e.Category.Label(), e.Notes, row.Assessment.Stage.Label(),
		}, "\u0000"))
		if strings.Contains(haystack, needle) || (normalised != "" && strings.Contains(haystack, normalised)) {
			out = append(out, row)
		}
	}
	return out
}

// integrityFlagCodes are the findings that say the record itself does not add
// up, as opposed to a deadline approaching.
var integrityFlagCodes = map[string]bool{
	"udin-characters":             true,
	"udin-length":                 true,
	"udin-year-digits":            true,
	"udin-membership-digits":      true,
	"udin-random-part-shape":      true,
	"udin-mrn-mismatch":           true,
	"udin-year-mismatch":          true,
	"financial-year-ahead":        true,
	"revoked-in-circulation":      true,
	"revoked-after-portal-window": true,
}

func hasIntegrityFlag(flags []Finding) bool {
	for _, f := range flags {
		if integrityFlagCodes[f.Code] {
			return true
		}
	}
	return false
}

func hasFlag(flags []Finding, code string) bool {
	for _, f := range flags {
		if f.Code == code {
			return true
		}
	}
	return false
}

func days(n int) string {
	if n == 1 {
		return "1 day"
	}
	return fmt.Sprintf("%d days", n)
}

func figureCount(n int) string {
	switch n {
	case 0:
		return "No figures"
	case 1:
		return "One figure"
	}
	return fmt.Sprintf("%d figures", n)
}

func lowerFirst(s string) string {
	if s == "" {
		return s
	}
	runes := []rune(s)
	if runes[0] >= 'A' && runes[0] <= 'Z' {
		runes[0] = runes[0] + 32
	}
	return string(runes)
}
