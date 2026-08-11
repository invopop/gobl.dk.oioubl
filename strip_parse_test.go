package oioubl_test

import (
	"strings"
	"testing"

	oioubl "github.com/invopop/gobl.dk.oioubl"
	"github.com/invopop/gobl/cbc"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConvertRejectsNonOIOUBL(t *testing.T) {
	doc := strings.Replace(bareInvoice(t), "OIOUBL-2.1", "urn:cen.eu:en16931:2017", 1)
	in, err := oioubl.ParseInvoice([]byte(doc))
	require.NoError(t, err)
	_, err = in.Convert()
	require.Error(t, err, "a plain EN 16931 invoice is not an OIOUBL document")
	assert.Contains(t, err.Error(), "customization id")
}

func TestParseAllowanceMultiplier(t *testing.T) {
	// OIOUBL writes the factor (F-LIB228), GOBL wants the percentage.
	allowance := `<cac:AllowanceCharge>
    <cbc:ChargeIndicator>false</cbc:ChargeIndicator>
    <cbc:AllowanceChargeReason>Rabat</cbc:AllowanceChargeReason>
    <cbc:MultiplierFactorNumeric>0.25</cbc:MultiplierFactorNumeric>
    <cbc:Amount currencyID="DKK">100.00</cbc:Amount>
    <cbc:BaseAmount currencyID="DKK">400.00</cbc:BaseAmount>
    <cac:TaxCategory>
      <cbc:ID schemeAgencyID="320" schemeID="urn:oioubl:id:taxcategoryid-1.1">StandardRated</cbc:ID>
      <cbc:Percent>25</cbc:Percent>
      <cac:TaxScheme>
        <cbc:ID schemeAgencyID="320" schemeID="urn:oioubl:id:taxschemeid-1.1">63</cbc:ID>
        <cbc:Name>Moms</cbc:Name>
      </cac:TaxScheme>
    </cac:TaxCategory>
  </cac:AllowanceCharge>
  <cac:TaxTotal>`
	doc := strings.Replace(bareInvoice(t), "<cac:TaxTotal>", allowance, 1)

	inv, err := convertString(t, doc)
	require.NoError(t, err)
	require.Len(t, inv.Discounts, 1)
	require.NotNil(t, inv.Discounts[0].Percent)
	assert.Equal(t, "25%", inv.Discounts[0].Percent.String(), "0.25 on the wire is 25 percent")
}

func TestParseGiroPaymentReference(t *testing.T) {
	// Giro puts the payment reference in InstructionID; PaymentID is a routing
	// code, which is what the generic parser would otherwise pick up.
	means := `<cbc:PaymentChannelCode listID="urn:oioubl:codelist:paymentchannelcode-1.1">DK:GIRO</cbc:PaymentChannelCode>
    <cbc:InstructionID>1234567890123456</cbc:InstructionID>
    <cbc:PaymentID>73</cbc:PaymentID>
    <cac:CreditAccount>
      <cbc:AccountID>87654321</cbc:AccountID>
    </cac:CreditAccount>`
	doc := bareInvoice(t)
	doc = strings.Replace(doc,
		`<cbc:PaymentChannelCode listID="urn:oioubl:codelist:paymentchannelcode-1.1">IBAN</cbc:PaymentChannelCode>`,
		means, 1)
	doc = strings.Replace(doc, "<cbc:PaymentID>SAMPLE-001</cbc:PaymentID>", "", 1)

	inv, err := convertString(t, doc)
	require.NoError(t, err)
	require.NotNil(t, inv.Payment)
	instr := inv.Payment.Instructions
	require.NotNil(t, instr)
	assert.Equal(t, cbc.Code("1234567890123456"), instr.Ref, "the reference is InstructionID, not PaymentID")
	require.Len(t, instr.CreditTransfer, 1)
	assert.Equal(t, cbc.Code("87654321"), instr.CreditTransfer[0].Number)
}
