package udin

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
)

// Limits on the free-text fields. The description limits are the portal's own:
// the ICAI FAQ states that Document Description must be filled in 5 to 250
// characters. The rest are there so one pasted spreadsheet cell cannot turn the
// register into something nothing can print.
const (
	// DescriptionMin and DescriptionMax mirror the portal's Document
	// Description field. Source: ICAI FAQ on UDIN, Q34.
	DescriptionMin = 5
	DescriptionMax = 250

	maxNameLength    = 160
	maxShortLength   = 120
	maxNotesLength   = 2000
	maxFiguresPerDoc = 8
	maxReasonLength  = 500
	minReasonLength  = 5

	// figuresExpected is how many keyword-and-value pairs the portal asks for
	// at the least, across the three categories. Fewer than this on an entry
	// that already carries a UDIN is a gap in the firm's own record rather
	// than a breach of anything.
	figuresExpected = 2
)

// Figure is one keyword and value a UDIN was generated against — turnover,
// shareholders' funds, the figure the certificate states. The portal asks for
// two or three of these and nothing is uploaded, so they are the only link
// between the number on the portal and the document in the file. A register
// that does not hold them cannot answer "is this the UDIN for this version of
// the accounts?", which is the question that arises when a draft and a final
// both exist.
type Figure struct {
	Keyword string `json:"keyword"`
	Value   string `json:"value"`
}

// Revocation is the record of a UDIN being withdrawn. It is only ever added,
// never removed: a revoked entry stays in the register, because the fact that a
// document was signed, numbered and then withdrawn is exactly what a later
// query is about.
type Revocation struct {
	// RevokedAt is when the revocation was made on the portal.
	RevokedAt Stamp `json:"revokedAt"`
	// Reason is what was entered in the portal's "reason to revoke this
	// document" field, kept here so the firm's own copy survives the portal's
	// one-year archiving of UDIN records.
	Reason string `json:"reason"`
	// ReplacementUDIN is the fresh UDIN issued for the corrected document, if
	// one was generated.
	ReplacementUDIN string `json:"replacementUdin"`
	// Recalled records that the document itself was taken back from whoever
	// held it. Until this is set, the register treats the document as still in
	// circulation under a revoked number.
	Recalled bool `json:"recalled"`
}

// Entry is one signed document in the register: what was signed, for whom, by
// which partner, against which figures, and what happened to its UDIN.
type Entry struct {
	ID            string   `json:"id"`
	Category      Category `json:"category"`
	DocumentType  string   `json:"documentType"`
	Description   string   `json:"description"`
	ClientName    string   `json:"clientName"`
	ClientRef     string   `json:"clientRef"`
	FinancialYear FinYear  `json:"financialYear"`

	// SignedOn is the date the member signed or certified the document. Every
	// clock in this register starts here.
	SignedOn Date `json:"signedOn"`

	PartnerName string `json:"partnerName"`
	PartnerMRN  string `json:"partnerMrn"`

	// UDIN is empty until the number exists. An empty UDIN on a signed
	// document is not a gap in the data; it is the thing the tool is for.
	UDIN string `json:"udin"`
	// GeneratedAt is when the UDIN was generated on the portal. The time of
	// day is optional and only matters for the revocation window.
	GeneratedAt Stamp `json:"generatedAt"`

	Figures []Figure `json:"figures"`

	// EFilingSubmittedOn is the date the document was submitted on the
	// income-tax e-filing portal, if it was. It starts the shorter CBDT clock.
	EFilingSubmittedOn Date `json:"eFilingSubmittedOn"`

	// InCirculation records that the signed document has gone out — to the
	// client, a bank, a regulator. It decides how loud a revocation is.
	InCirculation bool `json:"inCirculation"`

	Revocation *Revocation `json:"revocation"`

	Notes string `json:"notes"`
}

// HasUDIN reports whether a UDIN has been recorded.
func (e Entry) HasUDIN() bool { return strings.TrimSpace(e.UDIN) != "" }

// Revoked reports whether this entry's UDIN has been revoked.
func (e Entry) Revoked() bool { return e.Revocation != nil }

