package udin

import (
	"strings"
	"testing"
)

// The reference values are the point of this tool, so they are pinned here with
// what they rest on. Changing one should mean changing the source note and the
// documentation in the same breath.
func TestReferenceWindowsMatchTheirSources(t *testing.T) {
	if DefaultGenerationWindowDays != 60 {
		t.Fatalf("the generation window is %d days; the ICAI announcement of 17 September 2021 sets it at 60", DefaultGenerationWindowDays)
	}
	if DefaultEFilingWindowDays != 15 {
		t.Fatalf("the e-filing update window is %d days; the CBDT press release sets it at 15", DefaultEFilingWindowDays)
	}
	if DefaultRevocationWindowHours != 48 {
		t.Fatalf("the revocation window is %d hours; the ICAI announcement of 23 June 2023 sets it at 48", DefaultRevocationWindowHours)
	}
	for _, source := range []string{GenerationWindowSource, EFilingWindowSource, RevocationWindowSource, CountingSource, MandateSource, FormatSource} {
		if len(source) < 40 {
			t.Fatalf("a source note is too short to say anything: %q", source)
		}
	}
	if !strings.Contains(GenerationWindowSource, "17 September 2021") {
		t.Fatalf("the generation window's source note does not name the announcement date: %q", GenerationWindowSource)
	}
	if !strings.Contains(RevocationWindowSource, "48 hours") {
		t.Fatalf("the revocation window's source note does not state the window: %q", RevocationWindowSource)
	}
}

func TestGenerationDeadlineCountsBothEnds(t *testing.T) {
	cases := []struct {
		name     string
		signedOn Date
		window   int
		want     string
	}{
		{
			name:     "60 days from new year in a leap year lands on the leap day",
			signedOn: MustDate(2024, 1, 1), window: 60, want: "2024-02-29",
		},
		{
			name:     "the same 60 days in a common year land a day later in March",
			signedOn: MustDate(2023, 1, 1), window: 60, want: "2023-03-01",
		},
		{
			name:     "signed on a leap day",
			signedOn: MustDate(2024, 2, 29), window: 60, want: "2024-04-28",
		},
		{
			name:     "signed on the last day of a financial year",
			signedOn: MustDate(2026, 3, 31), window: 60, want: "2026-05-29",
		},
		{
			name:     "signed on the first day of a financial year",
			signedOn: MustDate(2026, 4, 1), window: 60, want: "2026-05-30",
		},
		{
			name:     "the window runs across the calendar year end",
			signedOn: MustDate(2025, 12, 20), window: 60, want: "2026-02-17",
		},
		{
			name:     "the old 15-day window, for a register of older documents",
			signedOn: MustDate(2026, 1, 1), window: 15, want: "2026-01-15",
		},
		{
			name:     "a one-day window is the signing day itself",
			signedOn: MustDate(2026, 1, 1), window: 1, want: "2026-01-01",
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := GenerationDeadline(c.signedOn, c.window)
			if got.String() != c.want {
				t.Fatalf("GenerationDeadline(%s, %d) = %s, want %s", c.signedOn, c.window, got, c.want)
			}
			// The deadline is the window's last day, so counting the signing
			// day and the deadline must come to exactly the window.
			if used := DaysUsed(c.signedOn, got); used != c.window {
				t.Fatalf("the deadline %s is day %d of a %d-day window", got, used, c.window)
			}
		})
	}

	if !GenerationDeadline(Date{}, 60).IsZero() {
		t.Fatal("a deadline was produced for a document with no signing date")
	}
	if !GenerationDeadline(MustDate(2026, 1, 1), 0).IsZero() {
		t.Fatal("a deadline was produced for a window of zero days")
	}
}

