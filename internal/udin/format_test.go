package udin

import (
	"strings"
	"testing"
)

func TestNormaliseUDIN(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{in: "19304576AKTSBN1359", want: "19304576AKTSBN1359"},
		{in: "19304576aktsbn1359", want: "19304576AKTSBN1359"},
		{in: "19304576 AKTSBN 1359", want: "19304576AKTSBN1359"},
		{in: " 19-304576-AKTSBN1359 ", want: "19304576AKTSBN1359"},
		{in: "UDIN: 19304576AKTSBN1359", want: "UDIN19304576AKTSBN1359"},
		{in: "", want: ""},
	}
	for _, c := range cases {
		t.Run(c.in, func(t *testing.T) {
			if got := NormaliseUDIN(c.in); got != c.want {
				t.Fatalf("NormaliseUDIN(%q) = %q, want %q", c.in, got, c.want)
			}
		})
	}
}

func TestParseUDINAgainstTheDocumentedSyntax(t *testing.T) {
	cases := []struct {
		name         string
		in           string
		wantValid    bool
		wantYear     string
		wantMRN      string
		wantRandom   string
		wantCodes    []string
		wantSeverity Severity
	}{
		{
			name: "the example from the ICAI FAQ", in: "19304576AKTSBN1359",
			wantValid: true, wantYear: "19", wantMRN: "304576", wantRandom: "AKTSBN1359",
			wantSeverity: SeverityOK,
		},
		{
			name: "spaced and lower case, as copied from a report footer", in: "26 304576 aktsbn1359",
			wantValid: true, wantYear: "26", wantMRN: "304576", wantRandom: "AKTSBN1359",
			wantSeverity: SeverityOK,
		},
		{
			name: "a membership number with a leading zero", in: "26030457AKTSBN1359",
			wantValid: true, wantYear: "26", wantMRN: "030457", wantRandom: "AKTSBN1359",
			wantSeverity: SeverityOK,
		},
		{
			name: "nothing at all", in: "   ",
			wantCodes: []string{"udin-empty"}, wantSeverity: SeverityBreach,
		},
		{
			name: "one character short", in: "19304576AKTSBN135",
			wantCodes: []string{"udin-length"}, wantSeverity: SeverityBreach,
		},
		{
			name: "one character too long", in: "19304576AKTSBN13590",
			wantCodes: []string{"udin-length"}, wantSeverity: SeverityBreach,
		},
		{
			name: "a stray punctuation mark", in: "19304576AKTSBN13#9",
			wantCodes: []string{"udin-characters"}, wantSeverity: SeverityBreach,
		},
		{
			name: "letters where the year should be", in: "YY304576AKTSBN1359",
			wantCodes: []string{"udin-year-digits"}, wantSeverity: SeverityBreach,
		},
		{
			name: "letters in the membership number", in: "1930457XAKTSBN1359",
			wantCodes: []string{"udin-membership-digits"}, wantSeverity: SeverityBreach,
		},
		{
			name: "all digits, so the random part does not follow the documented shape", in: "193045761234567890",
			wantValid: true, wantYear: "19", wantMRN: "304576", wantRandom: "1234567890",
			wantCodes: []string{"udin-random-part-shape"}, wantSeverity: SeverityWarn,
		},
		{
			name: "the random part split the other way round", in: "193045761359AKTSBN",
			wantValid: true, wantCodes: []string{"udin-random-part-shape"}, wantSeverity: SeverityWarn,
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			parsed, findings := ParseUDIN(c.in)
			if got := ValidUDIN(c.in); got != c.wantValid {
				t.Fatalf("ValidUDIN(%q) = %v, want %v (findings: %v)", c.in, got, c.wantValid, findings)
			}
			if got := worstOf(findings); got != c.wantSeverity {
				t.Fatalf("worst finding for %q was %q, want %q (%v)", c.in, got, c.wantSeverity, findings)
			}
			for _, code := range c.wantCodes {
				if !hasFinding(findings, code) {
					t.Fatalf("%q did not produce finding %q; got %v", c.in, code, findings)
				}
			}
			if c.wantYear != "" && parsed.YearDigits != c.wantYear {
				t.Fatalf("year digits of %q = %q, want %q", c.in, parsed.YearDigits, c.wantYear)
			}
			if c.wantMRN != "" && parsed.MembershipNumber != c.wantMRN {
				t.Fatalf("membership number of %q = %q, want %q", c.in, parsed.MembershipNumber, c.wantMRN)
			}
			if c.wantRandom != "" && parsed.RandomPart != c.wantRandom {
				t.Fatalf("random part of %q = %q, want %q", c.in, parsed.RandomPart, c.wantRandom)
			}
			if c.wantValid && len(parsed.Value) != UDINLength {
				t.Fatalf("a valid UDIN came back as %q", parsed.Value)
			}
		})
	}
}

