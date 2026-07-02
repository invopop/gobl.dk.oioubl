package dkoioubl

import (
	"encoding/xml"
	"strconv"

	oioubl "github.com/invopop/gobl.dk.oioubl/addon"
	"github.com/invopop/gobl/bill"
	"github.com/invopop/gobl/catalogues/untdid"
	"github.com/invopop/gobl/num"
	"github.com/invopop/gobl/org"
	"github.com/invopop/gobl/pay"
)

// NamespaceUBLReminder is the UBL 2.1 Reminder root namespace.
const NamespaceUBLReminder = "urn:oasis:names:specification:ubl:schema:xsd:Reminder-2"

// Reminder is a UBL 2.1 Reminder, the OIOUBL dunning document (Rykker) mapped
// from a bill.Payment of type "request".
type Reminder struct {
	XMLName      xml.Name
	CBCNamespace string `xml:"xmlns:cbc,attr"`
	CACNamespace string `xml:"xmlns:cac,attr"`
	UBLNamespace string `xml:"xmlns,attr"`

	UBLVersionID    string  `xml:"cbc:UBLVersionID,omitempty"`
	CustomizationID string  `xml:"cbc:CustomizationID,omitempty"`
	ProfileID       *IDType `xml:"cbc:ProfileID,omitempty"`
	ID              string  `xml:"cbc:ID"`
	CopyIndicator   string  `xml:"cbc:CopyIndicator,omitempty"`
	UUID            string  `xml:"cbc:UUID,omitempty"`
	IssueDate       string  `xml:"cbc:IssueDate"`
	IssueTime       string  `xml:"cbc:IssueTime,omitempty"`

	ReminderTypeCode        *IDType `xml:"cbc:ReminderTypeCode,omitempty"`
	ReminderSequenceNumeric string  `xml:"cbc:ReminderSequenceNumeric,omitempty"`
	DocumentCurrencyCode    string  `xml:"cbc:DocumentCurrencyCode,omitempty"`

	Note []string `xml:"cbc:Note,omitempty"`

	AccountingSupplierParty SupplierParty  `xml:"cac:AccountingSupplierParty"`
	AccountingCustomerParty CustomerParty  `xml:"cac:AccountingCustomerParty"`
	PayeeParty              *Party         `xml:"cac:PayeeParty,omitempty"`
	PaymentMeans            []PaymentMeans `xml:"cac:PaymentMeans,omitempty"`
	TaxTotal                []TaxTotal     `xml:"cac:TaxTotal,omitempty"`
	LegalMonetaryTotal      MonetaryTotal  `xml:"cac:LegalMonetaryTotal"`
	ReminderLine            []ReminderLine `xml:"cac:ReminderLine"`
}

// ReminderLine restates one outstanding amount and references the document it concerns.
type ReminderLine struct {
	ID               string            `xml:"cbc:ID"`
	DebitLineAmount  Amount            `xml:"cbc:DebitLineAmount"`
	BillingReference *BillingReference `xml:"cac:BillingReference,omitempty"`
}

func newReminder(pmt *bill.Payment) *Reminder {
	currency := pmt.Currency.String()

	out := &Reminder{
		XMLName:                 xml.Name{Local: "Reminder"},
		CBCNamespace:            NamespaceCBC,
		CACNamespace:            NamespaceCAC,
		UBLNamespace:            NamespaceUBLReminder,
		UBLVersionID:            Version,
		CustomizationID:         CustomizationID,
		ID:                      invoiceNumber(pmt.Series, pmt.Code),
		IssueDate:               formatDate(pmt.IssueDate),
		DocumentCurrencyCode:    currency,
		AccountingSupplierParty: SupplierParty{Party: newParty(pmt.Supplier)},
		AccountingCustomerParty: CustomerParty{Party: newParty(pmt.Customer)},
	}
	out.ProfileID = &IDType{Value: ProfileID}
	if !pmt.UUID.IsZero() {
		out.UUID = pmt.UUID.String()
	}
	if pmt.IssueTime != nil {
		out.IssueTime = pmt.IssueTime.String()
	}
	for _, n := range pmt.Notes {
		if n != nil && n.Text != "" {
			out.Note = append(out.Note, n.Text)
		}
	}
	if pmt.Payee != nil {
		out.PayeeParty = newParty(pmt.Payee)
	}

	out.addReminderLines(pmt, currency)
	out.addReminderTotals(pmt, currency)
	out.addReminderPaymentMeans(pmt)

	applyReminder(out, pmt)

	return out
}

