package oioubl_test

import (
	"testing"

	"github.com/invopop/gobl.dk.oioubl"
	"github.com/invopop/gobl/bill"
	"github.com/invopop/gobl/org"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConvertCustomerContactID(t *testing.T) {
	t.Run("a person's identity code comes first", func(t *testing.T) {
		env := envelopeWithCustomerEndpoint(t, "iso6523-actorid-upis::0184:88146328")
		inv := env.Extract().(*bill.Invoice)
		inv.Customer.People = []*org.Person{{
			Name:       &org.Name{Given: "Hans Hansen"},
			Identities: []*org.Identity{{Code: "7778"}},
		}}
		require.NoError(t, env.Calculate())

		doc, err := oioubl.ConvertInvoice(env)
		require.NoError(t, err)

		c := doc.AccountingCustomerParty.Party.Contact
		require.NotNil(t, c)
		assert.Equal(t, "7778", *c.ID)
	})

	t.Run("the email stands in when no person carries a code", func(t *testing.T) {
		env := envelopeWithCustomerEndpoint(t, "iso6523-actorid-upis::0184:88146328")

		doc, err := oioubl.ConvertInvoice(env)
		require.NoError(t, err)

		c := doc.AccountingCustomerParty.Party.Contact
		require.NotNil(t, c)
		assert.Equal(t, "bogholderi@kunde.dk", *c.ID)
		assert.Equal(t, "bogholderi@kunde.dk", *c.ElectronicMail)
	})
}
