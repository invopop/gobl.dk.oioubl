# GOBL ➡️ Danish OIOUBL 2.1

Danish OIOUBL 2.1 addon for [GOBL](https://github.com/invopop/gobl), used for
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

This module implements the OIOUBL 2.1 profile (schematron v1.17.2) as a GOBL tax
addon (`dk-oioubl-v2`). It `Requires` the EN 16931 addon and layers the
OIOUBL-specific rules and extensions on top:

- **Tax categories** — VAT rates are restricted to what OIOUBL's `taxcategoryid`
  codelist supports (standard/zero/reverse-charge; exempt folds into zero-rated).
  Excise and other non-VAT duties are modelled as charges (as in EN 16931 /
  Peppol BIS), not tax categories.
- **Payment** — the OIOUBL payment channel (IBAN / Giro / FIK) is derived from the
  payment means.
- **Participants** — parties are routed by OIOUBL endpoints
  (`DK:CVR:<CVR>`); a Danish party carrying only a tax identity derives its CVR
  participant automatically, and explicit endpoints or inboxes always win. The endpoint URI carries the symbolic scheme
  (`DK:CVR`, `GLN`, `DK:SE`).
- **Invoice / credit note** — participant, contact and ordering references
  required by the OIOUBL schematron, plus the non-negative totals rule
  (corrections are credit notes in Denmark).

Unlike the format converters in the GOBL ecosystem, this is a true **addon**: it
registers extensions, normalizers and validation rules into GOBL's global
registry. It lives in its own module so that only projects handling Danish
OIOUBL documents take on its weight. The XML serialization itself lives in
[`gobl.ubl`](https://github.com/invopop/gobl.ubl) (`ContextOIOUBL21`).

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
  # iso6523-actorid-upis::0184:12345674 from the tax identity.
```

See [`examples/`](examples/) for complete invoice, credit note and Invoice
Response documents with their calculated envelopes.