// Register is the whole book: the firm's settings and every entry. One file,
// one firm.
type Register struct {
	FirmName string   `json:"firmName"`
	Settings Settings `json:"settings"`
	Entries  []Entry  `json:"entries"`
	NextID   int      `json:"nextId"`
}

// New returns an empty register with the reference windows.
func New() *Register {
	return &Register{Settings: DefaultSettings(), Entries: []Entry{}, NextID: 1}
}

// Normalise repairs a register read from disk: it fills in settings a hand-edit
// left blank, puts the entries back in date order and makes sure the next
// identifier cannot collide with one already in use. A corrupt counter would
// otherwise overwrite an existing entry on the next save.
func (r *Register) Normalise() {
	r.Settings = r.Settings.withDefaults()
	if r.Entries == nil {
		r.Entries = []Entry{}
	}
	highest := 0
	for i := range r.Entries {
		if n := idNumber(r.Entries[i].ID); n > highest {
			highest = n
		}
		if r.Entries[i].Figures == nil {
			r.Entries[i].Figures = []Figure{}
		}
	}
	if r.NextID <= highest {
		r.NextID = highest + 1
	}
	if r.NextID < 1 {
		r.NextID = 1
	}
	r.sort()
}

// SetSettings replaces the windows after checking them.
func (r *Register) SetSettings(s Settings) error {
	if err := s.Validate(); err != nil {
		return err
	}
	r.Settings = s
	return nil
}

// SetFirmName records whose register this is; it appears on the export.
func (r *Register) SetFirmName(name string) error {
	name = strings.TrimSpace(name)
	if len(name) > maxNameLength {
		return fmt.Errorf("the firm name is longer than %d characters", maxNameLength)
	}
	r.FirmName = name
	return nil
}

// Entry finds one entry by identifier.
func (r *Register) Entry(id string) (Entry, bool) {
	for _, e := range r.Entries {
		if e.ID == id {
			return e, true
		}
	}
	return Entry{}, false
}

// Put adds a document or updates an existing one, and returns the entry as it
// was stored. today is passed in rather than read from the clock so the rule is
// testable and so a register exported from one machine and checked on another
// cannot disagree with itself.
//
// A revocation already on file is kept: it can only be set through Revoke, so
// that a stray edit cannot quietly erase the fact that a document was
// withdrawn.
func (r *Register) Put(in Entry, today Date) (Entry, error) {
	existingIndex := -1
	if id := strings.TrimSpace(in.ID); id != "" {
		for i := range r.Entries {
			if r.Entries[i].ID == id {
				existingIndex = i
				break
			}
		}
		if existingIndex < 0 {
			return Entry{}, fmt.Errorf("there is no entry %q in this register", id)
		}
		in.Revocation = r.Entries[existingIndex].Revocation
	} else {
		in.Revocation = nil
	}

	cleaned, err := r.clean(in, today)
	if err != nil {
		return Entry{}, err
	}

	if existingIndex >= 0 {
		r.Entries[existingIndex] = cleaned
	} else {
		cleaned.ID = fmt.Sprintf("E-%04d", r.NextID)
		r.NextID++
		r.Entries = append(r.Entries, cleaned)
	}
	r.sort()
	return cleaned, nil
}

