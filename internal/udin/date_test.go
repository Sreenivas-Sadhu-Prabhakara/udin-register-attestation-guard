package udin

import (
	"encoding/json"
	"testing"
)

func TestNewDateRejectsDaysThatDoNotExist(t *testing.T) {
	cases := []struct {
		name             string
		year, month, day int
		wantErr          bool
	}{
		{name: "ordinary day", year: 2026, month: 11, day: 24},
		{name: "29 February in a leap year", year: 2024, month: 2, day: 29},
		{name: "29 February in a common year", year: 2023, month: 2, day: 29, wantErr: true},
		{name: "29 February in 1900, not a leap year", year: 1900, month: 2, day: 29, wantErr: true},
		{name: "29 February in 2000, a leap year", year: 2000, month: 2, day: 29},
		{name: "31 April", year: 2026, month: 4, day: 31, wantErr: true},
		{name: "30 April", year: 2026, month: 4, day: 30},
		{name: "month zero", year: 2026, month: 0, day: 1, wantErr: true},
		{name: "month thirteen", year: 2026, month: 13, day: 1, wantErr: true},
		{name: "day zero", year: 2026, month: 1, day: 0, wantErr: true},
		{name: "year out of range", year: 1800, month: 1, day: 1, wantErr: true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := NewDate(c.year, c.month, c.day)
			if c.wantErr {
				if err == nil {
					t.Fatalf("NewDate(%d, %d, %d) = %s, want an error", c.year, c.month, c.day, got)
				}
				return
			}
			if err != nil {
				t.Fatalf("NewDate(%d, %d, %d): %v", c.year, c.month, c.day, err)
			}
			if got.Year() != c.year || got.Month() != c.month || got.Day() != c.day {
				t.Fatalf("NewDate(%d, %d, %d) = %s", c.year, c.month, c.day, got)
			}
		})
	}
}

func TestParseDate(t *testing.T) {
	cases := []struct {
		in      string
		want    string
		wantErr bool
	}{
		{in: "2026-04-05", want: "2026-04-05"},
		{in: "  2026-04-05 ", want: "2026-04-05"},
		{in: "2024-02-29", want: "2024-02-29"},
		{in: "2023-02-29", wantErr: true},
		{in: "2026-4-5", wantErr: true},
		{in: "05-04-2026", wantErr: true},
		{in: "", wantErr: true},
		{in: "yesterday", wantErr: true},
		{in: "2026-04-05T10:00", wantErr: true},
	}
	for _, c := range cases {
		t.Run(c.in, func(t *testing.T) {
			got, err := ParseDate(c.in)
			if c.wantErr {
				if err == nil {
					t.Fatalf("ParseDate(%q) = %s, want an error", c.in, got)
				}
				return
			}
			if err != nil {
				t.Fatalf("ParseDate(%q): %v", c.in, err)
			}
			if got.String() != c.want {
				t.Fatalf("ParseDate(%q) = %s, want %s", c.in, got, c.want)
			}
		})
	}
}

func TestParseDateFlexibleReadsWhatPeopleType(t *testing.T) {
	cases := []struct {
		in      string
		want    string
		wantErr bool
	}{
		{in: "2026-04-05", want: "2026-04-05"},
		{in: "05-04-2026", want: "2026-04-05"},
		{in: "05/04/2026", want: "2026-04-05"},
		{in: "5.4.2026", want: "2026-04-05"},
		{in: "2026/04/05", want: "2026-04-05"},
		{in: "05-04-26", want: "2026-04-05"},
		{in: "29-02-2024", want: "2024-02-29"},
		{in: "29-02-2023", wantErr: true},
		{in: "31-04-2026", wantErr: true},
		{in: "05 April 2026", wantErr: true},
		{in: "2026-04", wantErr: true},
		{in: "", wantErr: true},
	}
	for _, c := range cases {
		t.Run(c.in, func(t *testing.T) {
			got, err := ParseDateFlexible(c.in)
			if c.wantErr {
				if err == nil {
					t.Fatalf("ParseDateFlexible(%q) = %s, want an error", c.in, got)
				}
				return
			}
			if err != nil {
				t.Fatalf("ParseDateFlexible(%q): %v", c.in, err)
			}
			if got.String() != c.want {
				t.Fatalf("ParseDateFlexible(%q) = %s, want %s", c.in, got, c.want)
			}
		})
	}
}