func TestWithinWindowAtTheBoundary(t *testing.T) {
	signedOn := MustDate(2026, 1, 1) // deadline 1 March 2026 on a 60-day window
	cases := []struct {
		name        string
		generatedOn Date
		wantDays    int
		wantWithin  bool
	}{
		{name: "generated on the day of signing", generatedOn: MustDate(2026, 1, 1), wantDays: 1, wantWithin: true},
		{name: "the day after signing", generatedOn: MustDate(2026, 1, 2), wantDays: 2, wantWithin: true},
		{name: "a day inside the window", generatedOn: MustDate(2026, 2, 28), wantDays: 59, wantWithin: true},
		{name: "exactly on the last day of the window", generatedOn: MustDate(2026, 3, 1), wantDays: 60, wantWithin: true},
		{name: "one day past the window", generatedOn: MustDate(2026, 3, 2), wantDays: 61, wantWithin: false},
		{name: "well past the window", generatedOn: MustDate(2026, 6, 1), wantDays: 152, wantWithin: false},
		{name: "dated before the document was signed", generatedOn: MustDate(2025, 12, 31), wantDays: 0, wantWithin: false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := DaysUsed(signedOn, c.generatedOn); got != c.wantDays {
				t.Fatalf("DaysUsed(%s, %s) = %d, want %d", signedOn, c.generatedOn, got, c.wantDays)
			}
			if got := WithinWindow(signedOn, c.generatedOn, 60); got != c.wantWithin {
				t.Fatalf("WithinWindow(%s, %s, 60) = %v, want %v", signedOn, c.generatedOn, got, c.wantWithin)
			}
		})
	}

	// The same boundary on a leap-year window: the 60th day is 29 February.
	leapSigned := MustDate(2024, 1, 1)
	if !WithinWindow(leapSigned, MustDate(2024, 2, 29), 60) {
		t.Fatal("29 February 2024 is day 60 for a document signed on 1 January 2024 and should be within the window")
	}
	if WithinWindow(leapSigned, MustDate(2024, 3, 1), 60) {
		t.Fatal("1 March 2024 is day 61 for a document signed on 1 January 2024 and should be outside the window")
	}
	if DaysUsed(Date{}, MustDate(2024, 3, 1)) != 0 {
		t.Fatal("days were counted from a document with no signing date")
	}
}

func TestEFilingDeadlineIsTheShorterClock(t *testing.T) {
	cases := []struct {
		name        string
		submittedOn Date
		window      int
		want        string
	}{
		{name: "the usual case", submittedOn: MustDate(2026, 10, 31), window: 15, want: "2026-11-14"},
		{name: "across a month end", submittedOn: MustDate(2026, 9, 30), window: 15, want: "2026-10-14"},
		{name: "across a leap day", submittedOn: MustDate(2024, 2, 20), window: 15, want: "2024-03-05"},
		{name: "the same dates in a common year", submittedOn: MustDate(2023, 2, 20), window: 15, want: "2023-03-06"},
		{name: "the looser reading of the press release", submittedOn: MustDate(2026, 10, 31), window: 16, want: "2026-11-15"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := EFilingDeadline(c.submittedOn, c.window)
			if got.String() != c.want {
				t.Fatalf("EFilingDeadline(%s, %d) = %s, want %s", c.submittedOn, c.window, got, c.want)
			}
		})
	}
	if !EFilingDeadline(Date{}, 15).IsZero() {
		t.Fatal("an e-filing deadline was produced with no submission date")
	}
}

func TestRevocationDeadline(t *testing.T) {
	exact, err := ParseStamp("2026-04-05T14:30")
	if err != nil {
		t.Fatalf("ParseStamp: %v", err)
	}
	deadline, isExact := RevocationDeadline(exact, 48)
	if deadline.String() != "2026-04-07T14:30" || !isExact {
		t.Fatalf("RevocationDeadline(%s, 48) = %s exact=%v", exact, deadline, isExact)
	}

	dateOnly := DateStamp(MustDate(2026, 4, 5))
	deadline, isExact = RevocationDeadline(dateOnly, 48)
	if deadline.String() != "2026-04-07T00:00" {
		t.Fatalf("a date-only generation gave %s", deadline)
	}
	if isExact {
		t.Fatal("a deadline worked out from a date with no time was reported as exact")
	}

	if deadline, _ := RevocationDeadline(Stamp{}, 48); !deadline.IsZero() {
		t.Fatal("a revocation deadline was produced with no generation stamp")
	}
	if deadline, _ := RevocationDeadline(exact, 0); !deadline.IsZero() {
		t.Fatal("a revocation deadline was produced for a window of zero hours")
	}
}

