package udin

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"
)

// Date is a calendar date with no clock and no zone. The UDIN rules are counted
// in whole days ("within 60 days of signing"), so a date is the right unit: a
// timestamp would drag in a zone and let a deadline move by a day depending on
// where the machine thinks it is.
//
// The zero Date means "not recorded" and reports IsZero.
type Date struct {
	year  int
	month int
	day   int
}

// NewDate builds a date and rejects anything that is not a real day, so an
// invalid date can never reach the deadline arithmetic. 31 February fails here
// rather than silently becoming 2 or 3 March.
func NewDate(year, month, day int) (Date, error) {
	if year < 1900 || year > 2200 {
		return Date{}, fmt.Errorf("year %d is outside the range this register handles (1900-2200)", year)
	}
	if month < 1 || month > 12 {
		return Date{}, fmt.Errorf("there is no month %d", month)
	}
	if day < 1 || day > daysInMonth(year, month) {
		return Date{}, fmt.Errorf("%s %d has no day %d", time.Month(month), year, day)
	}
	return Date{year: year, month: month, day: day}, nil
}

// MustDate is for tests and for the built-in reference dates in this package,
// which are literals and cannot fail.
func MustDate(year, month, day int) Date {
	d, err := NewDate(year, month, day)
	if err != nil {
		panic(err)
	}
	return d
}

// Today reads the machine's own idea of the date. Nothing in this tool asks a
// server what day it is.
func Today() Date {
	now := time.Now()
	return Date{year: now.Year(), month: int(now.Month()), day: now.Day()}
}

// ParseDate reads the one unambiguous form, YYYY-MM-DD, used by the JSON API
// and by every date input in the interface.
func ParseDate(s string) (Date, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return Date{}, fmt.Errorf("no date given")
	}
	parts := strings.Split(s, "-")
	if len(parts) != 3 || len(parts[0]) != 4 || len(parts[1]) != 2 || len(parts[2]) != 2 {
		return Date{}, fmt.Errorf("%q is not a date in YYYY-MM-DD form", s)
	}
	nums := make([]int, 3)
	for i, p := range parts {
		n, err := strconv.Atoi(p)
		if err != nil {
			return Date{}, fmt.Errorf("%q is not a date in YYYY-MM-DD form", s)
		}
		nums[i] = n
	}
	return NewDate(nums[0], nums[1], nums[2])
}

// ParseDateFlexible also accepts the forms people actually type and paste in
// India: 05-04-2026, 05/04/2026, 5.4.2026 and 2026/04/05. Day-first is assumed
// for the dotted and slashed forms, because that is what every Indian audit
// report, challan and portal screen prints. Where the day is ambiguous
// (04/05/2026) day-first is used, which is why the paste importer reports the
// date it understood back to the user.
func ParseDateFlexible(s string) (Date, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return Date{}, fmt.Errorf("no date given")
	}
	if d, err := ParseDate(s); err == nil {
		return d, nil
	}
	sep := ""
	for _, candidate := range []string{"-", "/", "."} {
		if strings.Count(s, candidate) == 2 {
			sep = candidate
			break
		}
	}
	if sep == "" {
		return Date{}, fmt.Errorf("%q is not a date this register can read (try 2026-04-05 or 05-04-2026)", s)
	}
	parts := strings.Split(s, sep)
	nums := make([]int, 3)
	for i, p := range parts {
		n, err := strconv.Atoi(strings.TrimSpace(p))
		if err != nil {
			return Date{}, fmt.Errorf("%q is not a date this register can read (try 2026-04-05 or 05-04-2026)", s)
		}
		nums[i] = n
	}
	// Four-digit field first means year-first; otherwise day-first.
	if len(strings.TrimSpace(parts[0])) == 4 {
		return NewDate(nums[0], nums[1], nums[2])
	}
	year := nums[2]
	if year < 100 {
		// A two-digit year on an attestation register is this century: the
		// mandate itself only began in 2019.
		year += 2000
	}
	return NewDate(year, nums[1], nums[0])
}

// Year, Month and Day expose the parts for the few places that need them (the
// financial year rule and the UDIN year prefix check).
func (d Date) Year() int  { return d.year }
func (d Date) Month() int { return d.month }
func (d Date) Day() int   { return d.day }