// Revoke records that a UDIN has been withdrawn. The entry stays where it is
// and keeps its UDIN: the register's job from here on is to show that this
// document's number is dead, and to keep saying so while the document is still
// out in the world.
func (r *Register) Revoke(id string, revocation Revocation, today Date) (Entry, error) {
	index := -1
	for i := range r.Entries {
		if r.Entries[i].ID == id {
			index = i
			break
		}
	}
	if index < 0 {
		return Entry{}, fmt.Errorf("there is no entry %q in this register", id)
	}
	entry := r.Entries[index]
	if !entry.HasUDIN() {
		return Entry{}, fmt.Errorf("entry %s has no UDIN to revoke", id)
	}
	if entry.Revoked() {
		return Entry{}, fmt.Errorf("the UDIN on entry %s was already revoked on %s; a revocation is never undone, so record the replacement UDIN on a new entry if the document has been reissued",
			id, entry.Revocation.RevokedAt.Pretty())
	}

	reason := strings.TrimSpace(revocation.Reason)
	if len(reason) < minReasonLength {
		return Entry{}, fmt.Errorf("a revocation needs a reason of at least %d characters; the portal asks for one and the register keeps it", minReasonLength)
	}
	if len(reason) > maxReasonLength {
		return Entry{}, fmt.Errorf("the reason is longer than %d characters", maxReasonLength)
	}

	if revocation.RevokedAt.IsZero() {
		return Entry{}, fmt.Errorf("record when the UDIN was revoked")
	}
	if revocation.RevokedAt.Date().After(today) {
		return Entry{}, fmt.Errorf("the revocation is dated %s, which is in the future", revocation.RevokedAt.Date().Pretty())
	}
	if !entry.GeneratedAt.IsZero() && revocation.RevokedAt.Before(DateStamp(entry.GeneratedAt.Date())) {
		return Entry{}, fmt.Errorf("the revocation is dated %s but the UDIN was generated on %s; a UDIN cannot be revoked before it exists",
			revocation.RevokedAt.Date().Pretty(), entry.GeneratedAt.Date().Pretty())
	}

	replacement := ""
	if raw := strings.TrimSpace(revocation.ReplacementUDIN); raw != "" {
		parsed, findings := ParseUDIN(raw)
		if blocking := firstBlocking(findings); blocking != nil {
			return Entry{}, fmt.Errorf("the replacement UDIN is not usable: %s", blocking.Message)
		}
		if parsed.Value == NormaliseUDIN(entry.UDIN) {
			return Entry{}, fmt.Errorf("the replacement UDIN is the same number as the one being revoked")
		}
		replacement = parsed.Value
	}

	stored := Revocation{
		RevokedAt:       revocation.RevokedAt,
		Reason:          reason,
		ReplacementUDIN: replacement,
		Recalled:        revocation.Recalled,
	}
	r.Entries[index].Revocation = &stored
	return r.Entries[index], nil
}

// Recall marks the document as taken back out of circulation after its UDIN was
// revoked. It is the one part of a revocation that can change later, because it
// is a fact about the paper rather than about the portal.
func (r *Register) Recall(id string, recalled bool) (Entry, error) {
	for i := range r.Entries {
		if r.Entries[i].ID != id {
			continue
		}
		if !r.Entries[i].Revoked() {
			return Entry{}, fmt.Errorf("entry %s has no revoked UDIN", id)
		}
		r.Entries[i].Revocation.Recalled = recalled
		return r.Entries[i], nil
	}
	return Entry{}, fmt.Errorf("there is no entry %q in this register", id)
}

// Remove deletes an entry, and refuses to do so once a UDIN has been recorded
// against it.
//
// This is deliberate. A register whose uncomfortable rows can be deleted is
// worth nothing as evidence, and the uncomfortable rows — the signed document
// with no UDIN, the revoked number — are the ones somebody would want gone.
// Correcting a mistyped row is an edit; making a numbered document disappear is
// not something this tool will do.
func (r *Register) Remove(id string) error {
	for i := range r.Entries {
		if r.Entries[i].ID != id {
			continue
		}
		if r.Entries[i].HasUDIN() || r.Entries[i].Revoked() {
			return fmt.Errorf("entry %s carries a UDIN, so it stays in the register; correct the entry instead, and revoke the UDIN on the portal if the document was wrong", id)
		}
		r.Entries = append(r.Entries[:i], r.Entries[i+1:]...)
		return nil
	}
	return fmt.Errorf("there is no entry %q in this register", id)
}

// DistinctClients lists the client names already in the register, for the
// interface's suggestions. Spelling the same client three ways is how a
// register stops being searchable.
func (r *Register) DistinctClients() []string {
	seen := map[string]bool{}
	var out []string
	for _, e := range r.Entries {
		if e.ClientName == "" || seen[e.ClientName] {
			continue
		}
		seen[e.ClientName] = true
		out = append(out, e.ClientName)
	}
	sort.Strings(out)
	return out
}

