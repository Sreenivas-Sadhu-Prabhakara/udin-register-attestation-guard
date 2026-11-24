package udin

// Severity is how loudly the register should say something. It is ordered, so a
// row carrying several findings takes the worst of them.
type Severity string

const (
	// SeverityOK is nothing to do.
	SeverityOK Severity = "ok"
	// SeverityNotice is worth knowing but needs no action today.
	SeverityNotice Severity = "notice"
	// SeverityWarn is a deadline coming up, or a record that does not add up.
	SeverityWarn Severity = "warn"
	// SeverityUrgent is the last day, or a window about to shut.
	SeverityUrgent Severity = "urgent"
	// SeverityBreach is a rule already missed, or a document in circulation
	// that should not be.
	SeverityBreach Severity = "breach"
)

var severityRank = map[Severity]int{
	SeverityOK:     0,
	SeverityNotice: 1,
	SeverityWarn:   2,
	SeverityUrgent: 3,
	SeverityBreach: 4,
}

// Rank orders severities. An unknown value sorts as OK rather than crashing a
// report built from a hand-edited data file.
func (s Severity) Rank() int { return severityRank[s] }

// Worse returns whichever of the two severities is higher.
func Worse(a, b Severity) Severity {
	if b.Rank() > a.Rank() {
		return b
	}
	return a
}

// Finding is one thing the register noticed about one entry: a malformed UDIN,
// a membership number that does not match the partner who signed, a document
// still in circulation under a revoked number.
//
// Findings are data, not text the interface has to parse: Code is stable and
// can be filtered on, Message is the sentence a practitioner reads.
type Finding struct {
	Code     string   `json:"code"`
	Severity Severity `json:"severity"`
	Message  string   `json:"message"`
}

// worstOf is the severity of the loudest finding in a list.
func worstOf(findings []Finding) Severity {
	worst := SeverityOK
	for _, f := range findings {
		worst = Worse(worst, f.Severity)
	}
	return worst
}
