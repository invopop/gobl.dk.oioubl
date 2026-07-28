# GOBL ⬅️➡️ Danish OIOUBL 2.1

Danish OIOUBL 2.1 support for [GOBL](https://github.com/invopop/gobl), for
invoices and credit notes exchanged over the NemHandel network. Invoice
Responses and Reminders are a planned follow-up, not yet implemented here.

Released under the Apache 2.0 [LICENSE](https://github.com/invopop/gobl.dk.oioubl/blob/main/LICENSE), Copyright 2026 [Invopop S.L.](https://invopop.com).

[![Lint](https://github.com/invopop/gobl.dk.oioubl/actions/workflows/lint.yaml/badge.svg)](https://github.com/invopop/gobl.dk.oioubl/actions/workflows/lint.yaml)
[![Test Go](https://github.com/invopop/gobl.dk.oioubl/actions/workflows/test.yaml/badge.svg)](https://github.com/invopop/gobl.dk.oioubl/actions/workflows/test.yaml)
[![Go Report Card](https://goreportcard.com/badge/github.com/invopop/gobl.dk.oioubl)](https://goreportcard.com/report/github.com/invopop/gobl.dk.oioubl)
[![codecov](https://codecov.io/gh/invopop/gobl.dk.oioubl/graph/badge.svg)](https://codecov.io/gh/invopop/gobl.dk.oioubl)
[![GoDoc](https://godoc.org/github.com/invopop/gobl.dk.oioubl?status.svg)](https://godoc.org/github.com/invopop/gobl.dk.oioubl)
![Latest Tag](https://img.shields.io/github/v/tag/invopop/gobl.dk.oioubl)
[![Ask DeepWiki](https://deepwiki.com/badge.svg)](https://deepwiki.com/invopop/gobl.dk.oioubl)

This module is two things: a GOBL tax addon (`dk-oioubl-v2`) carrying the OIOUBL
rules and extensions, and a converter in both directions built on
[`gobl.ubl`](https://github.com/invopop/gobl.ubl)'s EN 16931 base.

- `ConvertInvoice(env)` — GOBL → OIOUBL 2.1
- `Parse(data)` / `ParseInvoice(data)` — OIOUBL 2.1 → GOBL

Both target the OIOUBL 2.1 profile, schematron v1.17.2.

## The addon

`dk-oioubl-v2` `Requires` the EN 16931 addon and layers OIOUBL on top:

- **Tax categories** — VAT is restricted to what OIOUBL's `taxcategoryid-1.1`
  codelist supports: standard-rated, zero-rated and reverse-charge. There is no
  exempt category on the wire, so an exempt VAT key is **rejected** rather than
  relabelled — state the supply as zero-rated instead. Excise and other non-VAT
  duties are modelled as charges (as in EN 16931 / Peppol BIS), not tax
  categories, and carry a SKAT duty code extension.
- **Payment** — the OIOUBL payment channel is derived from the payment means:
  `IBAN`, `DK:GIRO`, `DK:FIK`, `DK:BANK` and `DK:NEMKONTO`. Each shape has its
  own account rules (Giro's 7-8 digit account, FIK's 8-character creditor
  account, the 4-digit registration number a domestic transfer needs), and
  NemKonto rejects account details outright, since it resolves the payee's
  registered account.
- **Participants** — parties are routed by OIOUBL endpoints, whose URI carries
  the symbolic scheme (`DK:CVR:12345674`, and likewise `DK:SE`, `GLN`). A Danish
  party carrying only a tax identity derives its `DK:CVR` endpoint
  automatically; explicit endpoints or inboxes always win.
- **Invoice / credit note** — the document type must be an OIOUBL-supported code
  (325/380/393, or 381 for a credit note), the customer must resolve to an
  endpoint for NemHandel to route to, every line needs a VAT category and a
  non-zero quantity, and ordering is required once any line carries an order
  reference.

Rules the OIOUBL schematron already enforces are generally left to it rather
than duplicated here; the addon aims at the paths a sender realistically hits.

The addon registers extensions, normalizers and validation rules into GOBL's
global registry, and lives in its own module so only projects handling Danish
documents take on its weight.

The UBL document structs and their XML mapping come from `gobl.ubl`, reused here
as `type Invoice ubl.Invoice`. There is no OIOUBL context in `gobl.ubl` — the
outbound converter asks it for a plain EN 16931 document
(`ubl.WithContext(ubl.ContextEN16931)`) and does the OIOUBL-specific shaping in
this module, so `gobl.ubl` stays free of anything Danish.

## Usage

Import the addon for its side effects to register it, then declare the
`dk-oioubl-v2` addon on a GOBL document:

```go
import _ "github.com/invopop/gobl.dk.oioubl/addon"
```

```yaml
$schema: "https://gobl.org/draft-0/bill/invoice"
$regime: "DK"
$addons:
  - "dk-oioubl-v2"
supplier:
  name: "Eksempel A/S"
  tax_id:
    country: "DK"
    code: "12345674"
  # endpoints may be omitted: the addon derives
  # DK:CVR:12345674 from the tax identity.
```

See [`examples/`](examples/) for complete invoice and credit note documents with
their calculated envelopes.