// Partner is a signing member as the register knows them.
type Partner struct {
	Name string `json:"name"`
	MRN  string `json:"mrn"`
}

// DistinctPartners lists the engagement partners already in the register.
func (r *Register) DistinctPartners() []Partner {
	seen := map[string]bool{}
	var out []Partner
	for _, e := range r.Entries {
		if e.PartnerName == "" && e.PartnerMRN == "" {
			continue
		}
		key := e.PartnerName + "\x00" + e.PartnerMRN
		if seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, Partner{Name: e.PartnerName, MRN: e.PartnerMRN})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Name != out[j].Name {
			return out[i].Name < out[j].Name
		}
		return out[i].MRN < out[j].MRN
	})
	return out
}

// clean validates and tidies one entry. Everything it rejects is something the
// register could not honestly hold: a document with no signing date has no
// clock, and a UDIN that is not shaped like a UDIN is not a UDIN.
func (r *Register) clean(in Entry, today Date) (Entry, error) {
	out := Entry{
		ID:            strings.TrimSpace(in.ID),
		Category:      in.Category,
		DocumentType:  strings.TrimSpace(in.DocumentType),
		Description:   strings.TrimSpace(in.Description),
		ClientName:    strings.TrimSpace(in.ClientName),
		ClientRef:     strings.ToUpper(strings.TrimSpace(in.ClientRef)),
		FinancialYear: in.FinancialYear,
		SignedOn:      in.SignedOn,
		PartnerName:   strings.TrimSpace(in.PartnerName),
		InCirculation: in.InCirculation,
		Revocation:    in.Revocation,
		Notes:         strings.TrimSpace(in.Notes),
		Figures:       []Figure{},
	}

	if !out.Category.Valid() {
		if out.Category == "" {
			return Entry{}, fmt.Errorf("choose the UDIN category: certificate, GST and tax audit, or audit and assurance")
		}
		return Entry{}, fmt.Errorf("%q is not a UDIN category", string(out.Category))
	}
	if out.ClientName == "" {
		return Entry{}, fmt.Errorf("the client's name is needed: a register that cannot be searched by client cannot answer a query")
	}
	if len(out.ClientName) > maxNameLength {
		return Entry{}, fmt.Errorf("the client's name is longer than %d characters", maxNameLength)
	}
	if out.DocumentType == "" {
		return Entry{}, fmt.Errorf("say what the document is")
	}
	if len(out.DocumentType) > maxNameLength {
		return Entry{}, fmt.Errorf("the document type is longer than %d characters", maxNameLength)
	}
	if len(out.ClientRef) > maxShortLength {
		return Entry{}, fmt.Errorf("the client reference is longer than %d characters", maxShortLength)
	}
	if out.Description != "" {
		if length := len([]rune(out.Description)); length < DescriptionMin || length > DescriptionMax {
			return Entry{}, fmt.Errorf("the portal takes a document description of %d to %d characters; this one is %d",
				DescriptionMin, DescriptionMax, length)
		}
	}
	if len(out.Notes) > maxNotesLength {
		return Entry{}, fmt.Errorf("the note is longer than %d characters", maxNotesLength)
	}

	if out.SignedOn.IsZero() {
		return Entry{}, fmt.Errorf("the date the document was signed is needed: every UDIN deadline is counted from it")
	}
	if out.SignedOn.After(today) {
		return Entry{}, fmt.Errorf("the signing date %s is in the future", out.SignedOn.Pretty())
	}

	if out.PartnerName != "" && len(out.PartnerName) > maxNameLength {
		return Entry{}, fmt.Errorf("the partner's name is longer than %d characters", maxNameLength)
	}
	if raw := strings.TrimSpace(in.PartnerMRN); raw != "" {
		mrn, err := NormaliseMRN(raw)
		if err != nil {
			return Entry{}, fmt.Errorf("the engagement partner's membership number is not usable: %w", err)
		}
		out.PartnerMRN = mrn
	}

	for _, figure := range in.Figures {
		keyword := strings.TrimSpace(figure.Keyword)
		value := strings.TrimSpace(figure.Value)
		if keyword == "" && value == "" {
			continue
		}
		if keyword == "" {
			return Entry{}, fmt.Errorf("the figure %q has no keyword; the portal pairs every value with one", value)
		}
		if len(keyword) > maxShortLength || len(value) > maxShortLength {
			return Entry{}, fmt.Errorf("the figure %q is longer than %d characters", keyword, maxShortLength)
		}
		out.Figures = append(out.Figures, Figure{Keyword: keyword, Value: value})
	}
	if len(out.Figures) > maxFiguresPerDoc {
		return Entry{}, fmt.Errorf("the portal asks for a handful of figures, not %d", len(out.Figures))
	}

	if raw := strings.TrimSpace(in.UDIN); raw != "" {
		parsed, findings := ParseUDIN(raw)
		if blocking := firstBlocking(findings); blocking != nil {
			return Entry{}, fmt.Errorf("that is not a UDIN: %s", blocking.Message)
		}
		out.UDIN = parsed.Value

		if other, ok := r.entryWithUDIN(parsed.Value, out.ID); ok {
			return Entry{}, fmt.Errorf("UDIN %s is already recorded against entry %s (%s, %s). A UDIN belongs to one document; a duplicate copy of that document carries the same UDIN, so note it on that entry rather than opening a second one",
				parsed.Value, other.ID, other.ClientName, other.DocumentType)
		}

		if in.GeneratedAt.IsZero() {
			return Entry{}, fmt.Errorf("record the date the UDIN was generated: the register counts the 60-day window and the 48-hour revocation window from it")
		}
		if in.GeneratedAt.Date().Before(out.SignedOn) {
			return Entry{}, fmt.Errorf("the UDIN is dated %s, before the document was signed on %s",
				in.GeneratedAt.Date().Pretty(), out.SignedOn.Pretty())
		}
		if in.GeneratedAt.Date().After(today) {
			return Entry{}, fmt.Errorf("the UDIN generation date %s is in the future", in.GeneratedAt.Date().Pretty())
		}
		out.GeneratedAt = in.GeneratedAt
	} else if !in.GeneratedAt.IsZero() {
		return Entry{}, fmt.Errorf("there is a generation date but no UDIN; enter the number or clear the date")
	}

	if !in.EFilingSubmittedOn.IsZero() {
		if in.EFilingSubmittedOn.Before(out.SignedOn) {
			return Entry{}, fmt.Errorf("the e-filing submission date %s is before the document was signed on %s",
				in.EFilingSubmittedOn.Pretty(), out.SignedOn.Pretty())
		}
		if in.EFilingSubmittedOn.After(today) {
			return Entry{}, fmt.Errorf("the e-filing submission date %s is in the future", in.EFilingSubmittedOn.Pretty())
		}
		out.EFilingSubmittedOn = in.EFilingSubmittedOn
	}

	return out, nil
}

func (r *Register) entryWithUDIN(value, exceptID string) (Entry, bool) {
	for _, e := range r.Entries {
		if e.ID == exceptID {
			continue
		}
		if NormaliseUDIN(e.UDIN) == value {
			return e, true
		}
	}
	return Entry{}, false
}

// sort keeps the register in the order a book of attestations belongs in:
// oldest signing date first, and within a day the order the entries were made.
func (r *Register) sort() {
	sort.SliceStable(r.Entries, func(i, j int) bool {
		a, b := r.Entries[i], r.Entries[j]
		if !a.SignedOn.Equal(b.SignedOn) {
			return a.SignedOn.Before(b.SignedOn)
		}
		return idNumber(a.ID) < idNumber(b.ID)
	})
}

func firstBlocking(findings []Finding) *Finding {
	for i := range findings {
		if findings[i].Severity == SeverityBreach {
			return &findings[i]
		}
	}
	return nil
}

func idNumber(id string) int {
	_, digits, ok := strings.Cut(id, "-")
	if !ok {
		return 0
	}
	n, err := strconv.Atoi(digits)
	if err != nil {
		return 0
	}
	return n
}
