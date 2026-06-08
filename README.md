# GOBL ➡️ Danish OIOUBL 2.1

Danish OIOUBL 2.1 addon for [GOBL](https://github.com/invopop/gobl), used for
invoices, credit notes and Invoice Responses exchanged over the NemHandel
network.

Released under the Apache 2.0 [LICENSE](https://github.com/invopop/gobl.dk.oioubl/blob/main/LICENSE), Copyright 2026 [Invopop S.L.](https://invopop.com).

This module implements the OIOUBL 2.1 profile (schematron v1.17.2) as a GOBL tax
addon (`dk-oioubl-v2-1`). It `Requires` the EN 16931 addon and layers the
OIOUBL-specific rules and extensions on top:

- **Tax categories** — maps GOBL VAT to the OIOUBL `taxcategoryid` code list via
  the `dk-oioubl-tax-category` extension.
- **Payment** — OIOUBL payment channels (IBAN / Giro / FIK) and the structured
  Giro/FIK payment identifiers (`dk-oioubl-payment-channel`, `dk-oioubl-payment-id`).
- **Invoice / credit note** — supplier & customer inboxes, contacts and ordering
  references required by the OIOUBL schematron.
- **Invoice Response** — `bill.Status` validation for the OIOUBL
  ApplicationResponse (responsecode-1.1 event set, single response, party
  requirements).

Unlike the format converters in the GOBL ecosystem, this is a true **addon**: it
registers extensions, normalizers and validation rules into GOBL's global
registry. It lives in its own module so that only projects handling Danish
OIOUBL documents take on its weight. The XML serialization itself lives in
[`gobl.ubl`](https://github.com/invopop/gobl.ubl) (`ContextOIOUBL21`).

## Usage

Import the addon for its side effects to register it, then declare the
`dk-oioubl-v2-1` addon on a GOBL document:

```go
import _ "github.com/invopop/gobl.dk.oioubl/addon"
```
