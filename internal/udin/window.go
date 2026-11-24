package udin

import "fmt"

// The three windows this register counts, and where each one comes from.
//
// Every figure below is a reference value held in one place and overridable in
// the tool's settings, because they are council decisions and press releases
// rather than statute: they have changed before (the UDIN window was 15 days
// until September 2021) and they can change again. Treat them as indicative,
// confirm the current position at udin.icai.org, and change the setting rather
// than the document if ICAI moves the line.
const (
	// DefaultGenerationWindowDays is the outer limit for generating a UDIN
	// after signing a document.
	//
	// Source: ICAI announcement "Aligning the time limit of generating UDIN
	// from 15 days to 60 days", 17 September 2021 — the Council at its 405th
	// meeting held on 17 September 2021 decided the time limit would be 60
	// days from the date of signing the certificate, report or document,
	// instead of 15 days. The change was aligned with SQC 1 and SA 230 on
	// assembly of the audit file.
	//
	// The ICAI FAQ is explicit that this is an outer limit, not the expected
	// practice: a UDIN is to be generated at the time of signing, and where a
	// regulator or other stakeholder needs it on signing or inside a shorter
	// period, the member must provide it then.
	DefaultGenerationWindowDays = 60

	// DefaultEFilingWindowDays is the separate, shorter clock that applies to
	// a document submitted on the income-tax e-filing portal before its UDIN
	// existed.
	//
	// Source: CBDT press release on the e-filing portal's integration with
	// ICAI for UDIN validation — where a chartered accountant could not
	// generate the UDIN before submission, the portal permits submission
	// subject to the UDIN being updated against the form within 15 calendar
	// days from the date of submission; if it is not, the uploaded report or
	// certificate is treated as an invalid submission.
	DefaultEFilingWindowDays = 15

	// DefaultRevocationWindowHours is how long a generated UDIN can be
	// revoked on the portal.
	//
	// Source: ICAI announcement "Restricting revocation of the UDINs within
	// 48 hours", 23 June 2023 — the Council at its 420th meeting held on
	// 23-24 March 2023 decided that revocation would be possible only within
	// 48 hours from the time of generation. After that the portal tells a
	// verifier the UDIN can no longer be revoked, and the FAQ's answer to a
	// member who missed the window is to generate a fresh UDIN inside the
	// generation window instead.
	DefaultRevocationWindowHours = 48

	// DefaultWarnDays is how early the register starts calling a pending UDIN
	// urgent. This one is a working preference, not a rule from anybody.
	DefaultWarnDays = 10
)

// Sources for the three windows, shown in the interface next to the settings so
// a user can see what each number rests on before changing it.
const (
	GenerationWindowSource = "ICAI announcement of 17 September 2021 (Council, 405th meeting): 60 days from the date of signing, replacing 15 days."
	EFilingWindowSource    = "CBDT press release on UDIN validation at the income-tax e-filing portal: 15 calendar days from the date of form submission."
	RevocationWindowSource = "ICAI announcement of 23 June 2023 (Council, 420th meeting): revocation possible only within 48 hours of generation."
	CountingSource         = "ICAI FAQ on UDIN: both the date of signing and the date of generation are included in the time allowed."
)

// Settings holds the overridable windows for one register. Nothing in this
// package reads the constants above directly except Settings.
type Settings struct {
	GenerationWindowDays  int `json:"generationWindowDays"`
	EFilingWindowDays     int `json:"eFilingWindowDays"`
	RevocationWindowHours int `json:"revocationWindowHours"`
	WarnDays              int `json:"warnDays"`
}

// DefaultSettings returns the reference values.
func DefaultSettings() Settings {
	return Settings{
		GenerationWindowDays:  DefaultGenerationWindowDays,
		EFilingWindowDays:     DefaultEFilingWindowDays,
		RevocationWindowHours: DefaultRevocationWindowHours,
		WarnDays:              DefaultWarnDays,
	}
}

