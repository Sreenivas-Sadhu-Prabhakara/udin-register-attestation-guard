package udin

import (
	"fmt"
	"strings"
)

// UDINLength is the number of characters in a UDIN.
//
// Source: ICAI FAQ on UDIN, Q1 and Q4 — a UDIN is an 18-digit system-generated
// number with the syntax YY MMMMMM AAAAAANNNN, for example 19304576AKTSBN1359,
// where the first two digits are the last two of the year of generation, the
// next six are the member's ICAI membership number, and the last ten are
// alphanumeric characters generated at random by the portal.
const UDINLength = 18

const (
	udinYearDigits       = 2
	udinMembershipDigits = 6
	udinRandomChars      = 10
)

// FormatSource is shown next to the validator in the interface, so a user can
// see exactly how far the shape check goes.
const FormatSource = "ICAI FAQ on UDIN, Q4: an 18-character UDIN with the syntax YY MMMMMM AAAAAANNNN (year, membership number, random part)."

// NoCheckDigitNote is the limit of this validator, stated plainly because the
// alternative — a plausible-looking arithmetic check — would reject real UDINs
// and pass invented ones.
const NoCheckDigitNote = "ICAI does not publish a check digit or checksum for a UDIN, so no arithmetic here can tell a well-formed invention from a real number. Only verification at udin.icai.org can do that."

// UDIN is a parsed UDIN. It is a shape, not a verified number: everything in
// this file checks the pattern ICAI documents and nothing more.
type UDIN struct {
	// Value is the normalised 18-character form that gets stored.
	Value string `json:"value"`
	// YearDigits is the two-digit year of generation, "26" for 2026.
	YearDigits string `json:"yearDigits"`
	// MembershipNumber is the six-digit ICAI membership number of the member
	// who generated it.
	MembershipNumber string `json:"membershipNumber"`
	// RandomPart is the last ten characters, which the portal generates.
	RandomPart string `json:"randomPart"`
}

// NormaliseUDIN strips the separators people add when copying a UDIN out of a
// report footer or an email and upper-cases the letters. A UDIN is often
// written in three spaced groups, and a register that treated "19304576 AKTSBN
// 1359" as a different number from 19304576AKTSBN1359 would report duplicates
// and misses that are not real.
func NormaliseUDIN(raw string) string {
	var b strings.Builder
	for _, r := range strings.ToUpper(raw) {
		switch {
		case r >= '0' && r <= '9', r >= 'A' && r <= 'Z':
			b.WriteRune(r)
		case r == ' ', r == '-', r == '\t', r == ' ', r == '/', r == '.', r == ':':
			// Separators are dropped.
		default:
			// Anything else is kept so the validator can name it.
			b.WriteRune(r)
		}
	}
	return b.String()
}

// ParseUDIN checks a UDIN against the documented syntax and returns what it
// could read along with every finding.
//
// Findings of severity breach mean the string cannot be a UDIN. The one
// finding below that is only a caution is the split of the last ten characters:
// ICAI documents the syntax as six letters followed by four digits, but also
// describes that segment simply as "alpha-numeric generated randomly by the
// system". A number that is alphanumeric but splits differently is therefore
// flagged for a second look rather than rejected, because the register's job is
// to record what the portal issued, not to argue with it.
func ParseUDIN(raw string) (UDIN, []Finding) {
	value := NormaliseUDIN(raw)
	if value == "" {
		return UDIN{}, []Finding{{
			Code:     "udin-empty",
			Severity: SeverityBreach,
			Message:  "No UDIN was given.",
		}}
	}

	var findings []Finding
	if bad := invalidCharacters(value); bad != "" {
		findings = append(findings, Finding{
			Code:     "udin-characters",
			Severity: SeverityBreach,
			Message:  fmt.Sprintf("A UDIN is made only of digits and letters; this one contains %s.", bad),
		})
		return UDIN{Value: value}, findings
	}

	if len(value) != UDINLength {
		findings = append(findings, Finding{
			Code:     "udin-length",
			Severity: SeverityBreach,
			Message: fmt.Sprintf("A UDIN is %d characters; this one is %d (%s). Separators and spaces have already been ignored.",
				UDINLength, len(value), value),
		})
		return UDIN{Value: value}, findings
	}

	parsed := UDIN{
		Value:            value,
		YearDigits:       value[:udinYearDigits],
		MembershipNumber: value[udinYearDigits : udinYearDigits+udinMembershipDigits],
		RandomPart:       value[udinYearDigits+udinMembershipDigits:],
	}

	if !allDigits(parsed.YearDigits) {
		findings = append(findings, Finding{
			Code:     "udin-year-digits",
			Severity: SeverityBreach,
			Message:  fmt.Sprintf("The first two characters of a UDIN are the last two digits of the year it was generated in; %q is not a number.", parsed.YearDigits),
		})
	}
	if !allDigits(parsed.MembershipNumber) {
		findings = append(findings, Finding{
			Code:     "udin-membership-digits",
			Severity: SeverityBreach,
			Message:  fmt.Sprintf("Characters 3 to 8 of a UDIN are the member's six-digit ICAI membership number; %q is not a number.", parsed.MembershipNumber),
		})
	}
	if !matchesDocumentedRandomPart(parsed.RandomPart) {
		findings = append(findings, Finding{
			Code:     "udin-random-part-shape",
			Severity: SeverityWarn,
			Message: fmt.Sprintf("ICAI documents the last ten characters as six letters then four digits (AAAAAANNNN); %q does not follow that pattern. Check the number against the report before relying on it.",
				parsed.RandomPart),
		})
	}

	return parsed, findings
}

