package udin

import "testing"

func TestParseCategory(t *testing.T) {
	cases := []struct {
		in      string
		want    Category
		wantErr bool
	}{
		{in: "certificate", want: CategoryCertificate},
		{in: "Certificates", want: CategoryCertificate},
		{in: "  CERT  ", want: CategoryCertificate},
		{in: "gst-tax-audit", want: CategoryGSTTaxAudit},
		{in: "GST & Tax Audit", want: CategoryGSTTaxAudit},
		{in: "tax audit", want: CategoryGSTTaxAudit},
		{in: "audit-assurance", want: CategoryAuditAssurance},
		{in: "Audit and Assurance", want: CategoryAuditAssurance},
		{in: "attestation", want: CategoryAuditAssurance},
		{in: "", wantErr: true},
		{in: "peer review", wantErr: true},
	}
	for _, c := range cases {
		t.Run(c.in, func(t *testing.T) {
			got, err := ParseCategory(c.in)
			if c.wantErr {
				if err == nil {
					t.Fatalf("ParseCategory(%q) = %q, want an error", c.in, got)
				}
				return
			}
			if err != nil {
				t.Fatalf("ParseCategory(%q): %v", c.in, err)
			}
			if got != c.want {
				t.Fatalf("ParseCategory(%q) = %q, want %q", c.in, got, c.want)
			}
		})
	}
}

func TestMandatoryFromIsThePhasedSchedule(t *testing.T) {
	cases := []struct {
		category Category
		want     string
	}{
		{category: CategoryCertificate, want: "2019-02-01"},
		{category: CategoryGSTTaxAudit, want: "2019-04-01"},
		{category: CategoryAuditAssurance, want: "2019-07-01"},
	}
	for _, c := range cases {
		t.Run(string(c.category), func(t *testing.T) {
			if got := c.category.MandatoryFrom().String(); got != c.want {
				t.Fatalf("%s.MandatoryFrom() = %s, want %s", c.category, got, c.want)
			}
		})
	}
	if !Category("invented").MandatoryFrom().IsZero() {
		t.Fatal("an unknown category produced a mandate date")
	}
}

func TestWasMandatoryAtThePhaseBoundaries(t *testing.T) {
	cases := []struct {
		name     string
		category Category
		signedOn Date
		want     bool
	}{
		{name: "a certificate the day before the mandate", category: CategoryCertificate, signedOn: MustDate(2019, 1, 31)},
		{name: "a certificate on the first day of the mandate", category: CategoryCertificate, signedOn: MustDate(2019, 2, 1), want: true},
		{name: "a tax audit report in February 2019, before its phase", category: CategoryGSTTaxAudit, signedOn: MustDate(2019, 2, 15)},
		{name: "a tax audit report on 31 March 2019", category: CategoryGSTTaxAudit, signedOn: MustDate(2019, 3, 31)},
		{name: "a tax audit report on 1 April 2019", category: CategoryGSTTaxAudit, signedOn: MustDate(2019, 4, 1), want: true},
		{name: "a statutory audit report on 30 June 2019", category: CategoryAuditAssurance, signedOn: MustDate(2019, 6, 30)},
		{name: "a statutory audit report on 1 July 2019", category: CategoryAuditAssurance, signedOn: MustDate(2019, 7, 1), want: true},
		{name: "anything signed today", category: CategoryAuditAssurance, signedOn: MustDate(2026, 11, 24), want: true},
		{name: "a document with no signing date", category: CategoryAuditAssurance, signedOn: Date{}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := c.category.WasMandatory(c.signedOn); got != c.want {
				t.Fatalf("%s.WasMandatory(%s) = %v, want %v", c.category, c.signedOn, got, c.want)
			}
		})
	}
}

func TestCategoryLabelsAndValidity(t *testing.T) {
	if len(AllCategories()) != 3 {
		t.Fatalf("AllCategories() returned %d categories", len(AllCategories()))
	}
	for _, category := range AllCategories() {
		if !category.Valid() {
			t.Fatalf("%q did not report as valid", category)
		}
		if category.Label() == "" || category.Label() == string(category) {
			t.Fatalf("%q has no readable label", category)
		}
		types := CommonDocumentTypes(category)
		if len(types) < 3 {
			t.Fatalf("%q suggests only %d document types", category, len(types))
		}
		seen := map[string]bool{}
		for _, name := range types {
			if name == "" {
				t.Fatalf("%q suggests an empty document type", category)
			}
			if seen[name] {
				t.Fatalf("%q suggests %q twice", category, name)
			}
			seen[name] = true
		}
	}
	if Category("invented").Valid() {
		t.Fatal("an unknown category reported as valid")
	}
	if got := Category("invented").Label(); got != "invented" {
		t.Fatalf("an unknown category labelled itself %q", got)
	}
	if CommonDocumentTypes(Category("invented")) != nil {
		t.Fatal("an unknown category suggested document types")
	}
}