// IsZero reports a date that was never recorded.
func (d Date) IsZero() bool { return d == Date{} }

// String is the stored and transported form.
func (d Date) String() string {
	if d.IsZero() {
		return ""
	}
	return fmt.Sprintf("%04d-%02d-%02d", d.year, d.month, d.day)
}

// Pretty is the form used in CSV exports and printed reminders.
func (d Date) Pretty() string {
	if d.IsZero() {
		return "—"
	}
	return fmt.Sprintf("%02d %s %04d", d.day, time.Month(d.month).String()[:3], d.year)
}

// AddDays moves by whole calendar days. It goes through time.Time so month
// lengths and leap days are the standard library's problem, not ours: 1 January
// 2024 plus 59 days is 29 February 2024, and the same sum in 2023 is 1 March.
func (d Date) AddDays(n int) Date {
	t := d.time().AddDate(0, 0, n)
	return Date{year: t.Year(), month: int(t.Month()), day: t.Day()}
}

// Before, After and Equal order two dates.
func (d Date) Before(other Date) bool { return d.time().Before(other.time()) }
func (d Date) After(other Date) bool  { return d.time().After(other.time()) }
func (d Date) Equal(other Date) bool  { return d == other }

// DaysBetween counts the days from a to b, so it is negative when b is earlier.
// Both ends are midnight UTC, so no daylight-saving change can make a day
// 23 or 25 hours long and round the answer off by one.
func DaysBetween(a, b Date) int {
	return int(b.time().Sub(a.time()).Hours() / 24)
}

// InclusiveDays counts a and b and every day in between, which is how ICAI
// counts the UDIN window: the FAQ states that both the date of signing and the
// date of generation are included in the time allowed. A document signed and
// given a UDIN on the same day has used one day of its sixty, not none.
func InclusiveDays(a, b Date) int {
	return DaysBetween(a, b) + 1
}

func (d Date) time() time.Time {
	return time.Date(d.year, time.Month(d.month), d.day, 0, 0, 0, 0, time.UTC)
}

func daysInMonth(year, month int) int {
	return time.Date(year, time.Month(month)+1, 0, 0, 0, 0, 0, time.UTC).Day()
}

// MarshalJSON writes a date as "2026-04-05", and an unrecorded date as null.
func (d Date) MarshalJSON() ([]byte, error) {
	if d.IsZero() {
		return []byte("null"), nil
	}
	return json.Marshal(d.String())
}

// UnmarshalJSON accepts "2026-04-05", "" and null.
func (d *Date) UnmarshalJSON(data []byte) error {
	if string(data) == "null" {
		*d = Date{}
		return nil
	}
	var s string
	if err := json.Unmarshal(data, &s); err != nil {
		return fmt.Errorf("a date must be a string in YYYY-MM-DD form: %w", err)
	}
	if strings.TrimSpace(s) == "" {
		*d = Date{}
		return nil
	}
	parsed, err := ParseDate(s)
	if err != nil {
		return err
	}
	*d = parsed
	return nil
}

// Stamp is a date with an optional clock time, to the minute. The 48-hour
// revocation window runs from the moment the UDIN was generated, so for that
// one field a date alone is not enough — but a practitioner back-filling a
// register from last quarter often has only the date. Stamp carries both cases
// and says which it is, so the tool can be honest about how precisely it knows
// when the revocation window shuts.
type Stamp struct {
	date    Date
	minutes int
	hasTime bool
}

// NewStamp builds a stamp with a clock time.
func NewStamp(d Date, hour, minute int) (Stamp, error) {
	if d.IsZero() {
		return Stamp{}, fmt.Errorf("a time needs a date as well")
	}
	if hour < 0 || hour > 23 || minute < 0 || minute > 59 {
		return Stamp{}, fmt.Errorf("%02d:%02d is not a time of day", hour, minute)
	}
	return Stamp{date: d, minutes: hour*60 + minute, hasTime: true}, nil
}

// DateStamp builds a stamp that records the day only.
func DateStamp(d Date) Stamp {
	return Stamp{date: d}
}