// addReminderLines builds one ReminderLine per payment line.
func (rem *Reminder) addReminderLines(pmt *bill.Payment, currency string) {
	for _, l := range pmt.Lines {
		if l == nil {
			continue
		}
		line := ReminderLine{
			ID:              strconv.Itoa(l.Index),
			DebitLineAmount: Amount{Value: l.Amount.String(), CurrencyID: &currency},
		}
		if l.Document != nil {
			line.BillingReference = &BillingReference{
				InvoiceDocumentReference: reminderDocumentReference(l.Document),
			}
		}
		rem.ReminderLine = append(rem.ReminderLine, line)
	}
}

// addReminderTotals builds the LegalMonetaryTotal. A reminder restates
// already-taxed amounts, so it levies no tax of its own: TaxExclusiveAmount
// (OIOUBL reads this as the reminder's own tax, F-REM079) is zero and every
// other total equals the debit-line sum.
func (rem *Reminder) addReminderTotals(pmt *bill.Payment, currency string) {
	exp := pmt.Total.Exp()
	sum := num.MakeAmount(0, exp)
	for _, l := range pmt.Lines {
		if l != nil {
			sum = sum.Add(l.Amount)
		}
	}
	zero := num.MakeAmount(0, exp)
	rem.LegalMonetaryTotal = MonetaryTotal{
		LineExtensionAmount: Amount{Value: sum.String(), CurrencyID: &currency},
		TaxExclusiveAmount:  Amount{Value: zero.String(), CurrencyID: &currency},
		TaxInclusiveAmount:  Amount{Value: sum.String(), CurrencyID: &currency},
		PayableAmount:       &Amount{Value: sum.String(), CurrencyID: &currency},
	}
}

// addReminderPaymentMeans emits cac:PaymentMeans for the reminder's payment
// methods: credit transfer (IBAN) and the Danish Giro/FIK channels. Other means
// keys carry no OIOUBL payment channel and are not emitted.
func (rem *Reminder) addReminderPaymentMeans(pmt *bill.Payment) {
	for _, m := range pmt.Methods {
		if m == nil {
			continue
		}
		if pm, ok := reminderPaymentMeans(m); ok {
			rem.PaymentMeans = append(rem.PaymentMeans, pm)
		}
	}
}

// reminderPaymentMeans maps a payment Record to an OIOUBL PaymentMeans, or
// reports false when the means has no OIOUBL channel.
func reminderPaymentMeans(m *pay.Record) (PaymentMeans, bool) {
	code := reminderMeansCode(m)
	if code == "" {
		return PaymentMeans{}, false
	}
	pm := PaymentMeans{PaymentMeansCode: IDType{Value: code}}
	// The channel is derived from the means (Giro/FIK/IBAN); the kortart and
	// payment number follow.
	if ch := paymentChannel(code); ch != "" {
		pm.PaymentChannelCode = &IDType{Value: ch}
	}
	applyRecordPaymentID(&pm, m, code)
	addRecordCreditTransferAccount(&pm, m, code)
	return pm, true
}

