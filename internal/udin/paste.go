package udin

import (
	"encoding/csv"
	"fmt"
	"strings"
)

// maxPasteLines stops a mis-paste of a whole year's ledger from turning into a
// register nobody meant to create.
const maxPasteLines = 2000

// PasteResult is what a pasted list came to: the draft entries, the columns the
// parser recognised, and everything it had to say about rows it could not read
// in full. Nothing is silently dropped.
type PasteResult struct {
	Entries  []Entry  `json:"entries"`
	Columns  []string `json:"columns"`
	Warnings []string `json:"warnings"`
}

// column names the parser understands, keyed by the normalised header text.
var pasteColumns = map[string]string{
	"client": "client", "clientname": "client", "party": "client", "assessee": "client", "name": "client",
	"document": "documentType", "documenttype": "documentType", "doc": "documentType",
	"particulars": "documentType", "report": "documentType", "certificate": "documentType",
	"signed": "signedOn", "signedon": "signedOn", "dateofsigning": "signedOn",
	"signingdate": "signedOn", "date": "signedOn", "dated": "signedOn", "datesigned": "signedOn",
	"udin": "udin", "udinno": "udin", "udinnumber": "udin",
	"udindate": "generatedOn", "generated": "generatedOn", "generatedon": "generatedOn",
	"dateofgeneration": "generatedOn", "udingeneratedon": "generatedOn",
	"fy": "financialYear", "financialyear": "financialYear", "year": "financialYear", "period": "financialYear",
	"partner": "partnerName", "signedby": "partnerName", "engagementpartner": "partnerName", "member": "partnerName",
	"mrn": "partnerMrn", "membership": "partnerMrn", "membershipno": "partnerMrn",
	"membershipnumber": "partnerMrn", "mno": "partnerMrn",
	"category": "category", "udincategory": "category",
	"description": "description", "documentdescription": "description",
	"pan": "clientRef", "gstin": "clientRef", "clientref": "clientRef",
	"ref": "clientRef", "clientcode": "clientRef",
	"efiling": "eFilingSubmittedOn", "efilingdate": "eFilingSubmittedOn",
	"submittedon": "eFilingSubmittedOn", "uploadedon": "eFilingSubmittedOn",
	"notes": "notes", "remarks": "notes", "remark": "notes",
}

// defaultPasteOrder is the order assumed when the pasted block has no header
// row. It is the order a firm's own tracking sheet almost always uses, and it
// is stated in the interface next to the box.
var defaultPasteOrder = []string{"client", "documentType", "signedOn", "udin", "generatedOn"}

// ParsePaste reads a pasted list of signed documents into draft entries.
//
// It accepts tab-separated text (what a spreadsheet puts on the clipboard),
// comma-separated text with quoting, and pipe-separated text. A header row is
// used when one is recognised; otherwise the columns are read in
// defaultPasteOrder.
//
// Dates are read day-first for the dotted and slashed forms, because that is
// how an Indian audit report is dated. Every row the parser could not take in
// full produces a warning naming the line, so the result can be checked
// against the paste.
//
// fallbackCategory applies to rows with no category of their own.
func ParsePaste(text string, fallbackCategory Category) (PasteResult, error) {
	result := PasteResult{Entries: []Entry{}, Columns: []string{}, Warnings: []string{}}
	if !fallbackCategory.Valid() {
		return result, fmt.Errorf("choose the UDIN category the pasted documents fall under")
	}

	lines := splitLines(text)
	if len(lines) == 0 {
		return result, fmt.Errorf("there is nothing in the pasted text")
	}
	if len(lines) > maxPasteLines {
		return result, fmt.Errorf("that is %d lines; this tool takes up to %d at a time", len(lines), maxPasteLines)
	}

	records, err := splitFields(lines)
	if err != nil {
		return result, err
	}

	order := defaultPasteOrder
	start := 0
	if mapped, ok := headerOrder(records[0]); ok {
		order = mapped
		start = 1
		if len(records) == 1 {
			return result, fmt.Errorf("the pasted text has a header row and no documents under it")
		}
	}
	result.Columns = append(result.Columns, order...)

	for i := start; i < len(records); i++ {
		line := i + 1
		entry, warnings, ok := entryFromFields(records[i], order, fallbackCategory, line)
		result.Warnings = append(result.Warnings, warnings...)
		if ok {
			result.Entries = append(result.Entries, entry)
		}
	}

	if len(result.Entries) == 0 {
		return result, fmt.Errorf("no document in the pasted text could be read; check that each line has a client, a document and a signing date")
	}
	return result, nil
}

