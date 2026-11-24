// Package udin holds the rules of the UDIN discipline for a practising
// chartered accountant's firm: the register of attested documents, the window
// in which a UDIN must be generated after signing, the shape of a UDIN, and
// what revocation does to a document that has already gone out.
//
// Everything here is a pure function over values. Nothing in this package
// reads the clock, touches a file, or opens a network connection — the day
// being assessed is always a parameter, so a reminder can be reproduced
// exactly, and the rules can be tested without a server.
//
// What this package does not do: it does not generate UDINs. A UDIN exists
// only when ICAI's portal issues one, and nothing here contacts that portal or
// any other. The register records numbers the member generated themselves and
// counts the days.
package udin

// PortalURL is where a UDIN is actually generated and verified. It appears in
// the interface as text to open yourself. This tool never calls it.
const PortalURL = "https://udin.icai.org"

// NotAGeneratorNote is the one thing about this tool that must never be
// misread, so it is stated in one place and shown wherever a UDIN is entered.
const NotAGeneratorNote = "This tool does not generate UDINs and cannot. Only ICAI's portal issues a UDIN, and only the member can log in and generate one. What this tool does is keep the register, count the days, and tell you what is outstanding."

// ArchivingNote is why a firm's own register is not a duplicate of the portal.
//
// Source: ICAI announcement of 25 October 2023 — UDINs are archived after one
// year of their generation to ease the load on the servers, and archived
// numbers are reached through a year filter on the member's own login.
const ArchivingNote = "ICAI archives UDINs a year after generation, and the portal is reached only through the signing member's own login. A query about a three-year-old engagement is answered from the firm's register or not at all."

// ConsequenceNote is what is at stake, stated once.
//
// Source: ICAI FAQ on UDIN — not generating a UDIN for a document for which it
// is mandatory amounts to non-adherence to a Council decision and may attract
// disciplinary proceedings under the Second Schedule, Part II of the Chartered
// Accountants Act, 1949.
const ConsequenceNote = "Not generating a UDIN where it is mandatory is non-adherence to a Council decision and may attract disciplinary proceedings under the Second Schedule, Part II of the Chartered Accountants Act, 1949."