func TestAddDaysIsCalendarCorrect(t *testing.T) {
	cases := []struct {
		name string
		from Date
		add  int
		want string
	}{
		{name: "into a leap day", from: MustDate(2024, 2, 28), add: 1, want: "2024-02-29"},
		{name: "over a leap day", from: MustDate(2024, 2, 28), add: 2, want: "2024-03-01"},
		{name: "same span in a common year", from: MustDate(2023, 2, 28), add: 1, want: "2023-03-01"},
		{name: "59 days from new year in a leap year", from: MustDate(2024, 1, 1), add: 59, want: "2024-02-29"},
		{name: "59 days from new year in a common year", from: MustDate(2023, 1, 1), add: 59, want: "2023-03-01"},
		{name: "across the year end", from: MustDate(2025, 12, 20), add: 20, want: "2026-01-09"},
		{name: "across the financial year end", from: MustDate(2026, 3, 31), add: 1, want: "2026-04-01"},
		{name: "backwards over a leap day", from: MustDate(2024, 3, 1), add: -2, want: "2024-02-28"},
		{name: "a 31-day month end", from: MustDate(2026, 1, 31), add: 1, want: "2026-02-01"},
		{name: "a 30-day month end", from: MustDate(2026, 4, 30), add: 1, want: "2026-05-01"},
		{name: "nothing added", from: MustDate(2026, 4, 30), add: 0, want: "2026-04-30"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := c.from.AddDays(c.add); got.String() != c.want {
				t.Fatalf("%s.AddDays(%d) = %s, want %s", c.from, c.add, got, c.want)
			}
		})
	}
}

func TestDaysBetweenAndInclusiveDays(t *testing.T) {
	cases := []struct {
		name          string
		a, b          Date
		want          int
		wantInclusive int
	}{
		{name: "same day", a: MustDate(2026, 4, 5), b: MustDate(2026, 4, 5), want: 0, wantInclusive: 1},
		{name: "next day", a: MustDate(2026, 4, 5), b: MustDate(2026, 4, 6), want: 1, wantInclusive: 2},
		{name: "over February in a leap year", a: MustDate(2024, 2, 1), b: MustDate(2024, 3, 1), want: 29, wantInclusive: 30},
		{name: "over February in a common year", a: MustDate(2023, 2, 1), b: MustDate(2023, 3, 1), want: 28, wantInclusive: 29},
		{name: "a leap year is 366 days", a: MustDate(2024, 1, 1), b: MustDate(2025, 1, 1), want: 366, wantInclusive: 367},
		{name: "a financial year is 365 days", a: MustDate(2025, 4, 1), b: MustDate(2026, 3, 31), want: 364, wantInclusive: 365},
		{name: "a leap financial year is 366 days", a: MustDate(2023, 4, 1), b: MustDate(2024, 3, 31), want: 365, wantInclusive: 366},
		{name: "backwards", a: MustDate(2026, 4, 6), b: MustDate(2026, 4, 5), want: -1, wantInclusive: 0},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := DaysBetween(c.a, c.b); got != c.want {
				t.Fatalf("DaysBetween(%s, %s) = %d, want %d", c.a, c.b, got, c.want)
			}
			if got := InclusiveDays(c.a, c.b); got != c.wantInclusive {
				t.Fatalf("InclusiveDays(%s, %s) = %d, want %d", c.a, c.b, got, c.wantInclusive)
			}
		})
	}
}

func TestDateOrderingAndPretty(t *testing.T) {
	early, late := MustDate(2026, 3, 31), MustDate(2026, 4, 1)
	if !early.Before(late) || early.After(late) || early.Equal(late) {
		t.Fatalf("%s and %s did not order correctly", early, late)
	}
	if !early.Equal(MustDate(2026, 3, 31)) {
		t.Fatal("two identical dates were not equal")
	}
	if got := late.Pretty(); got != "01 Apr 2026" {
		t.Fatalf("Pretty() = %q, want %q", got, "01 Apr 2026")
	}
	var unset Date
	if !unset.IsZero() || unset.String() != "" || unset.Pretty() != "—" {
		t.Fatalf("the zero date did not behave as unrecorded: %q / %q", unset.String(), unset.Pretty())
	}
}

func TestDateJSON(t *testing.T) {
	type holder struct {
		On Date `json:"on"`
	}

	encoded, err := json.Marshal(holder{On: MustDate(2026, 4, 5)})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if string(encoded) != `{"on":"2026-04-05"}` {
		t.Fatalf("marshal = %s", encoded)
	}

	encodedZero, err := json.Marshal(holder{})
	if err != nil {
		t.Fatalf("marshal zero: %v", err)
	}
	if string(encodedZero) != `{"on":null}` {
		t.Fatalf("marshal zero = %s", encodedZero)
	}

	cases := []struct {
		in      string
		want    string
		wantErr bool
	}{
		{in: `{"on":"2026-04-05"}`, want: "2026-04-05"},
		{in: `{"on":null}`, want: ""},
		{in: `{"on":""}`, want: ""},
		{in: `{"on":"2026-02-30"}`, wantErr: true},
		{in: `{"on":"05-04-2026"}`, wantErr: true},
		{in: `{"on":20260405}`, wantErr: true},
	}
	for _, c := range cases {
		t.Run(c.in, func(t *testing.T) {
			var got holder
			err := json.Unmarshal([]byte(c.in), &got)
			if c.wantErr {
				if err == nil {
					t.Fatalf("unmarshal %s = %s, want an error", c.in, got.On)
				}
				return
			}
			if err != nil {
				t.Fatalf("unmarshal %s: %v", c.in, err)
			}
			if got.On.String() != c.want {
				t.Fatalf("unmarshal %s = %q, want %q", c.in, got.On, c.want)
			}
		})
	}
}

