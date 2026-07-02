# GOBL ➡️ Danish OIOUBL 2.1

Danish OIOUBL 2.1 support for [GOBL](https://github.com/invopop/gobl), used for
invoices, credit notes, Invoice Responses and Reminders exchanged over the
NemHandel network.

Released under the Apache 2.0 [LICENSE](https://github.com/invopop/gobl.dk.oioubl/blob/main/LICENSE), Copyright 2026 [Invopop S.L.](https://invopop.com).

The module has two halves:

- the **addon** (`addon/`, key `dk-oioubl-v2`) — extensions, normalizers and
  validation rules registered into GOBL's global registry; and
- the **converter** (this root package) — GOBL ↔ OIOUBL 2.1 XML, both
  directions, for all four document types.

## The addon

The addon implements the OIOUBL 2.1 profile (schematron v1.17.2) as a GOBL tax
addon. It `Requires` the EN 16931 addon and layers the OIOUBL-specific rules
and extensions on top:

- **Tax categories** — the converter maps the GOBL VAT key to the OIOUBL
  `taxcategoryid` code directly (exempt supplies report as `ZeroRated` with a
  mandatory VATEX reason). Excise and other non-VAT duties are modelled as
  charges keyed with their `taxschemeid` duty code.
- **Payment** — the OIOUBL payment channel (IBAN / Giro / FIK) and the Giro/FIK
  kortart are derived from the payment means and reference.
- **Participants** — parties are routed by OIOUBL endpoints carrying the
  symbolic scheme and code (`DK:CVR:12345674`, `GLN:5790000436057`, `DK:SE:…`);
  a Danish party carrying only a tax identity derives its CVR participant
  automatically, and explicit endpoints or inboxes always win.
- **Invoice / credit note** — participant, contact and ordering references
  required by the OIOUBL schematron, plus the non-negative totals rule
  (corrections are credit notes in Denmark).
- **Invoice Response** — `bill.Status` validation for the OIOUBL
  ApplicationResponse (responsecode-1.1 event set, single response, party
  requirements).
- **Reminder** — `bill.Payment` (type `request`) validation for the OIOUBL
  Reminder (Rykker), with the `dk-oioubl-reminder-sequence` extension and the
  `advis` tag.

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
  # endpoints may be omitted: the addon derives the
  # DK:CVR:12345674 participant from the tax identity.
```

See [`examples/`](examples/) for complete documents with their calculated
envelopes.

## The converter

The root package converts GOBL envelopes to OIOUBL 2.1 XML and back. It builds
on [`gobl.ubl`](https://github.com/invopop/gobl.ubl) for the shared UBL
plumbing (serialization, namespaces, UBL version), while the OIOUBL document
model and mapping live here — OIOUBL is not a context of the generic converter.

| GOBL document                 | OIOUBL document       |
|-------------------------------|-----------------------|
| `bill.Invoice`                | Invoice / CreditNote  |
| `bill.Status`                 | ApplicationResponse   |
| `bill.Payment` (`request`)    | Reminder (Rykker)     |

```go
import dkoioubl "github.com/invopop/gobl.dk.oioubl"

// GOBL -> OIOUBL
doc, err := dkoioubl.ConvertInvoice(env) // or dkoioubl.Convert for any type
data, err := dkoioubl.Bytes(doc)

// OIOUBL -> GOBL
doc, err := dkoioubl.Parse(data)
if inv, ok := doc.(*dkoioubl.Invoice); ok {
    env, err := inv.Convert()
}
```

Missing addons are ensured during conversion, so a plain EN 16931 invoice is
normalized and validated under the OIOUBL rules before serialization.

### Testing

Golden tests cover conversion and parsing for every document type:

```bash
go test ./...
go test ./... -update    # regenerate the golden files
```

With a local [invopop/phive](https://github.com/invopop/phive) service on
`127.0.0.1:9090`, the generated XML is additionally validated against the
official OIOUBL schematron (`dk.oioubl:*:1.17.2`):

```bash
go test . -validate
```
