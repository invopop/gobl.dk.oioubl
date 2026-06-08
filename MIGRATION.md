# Externalization — remaining steps

This module is **pre-staged**: the `dk-oioubl-v2-1` addon still also lives in
`gobl` core (`addons/dk/oioubl-v2-1`). It builds here against a local gobl core
via a `replace` in `go.mod`. The steps below complete the move once
[gobl PR #847](https://github.com/invopop/gobl/pull/847) (approved
external-addon registry) merges and a core tag is cut.

Gate: **#847 merged + gobl core tag cut** (provides `tax.RegisterApprovedAddon`
and keeps `bill.Status` / `is.*` in core).

1. **gobl core — register the approved key.** In `addons/external.go` add an
   entry for `dk-oioubl-v2-1` (`Key`, `Name`, `Module:
   "github.com/invopop/gobl.dk.oioubl"`). Editing that file is the approval step;
   it does **not** bypass the runtime gate (a document declaring the addon still
   needs this module imported).

2. **gobl core — remove the in-core addon.** Delete `addons/dk/oioubl-v2-1/`,
   `data/addons/dk-oioubl*.json` and `data/rules/dk-oioubl.json`.

3. **gobl core — relocate the en16931 carve-out tests.** `addons/eu/en16931/`
   production code keeps its OIOUBL relaxations (they reference the addon by the
   literal string `tax.AddonIn("dk-oioubl-v2-1")`, no import). But
   `bill_test.go` / `tax_combo_test.go` *import* the oioubl package — move those
   OIOUBL-specific cases into this module (e.g. `addon/en16931_carveout_test.go`)
   so core has no dependency on the external addon.

4. **This module — pin the core tag.** Drop the `replace` in `go.mod` and pin
   `github.com/invopop/gobl` to the new core tag.

5. **gobl.ubl — re-point the import.** Change `oioubl
   "github.com/invopop/gobl/addons/dk/oioubl-v2-1"` →
   `oioubl "github.com/invopop/gobl.dk.oioubl/addon"` (the alias and all
   `oioubl.*` references stay), add `github.com/invopop/gobl.dk.oioubl` to
   `go.mod`, and re-pin gobl core.

6. **Publish.** Push this module to `invopop/gobl.dk.oioubl` and tag it.

Note: `gobl.ubl` only imports the addon for its keys/extension constants
(`oioubl.V2_1`, `oioubl.ExtKey*`); the XML serialization stays in `gobl.ubl`
(`ContextOIOUBL21`).
