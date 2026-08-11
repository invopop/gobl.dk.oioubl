package oioubl_test

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"testing"
	"time"

	oioubl "github.com/invopop/gobl.dk.oioubl"
	"github.com/invopop/gobl/bill"
	"github.com/invopop/phive"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

const phiveAddr = "127.0.0.1:9090"

var validate = flag.Bool("validate", false, "validate converted documents against the OIOUBL schematron via phive")

// TestSchematron converts every fixture and shipped example and validates each
// against the real OIOUBL schematron, which is the only definition of
// correctness the format has: unit tests and golden files only pin what the
// converter already does.
//
// Off by default, since it needs a phive service. Run it with -validate and
// phive listening on phiveAddr; CI does that with a service container.
func TestSchematron(t *testing.T) {
	if !*validate {
		t.Skip("schematron validation is off; run with -validate and phive on " + phiveAddr)
	}

	conn, err := grpc.NewClient(phiveAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	require.NoError(t, err)
	defer conn.Close() //nolint:errcheck
	client := phive.NewValidationServiceClient(conn)
	waitForRules(t, client)

	for _, example := range convertCases(t) {
		t.Run(example.src, func(t *testing.T) {
			env := loadTestEnvelope(t, example.src)
			inv, ok := env.Extract().(*bill.Invoice)
			require.True(t, ok, "example should hold an invoice")

			doc, err := oioubl.ConvertInvoice(env)
			require.NoError(t, err)
			data, err := oioubl.Bytes(doc)
			require.NoError(t, err)

			vesid := oioubl.GetVESID(inv)
			resp, err := client.ValidateXml(context.Background(), &phive.ValidateXmlRequest{
				Vesid:      vesid,
				XmlContent: data,
			})
			require.NoError(t, err)

			if !resp.Success {
				results, err := json.MarshalIndent(resp.Results, "", "  ")
				require.NoError(t, err)
				t.Fatalf("not valid against %s:\n%s", vesid, results)
			}
		})
	}
}

// waitForRules blocks until phive reports the OIOUBL ruleset loaded. A service
// still starting up accepts the connection and then fails every document, which
// reads exactly like a converter bug.
func waitForRules(t *testing.T, client phive.ValidationServiceClient) {
	t.Helper()

	deadline := time.Now().Add(90 * time.Second)
	for {
		err := fmt.Errorf("ruleset %s not loaded", oioubl.VESIDInvoice)
		resp, rerr := client.ListVesIds(context.Background(), &phive.ListVesIdsRequest{Filter: "oioubl"})
		if rerr != nil {
			err = rerr
		} else {
			for _, v := range resp.Vesids {
				if v.Vesid == oioubl.VESIDInvoice {
					return
				}
			}
		}
		if time.Now().After(deadline) {
			t.Fatalf("phive at %s never became ready: %v", phiveAddr, err)
		}
		time.Sleep(time.Second)
	}
}