func TestSettingsValidate(t *testing.T) {
	cases := []struct {
		name     string
		settings Settings
		wantErr  bool
	}{
		{name: "the reference values", settings: DefaultSettings()},
		{name: "the old 15-day window", settings: Settings{GenerationWindowDays: 15, EFilingWindowDays: 15, RevocationWindowHours: 48, WarnDays: 5}},
		{name: "the shortest usable warning", settings: Settings{GenerationWindowDays: 60, EFilingWindowDays: 15, RevocationWindowHours: 48, WarnDays: 1}},
		{name: "the shortest usable window", settings: Settings{GenerationWindowDays: 2, EFilingWindowDays: 15, RevocationWindowHours: 48, WarnDays: 1}},
		{name: "no early warning at all", settings: Settings{GenerationWindowDays: 60, EFilingWindowDays: 15, RevocationWindowHours: 48, WarnDays: 0}, wantErr: true},
		{name: "a window of zero days", settings: Settings{GenerationWindowDays: 0, EFilingWindowDays: 15, RevocationWindowHours: 48}, wantErr: true},
		{name: "a one-day window", settings: Settings{GenerationWindowDays: 1, EFilingWindowDays: 15, RevocationWindowHours: 48, WarnDays: 1}, wantErr: true},
		{name: "a window of a year and a day", settings: Settings{GenerationWindowDays: 366, EFilingWindowDays: 15, RevocationWindowHours: 48}, wantErr: true},
		{name: "a negative e-filing window", settings: Settings{GenerationWindowDays: 60, EFilingWindowDays: -1, RevocationWindowHours: 48}, wantErr: true},
		{name: "no revocation window", settings: Settings{GenerationWindowDays: 60, EFilingWindowDays: 15, RevocationWindowHours: 0}, wantErr: true},
		{name: "a warning further out than the window", settings: Settings{GenerationWindowDays: 60, EFilingWindowDays: 15, RevocationWindowHours: 48, WarnDays: 60}, wantErr: true},
		{name: "a negative warning", settings: Settings{GenerationWindowDays: 60, EFilingWindowDays: 15, RevocationWindowHours: 48, WarnDays: -1}, wantErr: true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			err := c.settings.Validate()
			if c.wantErr && err == nil {
				t.Fatalf("%+v was accepted", c.settings)
			}
			if !c.wantErr && err != nil {
				t.Fatalf("%+v was rejected: %v", c.settings, err)
			}
		})
	}
}

func TestSettingsWithDefaultsRepairsAHandEditedFile(t *testing.T) {
	got := Settings{}.withDefaults()
	if got != DefaultSettings() {
		t.Fatalf("an empty settings block came back as %+v, want %+v", got, DefaultSettings())
	}

	// A window shorter than the warning would otherwise make every row urgent.
	got = Settings{GenerationWindowDays: 5}.withDefaults()
	if got.WarnDays >= got.GenerationWindowDays {
		t.Fatalf("a 5-day window kept a %d-day warning", got.WarnDays)
	}
	if err := got.Validate(); err != nil {
		t.Fatalf("the repaired settings do not validate: %v", err)
	}

	got = Settings{GenerationWindowDays: 30, EFilingWindowDays: 15, RevocationWindowHours: 24, WarnDays: 7}.withDefaults()
	if got.GenerationWindowDays != 30 || got.RevocationWindowHours != 24 || got.WarnDays != 7 {
		t.Fatalf("deliberate overrides were changed: %+v", got)
	}
}