// ValidUDIN reports whether a string is a well-formed UDIN — well-formed only:
// see NoCheckDigitNote.
func ValidUDIN(raw string) bool {
	_, findings := ParseUDIN(raw)
	for _, f := range findings {
		if f.Severity == SeverityBreach {
			return false
		}
	}
	return true
}

// GenerationYears returns the two-digit years in which a UDIN for a document
// signed on signedOn could legitimately have been generated: the signing year,
// and the following year when the window runs across 31 December.
func GenerationYears(signedOn Date, windowDays int) []string {
	if signedOn.IsZero() {
		return nil
	}
	deadline := GenerationDeadline(signedOn, windowDays)
	years := []string{twoDigitYear(signedOn.Year())}
	if end := twoDigitYear(deadline.Year()); end != years[0] {
		years = append(years, end)
	}
	return years
}

// CrossCheck compares a parsed UDIN with what the register knows about the
// document: who signed it and when. These are the checks that catch a UDIN
// pasted into the wrong row, which a shape check alone cannot see.
//
// partnerMRN may be empty, in which case the membership number is not checked.
func CrossCheck(parsed UDIN, signedOn Date, partnerMRN string, windowDays int) []Finding {
	var findings []Finding
	if parsed.Value == "" {
		return nil
	}

	if parsed.MembershipNumber != "" && allDigits(parsed.MembershipNumber) && partnerMRN != "" {
		if normalised, err := NormaliseMRN(partnerMRN); err == nil && normalised != parsed.MembershipNumber {
			findings = append(findings, Finding{
				Code:     "udin-mrn-mismatch",
				Severity: SeverityBreach,
				Message: fmt.Sprintf("This UDIN was generated under membership number %s, but the engagement partner on record for this document is membership number %s. Either the UDIN is on the wrong row or the wrong partner is recorded as signing.",
					parsed.MembershipNumber, normalised),
			})
		}
	}

	if allDigits(parsed.YearDigits) && !signedOn.IsZero() {
		expected := GenerationYears(signedOn, windowDays)
		if len(expected) > 0 && !contains(expected, parsed.YearDigits) {
			findings = append(findings, Finding{
				Code:     "udin-year-mismatch",
				Severity: SeverityWarn,
				Message: fmt.Sprintf("This UDIN was generated in 20%s, but a document signed on %s had to have its UDIN generated in %s. Check the UDIN and the signing date against the report.",
					parsed.YearDigits, signedOn.Pretty(), joinYears(expected)),
			})
		}
	}

	return findings
}

// NormaliseMRN turns a membership number as typed into the six digits a UDIN
// carries. Members write theirs as "304576", "0304576", "304 576" or
// "M.No. 304576", and a register that compared those as strings would report a
// mismatch on every row.
func NormaliseMRN(raw string) (string, error) {
	var digits strings.Builder
	for _, r := range raw {
		if r >= '0' && r <= '9' {
			digits.WriteRune(r)
		}
	}
	s := strings.TrimLeft(digits.String(), "0")
	if s == "" {
		return "", fmt.Errorf("%q contains no membership number", raw)
	}
	if len(s) > udinMembershipDigits {
		return "", fmt.Errorf("an ICAI membership number is at most %d digits; %q has %d", udinMembershipDigits, raw, len(s))
	}
	return fmt.Sprintf("%0*s", udinMembershipDigits, s), nil
}

func invalidCharacters(value string) string {
	seen := map[rune]bool{}
	var bad []string
	for _, r := range value {
		if r >= '0' && r <= '9' || r >= 'A' && r <= 'Z' {
			continue
		}
		if seen[r] {
			continue
		}
		seen[r] = true
		bad = append(bad, fmt.Sprintf("%q", r))
	}
	return strings.Join(bad, ", ")
}

func allDigits(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

// matchesDocumentedRandomPart checks the AAAAAANNNN pattern from the FAQ:
// six letters then four digits.
func matchesDocumentedRandomPart(s string) bool {
	if len(s) != udinRandomChars {
		return false
	}
	for i, r := range s {
		if i < 6 {
			if r < 'A' || r > 'Z' {
				return false
			}
			continue
		}
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

func twoDigitYear(year int) string {
	return fmt.Sprintf("%02d", year%100)
}

func contains(list []string, want string) bool {
	for _, item := range list {
		if item == want {
			return true
		}
	}
	return false
}

func joinYears(years []string) string {
	full := make([]string, 0, len(years))
	for _, y := range years {
		full = append(full, "20"+y)
	}
	return strings.Join(full, " or ")
}