// Nothing in this package may claim to tell a real UDIN from a well-formed
// invention: ICAI publishes no check digit. This test holds that line.
func TestAWellFormedInventionPassesTheShapeCheck(t *testing.T) {
	invented := "26999999ZZZZZZ0000"
	if !ValidUDIN(invented) {
		t.Fatalf("%q is well formed and must pass the shape check; only the portal can say whether it exists", invented)
	}
	if !strings.Contains(NoCheckDigitNote, "udin.icai.org") {
		t.Fatalf("the note about the validator's limits does not point at the portal: %q", NoCheckDigitNote)
	}
}

func TestGenerationYearsCoverAWindowCrossingTheYearEnd(t *testing.T) {
	cases := []struct {
		name     string
		signedOn Date
		window   int
		want     []string
	}{
		{name: "inside one calendar year", signedOn: MustDate(2026, 4, 1), window: 60, want: []string{"26"}},
		{name: "across the year end", signedOn: MustDate(2025, 12, 20), window: 60, want: []string{"25", "26"}},
		{name: "signed on 31 December", signedOn: MustDate(2025, 12, 31), window: 60, want: []string{"25", "26"}},
		{name: "a one-day window on 31 December", signedOn: MustDate(2025, 12, 31), window: 1, want: []string{"25"}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := GenerationYears(c.signedOn, c.window)
			if strings.Join(got, ",") != strings.Join(c.want, ",") {
				t.Fatalf("GenerationYears(%s, %d) = %v, want %v", c.signedOn, c.window, got, c.want)
			}
		})
	}
	if GenerationYears(Date{}, 60) != nil {
		t.Fatal("years were produced for a document with no signing date")
	}
}

