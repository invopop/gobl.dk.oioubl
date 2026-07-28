package addon_test

import (
	"testing"

	"github.com/invopop/gobl/org"
	"github.com/invopop/gobl/rules"
	"github.com/invopop/gobl/tax"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPartyValidation(t *testing.T) {
	t.Run("bare DK supplier passes via the derived participant (F-INV031)", func(t *testing.T) {
		inv := testInvoiceStandard(t)
		inv.Supplier.Inboxes = nil
		require.NoError(t, inv.Calculate())
		assert.NoError(t, rules.Validate(inv))
	})

	t.Run("supplier without participant or tax ID code fails (F-INV031)", func(t *testing.T) {
		inv := testInvoiceStandard(t)
		inv.Supplier.Inboxes = nil
		inv.Supplier.TaxID = &tax.Identity{Country: "DK"}
		require.NoError(t, inv.Calculate())
		err := rules.Validate(inv)
		assert.ErrorContains(t, err, "F-INV031")
	})

	t.Run("customer without participant or tax ID code fails (F-INV044)", func(t *testing.T) {
		inv := testInvoiceStandard(t)
		inv.Customer.Inboxes = nil
		inv.Customer.TaxID = &tax.Identity{Country: "DK"}
		require.NoError(t, inv.Calculate())
		err := rules.Validate(inv)
		assert.ErrorContains(t, err, "F-INV044")
	})

	t.Run("missing customer people (F-INV046)", func(t *testing.T) {
		inv := testInvoiceStandard(t)
		inv.Customer.People = nil
		require.NoError(t, inv.Calculate())
		err := rules.Validate(inv)
		assert.ErrorContains(t, err, "F-INV046")
	})

	t.Run("customer with two people is allowed (loose vs F-INV046)", func(t *testing.T) {
		inv := testInvoiceStandard(t)
		inv.Customer.People = append(inv.Customer.People,
			&org.Person{Name: &org.Name{Given: "Mette", Surname: "Hansen"}},
		)
		require.NoError(t, inv.Calculate())
		assert.NoError(t, rules.Validate(inv))
	})

	t.Run("customer contact without an identity fails (F-INV051)", func(t *testing.T) {
		inv := testInvoiceStandard(t)
		inv.Customer.People = []*org.Person{
			{Name: &org.Name{Given: "Mette", Surname: "Sørensen"}},
		}
		require.NoError(t, inv.Calculate())
		assert.ErrorContains(t, rules.Validate(inv), "F-INV051")
	})

	t.Run("customer with no official company ID is still accepted", func(t *testing.T) {
		// Only the seller has to prove who they are; the buyer doesn't.
		inv := testInvoiceStandard(t)
		inv.Customer.TaxID = &tax.Identity{Country: "DE", Code: "282741168"}
		require.NoError(t, inv.Calculate())
		assert.NoError(t, rules.Validate(inv))
	})

	t.Run("foreign customer with a legal identity passes", func(t *testing.T) {
		inv := testInvoiceStandard(t)
		inv.Customer.TaxID = &tax.Identity{Country: "DE", Code: "282741168"}
		inv.Customer.Identities = []*org.Identity{{Scope: org.IdentityScopeLegal, Code: "HRB12345"}}
		require.NoError(t, inv.Calculate())
		assert.NoError(t, rules.Validate(inv))
	})

	t.Run("customer's blank company ID is rejected", func(t *testing.T) {
		inv := testInvoiceStandard(t)
		inv.Customer.TaxID = &tax.Identity{Country: "DE", Code: "282741168"}
		inv.Customer.Identities = []*org.Identity{{Scope: org.IdentityScopeLegal, Code: ""}}
		require.NoError(t, inv.Calculate())
		assert.ErrorContains(t, rules.Validate(inv), "identity code must be provided")
	})

	t.Run("Danish customer needs no explicit legal identity (CVR fabricated)", func(t *testing.T) {
		inv := testInvoiceStandard(t)
		require.NoError(t, inv.Calculate())
		assert.NoError(t, rules.Validate(inv))
	})

	t.Run("credit-note supplier without participant or tax ID code fails (F-CRN028)", func(t *testing.T) {
		cn := testCreditNoteStandard(t)
		cn.Supplier.Inboxes = nil
		cn.Supplier.TaxID = &tax.Identity{Country: "DK"}
		require.NoError(t, cn.Calculate())
		err := rules.Validate(cn)
		assert.ErrorContains(t, err, "F-CRN028")
	})
}

func TestNormalizePartyParticipant(t *testing.T) {
	t.Run("DK party without participant derives the CVR endpoint", func(t *testing.T) {
		inv := testInvoiceStandard(t)
		inv.Supplier.Inboxes = nil
		inv.Supplier.Endpoints = nil
		inv.Customer.Inboxes = nil
		inv.Customer.Endpoints = nil
		inv.Payment = bankPayment()
		require.NoError(t, inv.Calculate())
		require.Len(t, inv.Supplier.Endpoints, 1)
		assert.Equal(t, "DK:CVR:12345674", inv.Supplier.Endpoints[0].URI.String())
		assert.Empty(t, inv.Supplier.Inboxes, "no Peppol endpoint URI is fabricated; the deprecated inbox is not used")
		require.NoError(t, rules.Validate(inv), "a bare DK party should validate via the derived participant")
	})

	t.Run("an explicit inbox is migrated to an endpoint", func(t *testing.T) {
		inv := testInvoiceStandard(t)
		inv.Supplier.Endpoints = nil
		inv.Supplier.Inboxes = []*org.Inbox{{Scheme: "DK:SE", Code: "12345678"}}
		require.NoError(t, inv.Calculate())
		assert.Empty(t, inv.Supplier.Inboxes, "the deprecated inbox is migrated away")
		require.Len(t, inv.Supplier.Endpoints, 1, "the inbox becomes the participant endpoint")
		assert.Equal(t, "DK:SE:12345678", inv.Supplier.Endpoints[0].URI.String(),
			"an explicit DK:SE participant wins over the derived CVR")
	})

	t.Run("foreign party is left untouched", func(t *testing.T) {
		inv := testInvoiceStandard(t)
		inv.Customer.TaxID = &tax.Identity{Country: "DE", Code: "129273398"}
		inv.Customer.Inboxes = nil
		inv.Customer.Endpoints = nil
		require.NoError(t, inv.Calculate())
		assert.Empty(t, inv.Customer.Endpoints, "no participant can be derived for non-DK parties")
	})
}