// reminderMeansCode resolves the OIOUBL PaymentMeansCode for a record: an
// explicit UNTDID means (Giro 50 / FIK 93) wins, otherwise a credit transfer
// maps to 31. OIOUBL re-codes the 30 bank transfer to 31.
func reminderMeansCode(m *pay.Record) string {
	if code := m.Ext.Get(untdid.ExtKeyPaymentMeans).String(); code != "" {
		if code == "30" {
			return "31"
		}
		return code
	}
	if m.Key.HasPrefix(pay.MeansKeyCreditTransfer) {
		return "31"
	}
	return ""
}

// applyRecordPaymentID sets the Giro (50) / FIK (93) cbc:PaymentID to the
// kortart deduced for the record reference, mirroring the invoice path (see
// kortart); the reference rides cbc:InstructionID for the structured kortarts,
// while the free-text 01/73 carry no payment number.
func applyRecordPaymentID(pm *PaymentMeans, m *pay.Record, code string) {
	if code != "50" && code != "93" {
		return
	}
	k := kortart(code, m.Ref)
	pm.PaymentID = &k
	if m.Ref != "" && k != "01" && k != "73" {
		ref := m.Ref
		pm.InstructionID = &ref
	}
}

// addRecordCreditTransferAccount wires the credit-transfer account onto the
// payment means. For FIK (93) the creditor account lives in
// cac:CreditAccount/cbc:AccountID (F-LIB305) rather than PayeeFinancialAccount.
func addRecordCreditTransferAccount(pm *PaymentMeans, m *pay.Record, code string) {
	if m.CreditTransfer == nil {
		return
	}
	if code == "93" {
		pm.CreditAccount = &CreditAccount{AccountID: m.CreditTransfer.Number}
		return
	}
	pm.PayeeFinancialAccount = newCreditTransferAccount(m.CreditTransfer, code)
}

// reminderDocumentReference maps a paid document to a UBL Reference.
func reminderDocumentReference(doc *org.DocumentRef) *Reference {
	ref := &Reference{
		ID: IDType{Value: invoiceNumber(doc.Series, doc.Code)},
	}
	if !doc.UUID.IsZero() {
		ref.UUID = doc.UUID.String()
	}
	if doc.IssueDate != nil {
		ref.IssueDate = formatDate(*doc.IssueDate)
	}
	return ref
}

// Reminders ride the billing-only Procurement-BilSim-1.0 profile (profileid-1.2),
// NOT the profile5 invoices use: the OIOUBL Reminder belongs to the billing
// process, and the billing-only profile avoids advertising the order documents
// that the OrdSim-BilSim profiles carry.
const reminderProfileID = "Procurement-BilSim-1.0"

// applyReminder stamps the OIOUBL specifics: party formatting, the profileid
// scheme attributes, and the reminder type (F-REM006/061) and sequence
// (F-REM007) from the payment extensions.
func applyReminder(out *Reminder, pmt *bill.Payment) {
	applyParty(out.AccountingSupplierParty.Party)
	applyParty(out.AccountingCustomerParty.Party)
	if out.PayeeParty != nil {
		applyParty(out.PayeeParty)
	}

	for i := range out.PaymentMeans {
		stampPaymentChannel(&out.PaymentMeans[i])
	}

	if out.ProfileID == nil {
		out.ProfileID = &IDType{}
	}
	out.ProfileID.SchemeID = ptr(schemeProfileV12)
	out.ProfileID.SchemeAgencyID = ptr(agencyID)
	out.ProfileID.Value = reminderProfileID

	// remindertypecode-1.1 (F-REM061): the type is a GOBL document-variant tag,
	// not an extension — an untagged reminder is a formal Reminder, the addon's
	// advis tag marks it an advisory notice.
	code := "Reminder"
	if pmt.HasTags(oioubl.TagAdvis) {
		code = "Advis"
	}
	out.ReminderTypeCode = &IDType{
		ListAgencyID: ptr(agencyID),
		ListID:       ptr(listReminderType),
		Value:        code,
	}
	out.ReminderSequenceNumeric = pmt.Ext.Get(oioubl.ExtKeyReminderSequence).String()
}
