package udin

import (
	"fmt"
	"strconv"
	"strings"
)

// FinYear is an Indian financial year: 1 April to 31 March. It is stored as the
// calendar year it starts in, so FinYear{2025} is the year printed "2025-26".
//
// The register needs this as a first-class thing rather than a label, because a
// document signed on 31 March belongs to a different financial year from one
// signed the next morning, and a UDIN query from a bank or a regulator always
// arrives as "give me everything you signed for this client for FY 2024-25".
type FinYear struct {
	start int
}

// NewFinYear builds the financial year beginning 1 April of startYear.
func NewFinYear(startYear int) (FinYear, error) {
	if startYear < 1900 || startYear > 2199 {
		return FinYear{}, fmt.Errorf("%d is outside the range of financial years this register handles", startYear)
	}
	return FinYear{start: startYear}, nil
}

// FinYearOf returns the financial year a date falls in. April to December
// belong to the year that is starting; January to March belong to the year
// before. 29 February 2024 is in 2023-24, not 2024-25.
func FinYearOf(d Date) FinYear {
	if d.IsZero() {
		return FinYear{}
	}
	if d.Month() >= 4 {
		return FinYear{start: d.Year()}
	}
	return FinYear{start: d.Year() - 1}
}

// ParseFinYear reads the forms a practitioner types: "2025-26", "2025-2026",
// "2025/26", "FY 2025-26" and a bare "2025". A two-digit end that does not
// follow the start by one year is rejected, so "2025-27" is caught rather than
// quietly stored.
func ParseFinYear(s string) (FinYear, error) {
	cleaned := strings.ToUpper(strings.TrimSpace(s))
	cleaned = strings.TrimPrefix(cleaned, "F.Y.")
	cleaned = strings.TrimPrefix(cleaned, "FY")
	cleaned = strings.TrimSpace(strings.ReplaceAll(cleaned, "/", "-"))
	if cleaned == "" {
		return FinYear{}, fmt.Errorf("no financial year given")
	}

	start, rest, split := strings.Cut(cleaned, "-")
	startYear, err := strconv.Atoi(strings.TrimSpace(start))
	if err != nil {
		return FinYear{}, fmt.Errorf("%q is not a financial year (try 2025-26)", s)
	}
	if !split {
		return NewFinYear(startYear)
	}

	rest = strings.TrimSpace(rest)
	endYear, err := strconv.Atoi(rest)
	if err != nil {
		return FinYear{}, fmt.Errorf("%q is not a financial year (try 2025-26)", s)
	}
	switch len(rest) {
	case 2:
		// "2099-00" has to roll into the next century.
		if endYear != (startYear+1)%100 {
			return FinYear{}, fmt.Errorf("a financial year runs over two consecutive years, so %q cannot be right", s)
		}
	case 4:
		if endYear != startYear+1 {
			return FinYear{}, fmt.Errorf("a financial year runs over two consecutive years, so %q cannot be right", s)
		}
	default:
		return FinYear{}, fmt.Errorf("%q is not a financial year (try 2025-26)", s)
	}
	return NewFinYear(startYear)
}

// IsZero reports a financial year that was never recorded.
func (f FinYear) IsZero() bool { return f.start == 0 }

// StartYear is the calendar year the financial year begins in.
func (f FinYear) StartYear() int { return f.start }

// Label is the printed form, "2025-26".
func (f FinYear) Label() string {
	if f.IsZero() {
		return ""
	}
	return fmt.Sprintf("%04d-%02d", f.start, (f.start+1)%100)
}

// Start is 1 April.
func (f FinYear) Start() Date {
	if f.IsZero() {
		return Date{}
	}
	return MustDate(f.start, 4, 1)
}

// End is 31 March of the following calendar year.
func (f FinYear) End() Date {
	if f.IsZero() {
		return Date{}
	}
	return MustDate(f.start+1, 3, 31)
}

// Contains reports whether a date falls inside the financial year.
func (f FinYear) Contains(d Date) bool {
	if f.IsZero() || d.IsZero() {
		return false
	}
	return !d.Before(f.Start()) && !d.After(f.End())
}

// AssessmentYear is the income-tax assessment year for this financial year:
// FY 2025-26 is assessed in AY 2026-27. Tax audit forms and several UDIN
// categories are keyed by assessment year rather than financial year, and
// transposing the two is one of the easier mistakes to make on the portal.
func (f FinYear) AssessmentYear() string {
	if f.IsZero() {
		return ""
	}
	return fmt.Sprintf("%04d-%02d", f.start+1, (f.start+2)%100)
}

// MarshalJSON writes the label, so the data file reads as "2025-26".
func (f FinYear) MarshalJSON() ([]byte, error) {
	if f.IsZero() {
		return []byte("null"), nil
	}
	return []byte(`"` + f.Label() + `"`), nil
}

// UnmarshalJSON accepts any form ParseFinYear accepts, plus "" and null.
func (f *FinYear) UnmarshalJSON(data []byte) error {
	raw := strings.TrimSpace(string(data))
	if raw == "null" {
		*f = FinYear{}
		return nil
	}
	unquoted, err := strconv.Unquote(raw)
	if err != nil {
		return fmt.Errorf("a financial year must be a string such as \"2025-26\"")
	}
	if strings.TrimSpace(unquoted) == "" {
		*f = FinYear{}
		return nil
	}
	parsed, err := ParseFinYear(unquoted)
	if err != nil {
		return err
	}
	*f = parsed
	return nil
}