// ParseStamp reads "2026-04-05", "2026-04-05T14:30", "2026-04-05 14:30" and
// "2026-04-05T14:30:00" (seconds are dropped). It takes no zone offset: the
// portal shows the member a local wall-clock time, and that is what gets
// recorded here.
func ParseStamp(s string) (Stamp, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return Stamp{}, fmt.Errorf("no date given")
	}
	datePart, timePart, split := strings.Cut(s, "T")
	if !split {
		datePart, timePart, split = strings.Cut(s, " ")
	}
	d, err := ParseDate(datePart)
	if err != nil {
		return Stamp{}, err
	}
	if !split || strings.TrimSpace(timePart) == "" {
		return DateStamp(d), nil
	}
	fields := strings.Split(strings.TrimSpace(timePart), ":")
	if len(fields) < 2 || len(fields) > 3 {
		return Stamp{}, fmt.Errorf("%q is not a time of day in HH:MM form", timePart)
	}
	hour, errHour := strconv.Atoi(fields[0])
	minute, errMinute := strconv.Atoi(fields[1])
	if errHour != nil || errMinute != nil {
		return Stamp{}, fmt.Errorf("%q is not a time of day in HH:MM form", timePart)
	}
	return NewStamp(d, hour, minute)
}

// Date is the day part of the stamp.
func (s Stamp) Date() Date { return s.date }

// Exact reports whether a clock time was recorded. When it was not, the
// revocation window can only be stated as "no later than", because the stamp is
// read as midnight — the earliest moment that day.
func (s Stamp) Exact() bool { return s.hasTime }

// IsZero reports a stamp that was never recorded.
func (s Stamp) IsZero() bool { return s.date.IsZero() }

// Clock returns the hour and minute recorded, or 0:00 when only a date is known.
func (s Stamp) Clock() (int, int) { return s.minutes / 60, s.minutes % 60 }

// AddHours moves the stamp forward. The result always carries a clock time,
// because adding 48 hours to a day produces a moment, not a day.
func (s Stamp) AddHours(n int) Stamp {
	t := s.time().Add(time.Duration(n) * time.Hour)
	return Stamp{
		date:    Date{year: t.Year(), month: int(t.Month()), day: t.Day()},
		minutes: t.Hour()*60 + t.Minute(),
		hasTime: true,
	}
}

// Before orders two stamps.
func (s Stamp) Before(other Stamp) bool { return s.time().Before(other.time()) }

// String is the stored and transported form.
func (s Stamp) String() string {
	if s.IsZero() {
		return ""
	}
	if !s.hasTime {
		return s.date.String()
	}
	return fmt.Sprintf("%sT%02d:%02d", s.date.String(), s.minutes/60, s.minutes%60)
}

// Pretty is the form used in exports and reminders.
func (s Stamp) Pretty() string {
	if s.IsZero() {
		return "—"
	}
	if !s.hasTime {
		return s.date.Pretty()
	}
	return fmt.Sprintf("%s %02d:%02d", s.date.Pretty(), s.minutes/60, s.minutes%60)
}

func (s Stamp) time() time.Time {
	return time.Date(s.date.year, time.Month(s.date.month), s.date.day, s.minutes/60, s.minutes%60, 0, 0, time.UTC)
}

// MarshalJSON writes a stamp as "2026-04-05T14:30" or "2026-04-05".
func (s Stamp) MarshalJSON() ([]byte, error) {
	if s.IsZero() {
		return []byte("null"), nil
	}
	return json.Marshal(s.String())
}

// UnmarshalJSON accepts the forms ParseStamp accepts, plus "" and null.
func (s *Stamp) UnmarshalJSON(data []byte) error {
	if string(data) == "null" {
		*s = Stamp{}
		return nil
	}
	var raw string
	if err := json.Unmarshal(data, &raw); err != nil {
		return fmt.Errorf("a date and time must be a string such as \"2026-04-05T14:30\": %w", err)
	}
	if strings.TrimSpace(raw) == "" {
		*s = Stamp{}
		return nil
	}
	parsed, err := ParseStamp(raw)
	if err != nil {
		return err
	}
	*s = parsed
	return nil
}