// Validate keeps an override inside the range where the arithmetic still means
// something. A window of zero days or a warning further out than the window
// itself would quietly break every alert in the register.
func (s Settings) Validate() error {
	if s.GenerationWindowDays < 2 || s.GenerationWindowDays > 365 {
		return fmt.Errorf("the generation window must be between 2 and 365 days, not %d", s.GenerationWindowDays)
	}
	if s.EFilingWindowDays < 1 || s.EFilingWindowDays > 365 {
		return fmt.Errorf("the e-filing update window must be between 1 and 365 days, not %d", s.EFilingWindowDays)
	}
	if s.RevocationWindowHours < 1 || s.RevocationWindowHours > 8760 {
		return fmt.Errorf("the revocation window must be between 1 and 8760 hours, not %d", s.RevocationWindowHours)
	}
	if s.WarnDays < 1 || s.WarnDays >= s.GenerationWindowDays {
		return fmt.Errorf("the early warning must be between 1 and %d days, not %d", s.GenerationWindowDays-1, s.WarnDays)
	}
	return nil
}

// withDefaults fills in anything a hand-edited data file left at zero, so an
// incomplete settings block cannot turn into a deadline of "the day before
// signing".
func (s Settings) withDefaults() Settings {
	defaults := DefaultSettings()
	if s.GenerationWindowDays < 2 {
		s.GenerationWindowDays = defaults.GenerationWindowDays
	}
	if s.EFilingWindowDays < 1 {
		s.EFilingWindowDays = defaults.EFilingWindowDays
	}
	if s.RevocationWindowHours < 1 {
		s.RevocationWindowHours = defaults.RevocationWindowHours
	}
	if s.WarnDays < 1 || s.WarnDays >= s.GenerationWindowDays {
		s.WarnDays = defaults.WarnDays
		if s.WarnDays >= s.GenerationWindowDays {
			s.WarnDays = s.GenerationWindowDays - 1
		}
	}
	return s
}

// GenerationDeadline is the last day on which a UDIN may be generated for a
// document signed on signedOn.
//
// Both the signing date and the generation date count towards the window, so a
// 60-day window on a document signed on the 1st of January ends on the 60th day
// of the year and not the 61st: the deadline is signedOn plus windowDays minus
// one. Getting this wrong by a day is the single most common error in a
// hand-kept UDIN sheet, and it errs in the direction that breaches the rule.
func GenerationDeadline(signedOn Date, windowDays int) Date {
	if signedOn.IsZero() || windowDays < 1 {
		return Date{}
	}
	return signedOn.AddDays(windowDays - 1)
}

// DaysUsed is how many days of the window a UDIN generated on generatedOn
// consumed, counting both ends. Signed and generated the same day is 1.
func DaysUsed(signedOn, generatedOn Date) int {
	if signedOn.IsZero() || generatedOn.IsZero() {
		return 0
	}
	return InclusiveDays(signedOn, generatedOn)
}

// WithinWindow reports whether a UDIN generated on generatedOn was inside the
// window for a document signed on signedOn. A generation date before the
// signing date is not within the window: it is a data-entry error, and saying
// so is more useful than averaging it away.
func WithinWindow(signedOn, generatedOn Date, windowDays int) bool {
	used := DaysUsed(signedOn, generatedOn)
	return used >= 1 && used <= windowDays
}

// EFilingDeadline is the last day to update a UDIN against a form already
// submitted on the income-tax e-filing portal.
//
// The CBDT press release says "within 15 calendar days from the date of form
// submission" and does not say whether the submission date itself counts. This
// register counts it, which is the earlier of the two readings and therefore
// the safe one. A firm that has confirmed the other reading on the portal can
// set the window to 16 days instead.
func EFilingDeadline(submittedOn Date, windowDays int) Date {
	if submittedOn.IsZero() || windowDays < 1 {
		return Date{}
	}
	return submittedOn.AddDays(windowDays - 1)
}

// RevocationDeadline is the moment the portal stops accepting a revocation of a
// UDIN generated at generatedAt.
//
// When only the date of generation was recorded, the stamp is read as midnight,
// which is the earliest the UDIN could have been generated that day. The result
// is then the earliest the window could shut, and exact is false — the
// interface says "no later than" rather than putting a false time on screen.
func RevocationDeadline(generatedAt Stamp, windowHours int) (deadline Stamp, exact bool) {
	if generatedAt.IsZero() || windowHours < 1 {
		return Stamp{}, false
	}
	return generatedAt.AddHours(windowHours), generatedAt.Exact()
}