func TestParseStamp(t *testing.T) {
	cases := []struct {
		in        string
		want      string
		wantExact bool
		wantErr   bool
	}{
		{in: "2026-04-05", want: "2026-04-05", wantExact: false},
		{in: "2026-04-05T14:30", want: "2026-04-05T14:30", wantExact: true},
		{in: "2026-04-05 14:30", want: "2026-04-05T14:30", wantExact: true},
		{in: "2026-04-05T14:30:59", want: "2026-04-05T14:30", wantExact: true},
		{in: "2026-04-05T00:00", want: "2026-04-05T00:00", wantExact: true},
		{in: "2026-04-05T23:59", want: "2026-04-05T23:59", wantExact: true},
		{in: "2026-04-05T24:00", wantErr: true},
		{in: "2026-04-05T14:60", wantErr: true},
		{in: "2026-04-05T14", wantErr: true},
		{in: "2026-02-30T10:00", wantErr: true},
		{in: "", wantErr: true},
	}
	for _, c := range cases {
		t.Run(c.in, func(t *testing.T) {
			got, err := ParseStamp(c.in)
			if c.wantErr {
				if err == nil {
					t.Fatalf("ParseStamp(%q) = %s, want an error", c.in, got)
				}
				return
			}
			if err != nil {
				t.Fatalf("ParseStamp(%q): %v", c.in, err)
			}
			if got.String() != c.want {
				t.Fatalf("ParseStamp(%q) = %s, want %s", c.in, got, c.want)
			}
			if got.Exact() != c.wantExact {
				t.Fatalf("ParseStamp(%q).Exact() = %v, want %v", c.in, got.Exact(), c.wantExact)
			}
		})
	}
}

func TestStampAddHoursCrossesDaysAndMonths(t *testing.T) {
	cases := []struct {
		name  string
		from  string
		hours int
		want  string
	}{
		{name: "within the day", from: "2026-04-05T10:00", hours: 8, want: "2026-04-05T18:00"},
		{name: "over midnight", from: "2026-04-05T18:00", hours: 8, want: "2026-04-06T02:00"},
		{name: "48 hours", from: "2026-04-05T14:30", hours: 48, want: "2026-04-07T14:30"},
		{name: "over a month end", from: "2026-03-31T23:30", hours: 48, want: "2026-04-02T23:30"},
		{name: "over a leap day", from: "2024-02-28T12:00", hours: 48, want: "2024-03-01T12:00"},
		{name: "over the same dates in a common year", from: "2023-02-28T12:00", hours: 48, want: "2023-03-02T12:00"},
		{name: "a date-only stamp counts from midnight", from: "2026-04-05", hours: 48, want: "2026-04-07T00:00"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			from, err := ParseStamp(c.from)
			if err != nil {
				t.Fatalf("ParseStamp(%q): %v", c.from, err)
			}
			if got := from.AddHours(c.hours); got.String() != c.want {
				t.Fatalf("%s.AddHours(%d) = %s, want %s", c.from, c.hours, got, c.want)
			}
		})
	}
}

func TestStampJSONAndHelpers(t *testing.T) {
	type holder struct {
		At Stamp `json:"at"`
	}

	stamp, err := NewStamp(MustDate(2026, 4, 5), 14, 30)
	if err != nil {
		t.Fatalf("NewStamp: %v", err)
	}
	encoded, err := json.Marshal(holder{At: stamp})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if string(encoded) != `{"at":"2026-04-05T14:30"}` {
		t.Fatalf("marshal = %s", encoded)
	}
	if hour, minute := stamp.Clock(); hour != 14 || minute != 30 {
		t.Fatalf("Clock() = %d:%d", hour, minute)
	}
	if got := stamp.Pretty(); got != "05 Apr 2026 14:30" {
		t.Fatalf("Pretty() = %q", got)
	}
	if got := DateStamp(MustDate(2026, 4, 5)).Pretty(); got != "05 Apr 2026" {
		t.Fatalf("date-only Pretty() = %q", got)
	}

	var decoded holder
	if err := json.Unmarshal([]byte(`{"at":"2026-04-05"}`), &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if decoded.At.Exact() || decoded.At.Date().String() != "2026-04-05" {
		t.Fatalf("unmarshal gave %s exact=%v", decoded.At, decoded.At.Exact())
	}
	if err := json.Unmarshal([]byte(`{"at":"2026-04-05T99:00"}`), &decoded); err == nil {
		t.Fatal("an impossible time was accepted")
	}

	if _, err := NewStamp(Date{}, 10, 0); err == nil {
		t.Fatal("a time with no date was accepted")
	}
	if _, err := NewStamp(MustDate(2026, 4, 5), 25, 0); err == nil {
		t.Fatal("hour 25 was accepted")
	}
	if !(Stamp{}).IsZero() {
		t.Fatal("the zero stamp did not report as unrecorded")
	}
}