func splitLines(text string) []string {
	raw := strings.Split(strings.ReplaceAll(text, "\r\n", "\n"), "\n")
	out := make([]string, 0, len(raw))
	for _, line := range raw {
		if strings.TrimSpace(line) == "" {
			continue
		}
		out = append(out, line)
	}
	return out
}

// splitFields works out what separates the columns and splits on it. A tab
// anywhere wins, because that is a spreadsheet paste and a tab is never part of
// a client's name.
func splitFields(lines []string) ([][]string, error) {
	joined := strings.Join(lines, "\n")
	switch {
	case strings.Contains(joined, "\t"):
		return splitOn(lines, "\t"), nil
	case strings.Contains(joined, "|"):
		return splitOn(lines, "|"), nil
	default:
		reader := csv.NewReader(strings.NewReader(joined))
		reader.FieldsPerRecord = -1
		reader.LazyQuotes = true
		reader.TrimLeadingSpace = true
		records, err := reader.ReadAll()
		if err != nil {
			return nil, fmt.Errorf("the pasted text could not be read as a list: %w", err)
		}
		return records, nil
	}
}

func splitOn(lines []string, sep string) [][]string {
	records := make([][]string, 0, len(lines))
	for _, line := range lines {
		fields := strings.Split(line, sep)
		for i := range fields {
			fields[i] = strings.TrimSpace(fields[i])
		}
		records = append(records, fields)
	}
	return records
}

// headerOrder recognises a header row. Two known column names are enough: one
// could be a client called "Date" or a document called "Particulars", two in a
// row could not.
func headerOrder(fields []string) ([]string, bool) {
	order := make([]string, len(fields))
	known := 0
	for i, field := range fields {
		if name, ok := pasteColumns[normaliseHeader(field)]; ok {
			order[i] = name
			known++
			continue
		}
		order[i] = ""
	}
	if known < 2 {
		return nil, false
	}
	return order, true
}

func normaliseHeader(field string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(field) {
		if r >= 'a' && r <= 'z' || r >= '0' && r <= '9' {
			b.WriteRune(r)
		}
	}
	return b.String()
}

