package ripple

import (
	"encoding/json"
	"testing"
)

func TestAmountUnmarshallXRP(t *testing.T) {
	var raw = []byte(`"200000000"`)
	var a Amount

	err := json.Unmarshal(raw, &a)
	if err != nil {
		t.Error(err)
	}

	if a.Currency != "XRP" || a.Value.FloatString(6) != "200.000000" {
		t.Errorf("Expected 200 XRP, got %+v", a)
	}
}

func TestAmountUnmarshallIOU(t *testing.T) {
	var raw = []byte(`{
		"currency": "BTC",
		"issuer": "r000000000000000000000000000000000",
		"value": "0.0001"
    }`)
	var a Amount

	err := json.Unmarshal(raw, &a)
	if err != nil {
		t.Error(err)
	}

	if a.Currency != "BTC" || a.Value.FloatString(6) != "0.000100" {
		t.Errorf("Expected 0.0001 BTC, got %+v", a)
	}

	if a.Issuer != "r000000000000000000000000000000000" {
		t.Errorf("Expected issuer r000000000000000000000000000000000, got %+v", a.Issuer)
	}
}

func TestTransactionUnmarshall(t *testing.T) {
	var raw = []byte(`{
		"Account": "r00000000000000000000000000000000",
		"Amount": {
			"currency": "ICE",
			"issuer": "r4H3F9dDaYPFwbrUsusvNAHLz2sEZk4wE5",
			"value": "1"
		},
		"Destination": "r111111111111111111111111111111111",
		"Fee": "15",
		"Flags": 0,
		"SendMax": {
			"currency": "ICE",
			"issuer": "r4H3F9dDaYPFwbrUsusvNAHLz2sEZk4wE5",
			"value": "1.0015"
		},
		"Sequence": 42584,
		"SigningPubKey": "03F3B30EF0F9DD92A6DA915B17767CB76FFE968A0D708131996BADA0C74E61DE48",
		"TransactionType": "Payment",
		"TxnSignature": "30450221008886E009E4E8A05CEF8E001380994A8FB6E28A3138E4AE593D7A3083964C52A9022057823FB88BF24E57B538BA3B78FBB3FE837F17C3F64E9B1E1A203C2A4D6AA75F",
		"date": 444683680,
		"hash": "4655982C1FD13073F8B4715B6D1485030F4BB3DA9201EF1DBE4D1834764A09BF"
	}`)
	var r Transaction

	err := json.Unmarshal(raw, &r)
	if err != nil {
		t.Error(err)
	}

	if r.Account != "r00000000000000000000000000000000" {
		t.Errorf("Expected Account r000000000000000000000000000000000, got %+v", r.Account)
	}

	if r.Destination != "r111111111111111111111111111111111" {
		t.Errorf("Expected Destination r111111111111111111111111111111111, got %+v", r.Destination)
	}

	if r.DestinationTag != 0 {
		t.Errorf("Expected DestinationTag 0, got %d", r.DestinationTag)
	}

	if r.Date.T.Unix() != 1391368480 {
		t.Errorf("Expected Date 1391368480, got %d", r.Date.T.Unix())
	}
}
