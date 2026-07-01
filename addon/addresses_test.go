package addon_test

import (
	"testing"

	oioubl "github.com/invopop/gobl.dk.oioubl/addon"
	"github.com/invopop/gobl/org"
	"github.com/invopop/gobl/rules"
	"github.com/invopop/gobl/tax"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAddressFormatValidation(t *testing.T) {
	// formatFail is the stable part of the address-format completeness message.
	const formatFail = "declared OIOUBL address format"

	t.Run("no format declared passes (defaults to StructuredLax)", func(t *testing.T) {
		inv := testInvoiceStandard(t)
		require.NoError(t, inv.Calculate())
		assert.NoError(t, rules.Validate(inv))
	})

	t.Run("invalid format value is rejected (F-LIB027)", func(t *testing.T) {
		inv := testInvoiceStandard(t)
		inv.Supplier.Ext = tax.Extensions{}.Set(oioubl.ExtKeyAddressFormat, "Bogus")
		require.NoError(t, inv.Calculate())
		assert.ErrorContains(t, rules.Validate(inv), "F-LIB027")
	})

	t.Run("StructuredDK with a complete address passes", func(t *testing.T) {
		// The standard supplier address has a postal code, street and number.
		inv := testInvoiceStandard(t)
		inv.Supplier.Ext = tax.Extensions{}.Set(oioubl.ExtKeyAddressFormat, oioubl.ExtValueAddressFormatStructuredDK)
		require.NoError(t, inv.Calculate())
		assert.NoError(t, rules.Validate(inv))
	})

	t.Run("StructuredDK without a postal code fails (F-LIB033)", func(t *testing.T) {
		inv := testInvoiceStandard(t)
		inv.Supplier.Ext = tax.Extensions{}.Set(oioubl.ExtKeyAddressFormat, oioubl.ExtValueAddressFormatStructuredDK)
		inv.Supplier.Addresses[0].Code = ""
		require.NoError(t, inv.Calculate())
		assert.ErrorContains(t, rules.Validate(inv), formatFail)
	})

	t.Run("StructuredDK without street or post box fails (F-LIB034)", func(t *testing.T) {
		inv := testInvoiceStandard(t)
		inv.Supplier.Ext = tax.Extensions{}.Set(oioubl.ExtKeyAddressFormat, oioubl.ExtValueAddressFormatStructuredDK)
		inv.Supplier.Addresses[0].Street = ""
		inv.Supplier.Addresses[0].PostOfficeBox = ""
		require.NoError(t, inv.Calculate())
		assert.ErrorContains(t, rules.Validate(inv), formatFail)
	})

	t.Run("StructuredDK with a post box instead of street and number passes", func(t *testing.T) {
		inv := testInvoiceStandard(t)
		inv.Supplier.Ext = tax.Extensions{}.Set(oioubl.ExtKeyAddressFormat, oioubl.ExtValueAddressFormatStructuredDK)
		inv.Supplier.Addresses[0] = &org.Address{
			PostOfficeBox: "120", Code: "1000", Locality: "København", Country: "DK",
		}
		require.NoError(t, inv.Calculate())
		assert.NoError(t, rules.Validate(inv))
	})

	t.Run("Unstructured with a street passes", func(t *testing.T) {
		inv := testInvoiceStandard(t)
		inv.Supplier.Ext = tax.Extensions{}.Set(oioubl.ExtKeyAddressFormat, oioubl.ExtValueAddressFormatUnstructured)
		require.NoError(t, inv.Calculate())
		assert.NoError(t, rules.Validate(inv))
	})

	t.Run("Unstructured with no renderable content fails (F-LIB031)", func(t *testing.T) {
		inv := testInvoiceStandard(t)
		inv.Supplier.Ext = tax.Extensions{}.Set(oioubl.ExtKeyAddressFormat, oioubl.ExtValueAddressFormatUnstructured)
		inv.Supplier.Addresses[0] = &org.Address{Locality: "København", Code: "1000", Country: "DK"}
		require.NoError(t, inv.Calculate())
		assert.ErrorContains(t, rules.Validate(inv), formatFail)
	})

	t.Run("StructuredID with an identifier (number) passes", func(t *testing.T) {
		inv := testInvoiceStandard(t)
		inv.Supplier.Ext = tax.Extensions{}.
			Set(oioubl.ExtKeyAddressFormat, oioubl.ExtValueAddressFormatStructuredID)
		// The register GLN rides org.Address.Number; en16931 still requires the
		// address country (BR-9).
		inv.Supplier.Addresses[0] = &org.Address{Number: "5790000000001", Country: "DK"}
		require.NoError(t, inv.Calculate())
		assert.NoError(t, rules.Validate(inv))
	})

	t.Run("StructuredID without an identifier fails (F-LIB037)", func(t *testing.T) {
		inv := testInvoiceStandard(t)
		inv.Supplier.Ext = tax.Extensions{}.Set(oioubl.ExtKeyAddressFormat, oioubl.ExtValueAddressFormatStructuredID)
		inv.Supplier.Addresses[0] = &org.Address{Country: "DK"}
		require.NoError(t, inv.Calculate())
		assert.ErrorContains(t, rules.Validate(inv), formatFail)
	})

	t.Run("StructuredRegion with a region passes", func(t *testing.T) {
		inv := testInvoiceStandard(t)
		inv.Supplier.Ext = tax.Extensions{}.Set(oioubl.ExtKeyAddressFormat, oioubl.ExtValueAddressFormatStructuredRegion)
		inv.Supplier.Addresses[0] = &org.Address{Region: "Hovedstaden", Country: "DK"}
		require.NoError(t, inv.Calculate())
		assert.NoError(t, rules.Validate(inv))
	})

	t.Run("StructuredRegion with a district (locality) passes", func(t *testing.T) {
		inv := testInvoiceStandard(t)
		inv.Supplier.Ext = tax.Extensions{}.
			Set(oioubl.ExtKeyAddressFormat, oioubl.ExtValueAddressFormatStructuredRegion)
		// org.Address.Locality is the district OIOUBL emits as cbc:District; en16931
		// still requires the address country (BR-9).
		inv.Supplier.Addresses[0] = &org.Address{Locality: "Nørrebro", Country: "DK"}
		require.NoError(t, inv.Calculate())
		assert.NoError(t, rules.Validate(inv))
	})

	t.Run("StructuredRegion with nothing fails (F-LIB039)", func(t *testing.T) {
		inv := testInvoiceStandard(t)
		inv.Supplier.Ext = tax.Extensions{}.Set(oioubl.ExtKeyAddressFormat, oioubl.ExtValueAddressFormatStructuredRegion)
		inv.Supplier.Addresses[0] = &org.Address{}
		require.NoError(t, inv.Calculate())
		assert.ErrorContains(t, rules.Validate(inv), formatFail)
	})
}