func entryFromFields(fields, order []string, fallback Category, line int) (Entry, []string, bool) {
	var warnings []string
	entry := Entry{Category: fallback, Figures: []Figure{}}
	rawUDIN, rawGenerated := "", ""

	if len(fields) > len(order) {
		warnings = append(warnings, fmt.Sprintf("Line %d: %d columns were given but only %d could be placed; add a header row to name them.",
			line, len(fields), countNamed(order)))
	}

	for i, field := range fields {
		if i >= len(order) || order[i] == "" {
			continue
		}
		value := strings.TrimSpace(field)
		if value == "" {
			continue
		}
		switch order[i] {
		case "client":
			entry.ClientName = value
		case "documentType":
			entry.DocumentType = value
		case "description":
			entry.Description = value
		case "clientRef":
			entry.ClientRef = value
		case "partnerName":
			entry.PartnerName = value
		case "notes":
			entry.Notes = value
		case "udin":
			rawUDIN = value
		case "generatedOn":
			rawGenerated = value
		case "partnerMrn":
			if mrn, err := NormaliseMRN(value); err == nil {
				entry.PartnerMRN = mrn
			} else {
				warnings = append(warnings, fmt.Sprintf("Line %d: %q is not a membership number, so it was left off the entry.", line, value))
			}
		case "category":
			if category, err := ParseCategory(value); err == nil {
				entry.Category = category
			} else {
				warnings = append(warnings, fmt.Sprintf("Line %d: %q is not a UDIN category, so the chosen category was used.", line, value))
			}
		case "financialYear":
			if fy, err := ParseFinYear(value); err == nil {
				entry.FinancialYear = fy
			} else {
				warnings = append(warnings, fmt.Sprintf("Line %d: %q is not a financial year, so it was left off the entry.", line, value))
			}
		case "signedOn":
			if date, err := ParseDateFlexible(value); err == nil {
				entry.SignedOn = date
			} else {
				warnings = append(warnings, fmt.Sprintf("Line %d: %q is not a date this tool can read.", line, value))
			}
		case "eFilingSubmittedOn":
			if date, err := ParseDateFlexible(value); err == nil {
				entry.EFilingSubmittedOn = date
			} else {
				warnings = append(warnings, fmt.Sprintf("Line %d: the e-filing date %q could not be read, so it was left off the entry.", line, value))
			}
		}
	}

	if entry.ClientName == "" {
		warnings = append(warnings, fmt.Sprintf("Line %d was skipped: no client name.", line))
		return Entry{}, warnings, false
	}
	if entry.DocumentType == "" {
		warnings = append(warnings, fmt.Sprintf("Line %d was skipped: nothing said what the document is.", line))
		return Entry{}, warnings, false
	}
	if entry.SignedOn.IsZero() {
		warnings = append(warnings, fmt.Sprintf("Line %d was skipped: no signing date, and every UDIN deadline is counted from it.", line))
		return Entry{}, warnings, false
	}

	// A UDIN is only taken onto the entry when it is well formed and dated.
	// Anything else is kept as a note instead of being stored as a number the
	// register would then vouch for.
	if rawUDIN != "" {
		parsed, findings := ParseUDIN(rawUDIN)
		switch {
		case firstBlocking(findings) != nil:
			warnings = append(warnings, fmt.Sprintf("Line %d: %q is not in the 18-character UDIN form, so the entry was made without it and the text kept as a note. %s",
				line, rawUDIN, firstBlocking(findings).Message))
			entry.Notes = appendNote(entry.Notes, fmt.Sprintf("UDIN as pasted: %s", rawUDIN))
		case rawGenerated == "":
			warnings = append(warnings, fmt.Sprintf("Line %d: UDIN %s has no generation date, so the entry was made without it and the number kept as a note. The register needs the generation date to count the window.",
				line, parsed.Value))
			entry.Notes = appendNote(entry.Notes, fmt.Sprintf("UDIN as pasted: %s", parsed.Value))
		default:
			stamp, err := parseStampFlexible(rawGenerated)
			if err != nil {
				warnings = append(warnings, fmt.Sprintf("Line %d: the UDIN generation date %q could not be read, so UDIN %s was kept as a note instead.",
					line, rawGenerated, parsed.Value))
				entry.Notes = appendNote(entry.Notes, fmt.Sprintf("UDIN as pasted: %s, generated %s", parsed.Value, rawGenerated))
				break
			}
			entry.UDIN = parsed.Value
			entry.GeneratedAt = stamp
		}
	} else if rawGenerated != "" {
		warnings = append(warnings, fmt.Sprintf("Line %d: a generation date was given with no UDIN, so it was left off the entry.", line))
	}

	return entry, warnings, true
}

// parseStampFlexible reads a generation date from a pasted sheet, with or
// without a time of day.
func parseStampFlexible(value string) (Stamp, error) {
	if stamp, err := ParseStamp(value); err == nil {
		return stamp, nil
	}
	datePart, timePart, split := strings.Cut(value, " ")
	date, err := ParseDateFlexible(datePart)
	if err != nil {
		return Stamp{}, err
	}
	if !split || strings.TrimSpace(timePart) == "" {
		return DateStamp(date), nil
	}
	return ParseStamp(date.String() + "T" + strings.TrimSpace(timePart))
}

func appendNote(existing, addition string) string {
	if strings.TrimSpace(existing) == "" {
		return addition
	}
	return existing + " — " + addition
}

func countNamed(order []string) int {
	n := 0
	for _, name := range order {
		if name != "" {
			n++
		}
	}
	return n
}