func TestCrossCheckCatchesAUDINOnTheWrongRow(t *testing.T) {
	cases := []struct {
		name       string
		udin       string
		signedOn   Date
		partnerMRN string
		wantCodes  []string
		wantClean  bool
	}{
		{
			name: "the partner's own number, signed the same year",
			udin: "26304576AKTSBN1359", signedOn: MustDate(2026, 4, 1), partnerMRN: "304576", wantClean: true,
		},
		{
			name: "the membership number written long-hand",
			udin: "26304576AKTSBN1359", signedOn: MustDate(2026, 4, 1), partnerMRN: "M. No. 304576", wantClean: true,
		},
		{
			name: "a six-digit number with a leading zero on the entry",
			udin: "26030457AKTSBN1359", signedOn: MustDate(2026, 4, 1), partnerMRN: "30457", wantClean: true,
		},
		{
			name: "another member's number",
			udin: "26304576AKTSBN1359", signedOn: MustDate(2026, 4, 1), partnerMRN: "123456",
			wantCodes: []string{"udin-mrn-mismatch"},
		},
		{
			name: "no partner recorded, so nothing to compare",
			udin: "26304576AKTSBN1359", signedOn: MustDate(2026, 4, 1), partnerMRN: "", wantClean: true,
		},
		{
			name: "generated in the second calendar year of a window that crosses the year end",
			udin: "26304576AKTSBN1359", signedOn: MustDate(2025, 12, 20), partnerMRN: "304576", wantClean: true,
		},
		{
			name: "generated in the first calendar year of that window",
			udin: "25304576AKTSBN1359", signedOn: MustDate(2025, 12, 20), partnerMRN: "304576", wantClean: true,
		},
		{
			name: "a year the window never reached",
			udin: "24304576AKTSBN1359", signedOn: MustDate(2025, 12, 20), partnerMRN: "304576",
			wantCodes: []string{"udin-year-mismatch"},
		},
		{
			name: "both wrong at once",
			udin: "21304576AKTSBN1359", signedOn: MustDate(2026, 4, 1), partnerMRN: "999999",
			wantCodes: []string{"udin-mrn-mismatch", "udin-year-mismatch"},
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			parsed, findings := ParseUDIN(c.udin)
			if blocking := firstBlocking(findings); blocking != nil {
				t.Fatalf("the test UDIN %q is not well formed: %s", c.udin, blocking.Message)
			}
			got := CrossCheck(parsed, c.signedOn, c.partnerMRN, 60)
			if c.wantClean && len(got) != 0 {
				t.Fatalf("CrossCheck produced %v, want nothing", got)
			}
			for _, code := range c.wantCodes {
				if !hasFinding(got, code) {
					t.Fatalf("CrossCheck did not produce %q; got %v", code, got)
				}
			}
		})
	}

	if got := CrossCheck(UDIN{}, MustDate(2026, 4, 1), "304576", 60); got != nil {
		t.Fatalf("CrossCheck on an empty UDIN produced %v", got)
	}
}

func TestNormaliseMRN(t *testing.T) {
	cases := []struct {
		in      string
		want    string
		wantErr bool
	}{
		{in: "304576", want: "304576"},
		{in: "0304576", want: "304576"},
		{in: "304 576", want: "304576"},
		{in: "M.No. 304576", want: "304576"},
		{in: "30457", want: "030457"},
		{in: "7", want: "000007"},
		{in: "1234567", wantErr: true},
		{in: "abcdef", wantErr: true},
		{in: "", wantErr: true},
		{in: "000000", wantErr: true},
	}
	for _, c := range cases {
		t.Run(c.in, func(t *testing.T) {
			got, err := NormaliseMRN(c.in)
			if c.wantErr {
				if err == nil {
					t.Fatalf("NormaliseMRN(%q) = %q, want an error", c.in, got)
				}
				return
			}
			if err != nil {
				t.Fatalf("NormaliseMRN(%q): %v", c.in, err)
			}
			if got != c.want {
				t.Fatalf("NormaliseMRN(%q) = %q, want %q", c.in, got, c.want)
			}
		})
	}
}

func TestSeverityOrdering(t *testing.T) {
	ladder := []Severity{SeverityOK, SeverityNotice, SeverityWarn, SeverityUrgent, SeverityBreach}
	for i := 1; i < len(ladder); i++ {
		if ladder[i].Rank() <= ladder[i-1].Rank() {
			t.Fatalf("%q does not rank above %q", ladder[i], ladder[i-1])
		}
		if got := Worse(ladder[i-1], ladder[i]); got != ladder[i] {
			t.Fatalf("Worse(%q, %q) = %q", ladder[i-1], ladder[i], got)
		}
		if got := Worse(ladder[i], ladder[i-1]); got != ladder[i] {
			t.Fatalf("Worse(%q, %q) = %q", ladder[i], ladder[i-1], got)
		}
	}
	if got := worstOf(nil); got != SeverityOK {
		t.Fatalf("worstOf(nil) = %q", got)
	}
	if got := Severity("made up").Rank(); got != 0 {
		t.Fatalf("an unknown severity ranked %d", got)
	}
}

func hasFinding(findings []Finding, code string) bool {
	for _, f := range findings {
		if f.Code == code {
			if strings.TrimSpace(f.Message) == "" {
				return false
			}
			return true
		}
	}
	return false
}
