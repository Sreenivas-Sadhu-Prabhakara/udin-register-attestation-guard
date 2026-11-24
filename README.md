# UDIN register and attestation guard

A free, offline register of every document your firm attests, with the UDIN deadline counted from the date of signing.

One Go binary with no dependencies. It serves a small interface on `127.0.0.1` and keeps everything in one JSON file on the same machine. **It does not generate UDINs** — only ICAI's portal can — and it makes no outbound network connection of any kind.

## What it does

- **Keeps the register.** Every signed document: UDIN category, document type, client and reference, financial year and assessment year, date of signing, engagement partner and membership number, the document description keyed on the portal, and the figures the UDIN was generated against. A practitioner who cannot produce this register is exposed; the register is the product.
- **Counts the generation window** from the date of signing, with both ends included the way ICAI counts them, and shows the three things worth alerting on: signed with no UDIN, approaching the deadline, and past it.
- **Runs the second clock too.** A form submitted on the income-tax e-filing portal before its UDIN existed has 15 calendar days to have the UDIN updated there. That deadline usually falls long before the sixty days from signing, so the register shows whichever binds and names the rule.
- **Checks the shape of a UDIN** against ICAI's documented syntax, and cross-checks it against the document: a number carrying another member's membership number, or a year the window never reached, is flagged.
- **Keeps revocations visible.** A revoked UDIN stays in the register with its reason, its replacement number and the 48-hour portal window, and a document still in circulation under a revoked number is flagged until it is recalled. An entry that carries a UDIN cannot be deleted.
- **Takes a pasted list.** Tab, comma or pipe separated, with or without a header row, day-first dates read the way an Indian audit report is dated. Every line it could not take in full is reported.
- **Exports the whole register as CSV**, missing and revoked entries included.

## Who it's for

Practising chartered accountants in India and their firms — sole practitioners, partnerships, and the article and audit staff who keep the register up to date. Members of ICSI and ICMAI keep their own UDIN disciplines with their own windows; the rules here are ICAI's, and every window is a setting you can change once you have confirmed your own institute's position.

## The regulation

The **ICAI UDIN mandate** — Unique Document Identification Number, with a 60-day generation window.

- **Mandatory in phases**, per the ICAI Council's 379th meeting of 17–18 December 2018: all certificates from 1 February 2019, all GST and tax audit reports from 1 April 2019, all other audit, assurance and attestation functions from 1 July 2019.
- **Generated at the time of signing**, and in any case within **60 days** of it — the ICAI announcement of 17 September 2021 (Council, 405th meeting), replacing the 15 days that applied before, aligning the window with SQC 1 and SA 230 on assembly of the audit file. The FAQ on UDIN states that **both the date of signing and the date of generation count** towards those sixty days, which is why a document signed on 1 January 2024 had until 29 February 2024 and not 1 March.
- **Revocable on the portal only within 48 hours** of generation — the announcement of 23 June 2023 (Council, 420th meeting). After that a verifier is told the number can no longer be revoked, and the member generates a fresh UDIN inside the generation window instead.
- **15 calendar days** to update a UDIN against a form already submitted on the income-tax e-filing portal, per the CBDT press release on the portal's integration with ICAI; if it is not updated, the submission is treated as invalid.
- Not generating a UDIN where it is mandatory amounts to non-adherence to a Council decision and, per the FAQ, may attract disciplinary proceedings under the Second Schedule, Part II of the Chartered Accountants Act, 1949.

Each of these figures lives in exactly one place in the code — `internal/udin/window.go` and `internal/udin/mandate.go` — is shown in the interface beside the announcement it comes from, and is a setting you can override if ICAI moves it.

## Install and run

```sh
git clone https://github.com/sreenivas-sadhu-prabhakara/udin-register-attestation-guard.git
cd udin-register-attestation-guard
go build -o udin-register .
./udin-register

# or without building first
go run .

# a register of your own, somewhere of your choosing
./udin-register -data ~/firm/udin-register-2026.json

# a different port
./udin-register -addr 127.0.0.1:9000
```

Then open <http://127.0.0.1:8080>. Stop it with Ctrl-C.

| Flag | Default | What it is |
| --- | --- | --- |
| `-addr` | `127.0.0.1:8080` | Address to listen on. Loopback only unless you deliberately change it. |
| `-data` | `./udin-register-data.json` | Where the register is kept. |

## Where the data is stored

In the one JSON file named by `-data`, on the machine you run the tool on, and nowhere else. Each save is written to a temporary file in the same folder and renamed over the old one, so an interrupted write leaves the previous register intact rather than half of a new one. The file is written readable by its owner only, because it holds client names and figures. Back it up with the rest of the firm's working papers.

Nothing leaves the machine: no telemetry, no analytics, no update check, no fonts or scripts from anywhere else, and no connection to the UDIN portal or any other server. The interface is served from inside the binary and talks only to `/api` on the same loopback address.

The local file is the point. ICAI archives UDINs a year after generation, and the portal is reached only through the signing member's own login — so a question about a three-year-old engagement is answered from the firm's own register or not at all.

## Scope and accuracy

**This tool does not generate UDINs, and cannot.** Only ICAI's portal at udin.icai.org issues a UDIN, and only the signing member can log in and generate one. There is no integration here, no automation of the portal, and no connection to it. The tool records the number you generated and counts the days.

A UDIN that passes the shape check is not thereby a real UDIN. ICAI publishes no check digit or checksum, so no arithmetic in this or any other tool can tell a well-formed invention from a genuine number — the shape check confirms the pattern and nothing more. Only verification on the portal can confirm that a number exists and what it was issued against.

The windows, dates and formats are stated as they stood when this was written, with the announcement behind each one named in the code and in the interface. They are indicative, not advice. **Confirm the current position at udin.icai.org** before relying on a date, and change the setting if ICAI has moved the line. Whether a UDIN was required for a particular engagement, and what to do about one that was missed, are professional judgements this tool does not make: it is a register and a reminder.

## The rules, and where they live

All of the domain logic is in `internal/udin` as pure functions over values — no clock, no files, no network — with table-driven tests covering the boundaries, including leap days, month ends and the 1 April financial-year boundary.

| File | What it holds |
| --- | --- |
| `date.go` | Calendar dates and stamps, day arithmetic, the inclusive count ICAI uses. |
| `finyear.go` | The Indian financial year, 1 April to 31 March, and the assessment year. |
| `window.go` | The three windows, their sources, the deadlines and the overridable settings. |
| `format.go` | The shape of a UDIN, and the cross-checks against the document. |
| `mandate.go` | The three categories and the phased dates from which a UDIN became mandatory. |
| `register.go` | The register: entries, validation, revocation, and what may not be deleted. |
| `status.go` | Stages, severities, findings, the board, the counts and the filters. |
| `paste.go` | Reading a pasted list of signed documents. |
| `csv.go` | The register as a flat file. |

```sh
gofmt -l .
go vet ./...
go test ./... -count=1
go build -o udin-register .
```

MIT licensed.
