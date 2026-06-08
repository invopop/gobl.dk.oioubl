# Externalization — remaining steps

This module is **migrated locally**: the `dk-oioubl-v2-1` addon lives here (not in
gobl core), and `gobl.ubl` imports it. It currently builds via a dev `replace`
in `go.mod` against the local gobl core checked out on the **`externalize-dk-oioubl`**
branch — the branch that carries the EN 16931 OIOUBL relaxations and has no
in-core dk-oioubl addon. The steps below finalise it for publishing.

## Layout (mirrors `gobl.fr.ctc`)
- root `dkoioubl` package (`doc.go`) — reserved for OIOUBL tooling
- `addon/` — the `dk-oioubl-v2-1` addon (package `addon`); consumers import it
  aliased: `oioubl "github.com/invopop/gobl.dk.oioubl/addon"`
- `addon/en16931_carveout_test.go` — tests for core's EN 16931 relaxations that
  only apply with this addon (they moved out of core, which must not test-depend
  on an external addon)

## Gates & steps

1. **gobl 0.5 released** — brings PR #847: the approved external-addon registry
   (`tax.RegisterApprovedAddon`), `gobl/pkg/examples`, and fr/ctc externalised.
   *(Not released yet as of writing; expected imminently.)*

2. **A gobl core PR** (the OIOUBL core-side, always needed — only the addon
   *implementation* externalises, not its core hooks):
   - Land the EN 16931 OIOUBL relaxations in `addons/eu/en16931` (the
     `tax.AddonIn("dk-oioubl-v2-1")` guards — production code, literal key, no
     import). These are currently only on the `add-dk-oioubl-v2-1-addon` branch.
   - Register the approved key in `addons/external.go`: `dk-oioubl-v2-1`
     (`Key`, `Name`, `Module: "github.com/invopop/gobl.dk.oioubl"`).
   - (dk-oioubl was never merged into main's in-core addons, so there's nothing
     to delete there.)
   - Cut a gobl core tag.

3. **This module** — drop the `replace` in `go.mod`; pin
   `github.com/invopop/gobl` to the new core tag; add `examples/` +
   `examples_test.go` (using `gobl/pkg/examples`, available from 0.5); push to
   `invopop/gobl.dk.oioubl` and tag.

4. **gobl.ubl** — drop both dev `replace`s; pin the gobl core tag and the
   `gobl.dk.oioubl` tag.

Note: `gobl.ubl` only imports the addon for its keys/extension constants
(`oioubl.V2_1`, `oioubl.ExtKey*`); the XML serialization stays in `gobl.ubl`
(`ContextOIOUBL21`).
