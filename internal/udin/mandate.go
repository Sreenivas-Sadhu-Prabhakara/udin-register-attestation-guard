package udin

import (
	"fmt"
	"strings"
)

// Category is one of the three heads the UDIN portal is organised under. A
// document's category decides the date from which a UDIN became mandatory for
// it, which matters whenever a firm back-fills a register or answers a query
// about an old engagement.
type Category string

const (
	// CategoryCertificate covers certificates, including reports issued in
	// lieu of a certificate.
	CategoryCertificate Category = "certificate"
	// CategoryGSTTaxAudit covers GST and tax audit reports.
	CategoryGSTTaxAudit Category = "gst-tax-audit"
	// CategoryAuditAssurance covers the remaining audit, assurance and
	// attestation functions.
	CategoryAuditAssurance Category = "audit-assurance"
)

// MandateSource records the decision behind the three dates below.
const MandateSource = "ICAI Council, 379th meeting held on 17-18 December 2018: UDIN made mandatory in phases — all certificates from 1 February 2019, all GST and tax audit reports from 1 April 2019, all other audit, assurance and attestation functions from 1 July 2019."

// AllCategories lists the categories in the order the interface shows them.
func AllCategories() []Category {
	return []Category{CategoryCertificate, CategoryGSTTaxAudit, CategoryAuditAssurance}
}

// ParseCategory reads a category from the API or a pasted list, accepting the
// stored keys and the labels a person would type.
func ParseCategory(raw string) (Category, error) {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "certificate", "certificates", "cert":
		return CategoryCertificate, nil
	case "gst-tax-audit", "gst_tax_audit", "gst and tax audit", "gst & tax audit", "tax audit", "gst audit", "gst":
		return CategoryGSTTaxAudit, nil
	case "audit-assurance", "audit_assurance", "audit & assurance", "audit and assurance", "audit", "assurance", "attestation":
		return CategoryAuditAssurance, nil
	case "":
		return "", fmt.Errorf("no document category given (certificate, gst-tax-audit or audit-assurance)")
	}
	return "", fmt.Errorf("%q is not a UDIN category; use certificate, gst-tax-audit or audit-assurance", raw)
}

// Label is the category as the portal names it.
func (c Category) Label() string {
	switch c {
	case CategoryCertificate:
		return "Certificate"
	case CategoryGSTTaxAudit:
		return "GST and tax audit"
	case CategoryAuditAssurance:
		return "Audit and assurance"
	}
	return string(c)
}

// Valid reports whether the category is one of the three.
func (c Category) Valid() bool {
	switch c {
	case CategoryCertificate, CategoryGSTTaxAudit, CategoryAuditAssurance:
		return true
	}
	return false
}

// MandatoryFrom is the date from which a UDIN became mandatory for this
// category. See MandateSource.
func (c Category) MandatoryFrom() Date {
	switch c {
	case CategoryCertificate:
		return MustDate(2019, 2, 1)
	case CategoryGSTTaxAudit:
		return MustDate(2019, 4, 1)
	case CategoryAuditAssurance:
		return MustDate(2019, 7, 1)
	}
	return Date{}
}

// WasMandatory reports whether a document in this category, signed on signedOn,
// needed a UDIN. A register that flagged a 2018 certificate as a missing UDIN
// would be crying wolf, and a register that cried wolf would be ignored.
func (c Category) WasMandatory(signedOn Date) bool {
	from := c.MandatoryFrom()
	if from.IsZero() || signedOn.IsZero() {
		return false
	}
	return !signedOn.Before(from)
}

// CommonDocumentTypes are suggestions for the document-type field, offered as
// hints in the interface. They are the engagements a small practice meets most
// weeks; the portal's own drop-down is far longer and is the authority on what
// a document should be filed as. Nothing in this package validates the field
// against this list.
func CommonDocumentTypes(c Category) []string {
	switch c {
	case CategoryCertificate:
		return []string{
			"Net worth certificate",
			"Turnover certificate",
			"Form 15CB (remittance certificate)",
			"Certificate for a bank or financial institution",
			"Capital contribution certificate",
			"Certificate of utilisation of funds",
			"Any other certificate",
		}
	case CategoryGSTTaxAudit:
		return []string{
			"Form 3CA-3CD (tax audit report, section 44AB)",
			"Form 3CB-3CD (tax audit report, section 44AB)",
			"Form 10B / 10BB (trust audit)",
			"Form 29B (MAT)",
			"Form GSTR-9C (GST reconciliation statement)",
			"Form ADT-04 (special GST audit)",
			"Any other tax audit report",
		}
	case CategoryAuditAssurance:
		return []string{
			"Statutory audit report (Companies Act, section 143)",
			"Report on internal financial controls",
			"Limited review report",
			"Bank branch statutory audit report",
			"Concurrent or internal audit report",
			"Co-operative society audit report",
			"Any other audit or assurance report",
		}
	}
	return nil
}
