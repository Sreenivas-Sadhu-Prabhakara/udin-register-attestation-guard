package udin

import (
	"encoding/json"
	"testing"
)

func TestFinYearOfCrossesOn1April(t *testing.T) {
	cases := []struct {
		name string
		date Date
		want string
	}{
		{name: "the last day of a financial year", date: MustDate(2026, 3, 31), want: "2025-26"},
		{name: "the first day of a financial year", date: MustDate(2026, 4, 1), want: "2026-27"},
		{name: "mid-year", date: MustDate(2025, 11, 24), want: "2025-26"},
		{name: "January belongs to the year before", date: MustDate(2026, 1, 1), want: "2025-26"},
		{name: "a leap day", date: MustDate(2024, 2, 29), want: "2023-24"},
		{name: "31 December", date: MustDate(2025, 12, 31), want: "2025-26"},
		{name: "1 April is the pivot, not 1 January", date: MustDate(2026, 3, 1), want: "2025-26"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := FinYearOf(c.date).Label(); got != c.want {
				t.Fatalf("FinYearOf(%s) = %q, want %q", c.date, got, c.want)
			}
		})
	}
	if !FinYearOf(Date{}).IsZero() {
		t.Fatal("an unrecorded date produced a financial year")
	}
}

func TestParseFinYear(t *testing.T) {
	cases := []struct {
		in      string
		want    string
		wantErr bool
	}{
		{in: "2025-26", want: "2025-26"},
		{in: "2025-2026", want: "2025-26"},
		{in: "2025/26", want: "2025-26"},
		{in: "FY2025-26", want: "2025-26"},
		{in: "FY 2025-26", want: "2025-26"},
		{in: "F.Y. 2025-26", want: "2025-26"},
		{in: "  2025-26  ", want: "2025-26"},
		{in: "2025", want: "2025-26"},
		{in: "2099-00", want: "2099-00"},
		{in: "2025-27", wantErr: true},
		{in: "2025-2027", wantErr: true},
		{in: "2025-2", wantErr: true},
		{in: "last year", wantErr: true},
		{in: "", wantErr: true},
	}
	for _, c := range cases {
		t.Run(c.in, func(t *testing.T) {
			got, err := ParseFinYear(c.in)
			if c.wantErr {
				if err == nil {
					t.Fatalf("ParseFinYear(%q) = %q, want an error", c.in, got.Label())
				}
				return
			}
			if err != nil {
				t.Fatalf("ParseFinYear(%q): %v", c.in, err)
			}
			if got.Label() != c.want {
				t.Fatalf("ParseFinYear(%q) = %q, want %q", c.in, got.Label(), c.want)
			}
		})
	}
}

func TestFinYearSpansAprilToMarch(t *testing.T) {
	cases := []struct {
		label                string
		start, end           string
		assessmentYear       string
		inside, outsideEarly Date
	}{
		{
			label: "2025-26", start: "2025-04-01", end: "2026-03-31", assessmentYear: "2026-27",
			inside: MustDate(2026, 3, 31), outsideEarly: MustDate(2025, 3, 31),
		},
		{
			label: "2023-24", start: "2023-04-01", end: "2024-03-31", assessmentYear: "2024-25",
			inside: MustDate(2024, 2, 29), outsideEarly: MustDate(2023, 3, 31),
		},
	}
	for _, c := range cases {
		t.Run(c.label, func(t *testing.T) {
			fy, err := ParseFinYear(c.label)
			if err != nil {
				t.Fatalf("ParseFinYear(%q): %v", c.label, err)
			}
			if got := fy.Start().String(); got != c.start {
				t.Fatalf("Start() = %s, want %s", got, c.start)
			}
			if got := fy.End().String(); got != c.end {
				t.Fatalf("End() = %s, want %s", got, c.end)
			}
			if got := fy.AssessmentYear(); got != c.assessmentYear {
				t.Fatalf("AssessmentYear() = %q, want %q", got, c.assessmentYear)
			}
			if !fy.Contains(c.inside) {
				t.Fatalf("FY %s did not contain %s", c.label, c.inside)
			}
			if !fy.Contains(fy.Start()) || !fy.Contains(fy.End()) {
				t.Fatalf("FY %s did not contain its own first or last day", c.label)
			}
			if fy.Contains(c.outsideEarly) {
				t.Fatalf("FY %s contained %s", c.label, c.outsideEarly)
			}
			if fy.Contains(fy.End().AddDays(1)) {
				t.Fatalf("FY %s contained the day after it ended", c.label)
			}
			if fy.Contains(Date{}) {
				t.Fatalf("FY %s contained an unrecorded date", c.label)
			}
		})
	}

	var unset FinYear
	if !unset.IsZero() || unset.Label() != "" || unset.AssessmentYear() != "" ||
		!unset.Start().IsZero() || !unset.End().IsZero() || unset.Contains(MustDate(2026, 1, 1)) {
		t.Fatal("the zero financial year did not behave as unrecorded")
	}
	if _, err := NewFinYear(1800); err == nil {
		t.Fatal("a financial year starting in 1800 was accepted")
	}
	if got := (FinYear{start: 2099}).Label(); got != "2099-00" {
		t.Fatalf("the century rollover printed as %q", got)
	}
}

func TestFinYearJSON(t *testing.T) {
	type holder struct {
		FY FinYear `json:"fy"`
	}

	encoded, err := json.Marshal(holder{FY: FinYear{start: 2025}})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if string(encoded) != `{"fy":"2025-26"}` {
		t.Fatalf("marshal = %s", encoded)
	}
	encodedZero, err := json.Marshal(holder{})
	if err != nil {
		t.Fatalf("marshal zero: %v", err)
	}
	if string(encodedZero) != `{"fy":null}` {
		t.Fatalf("marshal zero = %s", encodedZero)
	}

	cases := []struct {
		in      string
		want    string
		wantErr bool
	}{
		{in: `{"fy":"2025-26"}`, want: "2025-26"},
		{in: `{"fy":"2025-2026"}`, want: "2025-26"},
		{in: `{"fy":null}`, want: ""},
		{in: `{"fy":""}`, want: ""},
		{in: `{"fy":"2025-27"}`, wantErr: true},
		{in: `{"fy":2025}`, wantErr: true},
	}
	for _, c := range cases {
		t.Run(c.in, func(t *testing.T) {
			var got holder
			err := json.Unmarshal([]byte(c.in), &got)
			if c.wantErr {
				if err == nil {
					t.Fatalf("unmarshal %s = %q, want an error", c.in, got.FY.Label())
				}
				return
			}
			if err != nil {
				t.Fatalf("unmarshal %s: %v", c.in, err)
			}
			if got.FY.Label() != c.want {
				t.Fatalf("unmarshal %s = %q, want %q", c.in, got.FY.Label(), c.want)
			}
		})
	}
}
