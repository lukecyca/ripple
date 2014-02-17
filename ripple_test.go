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
		t.Errorf("Expected DestinationTag 0, got %+v", r.DestinationTag)
	}
}

func TestHandleStreamMessages(t *testing.T) {
	c := Connection{Ledgers: make(chan *Ledger, 1)}

	// Handle ledgerClosed
	m := Message{
		Type:        "ledgerClosed",
		LedgerIndex: 1234567,
		TxnCount:    2,
		LedgerTime:  12345678,
		LedgerHash:  "DEADBEEF",
	}
	c.handleMessage(&m)

	if c.currentLedger.Hash != m.LedgerHash {
		t.Errorf("Ledger hash not parsed")
	}
	if c.currentLedgerIndex != m.LedgerIndex {
		t.Errorf("Ledger index not parsed")
	}
	if c.currentLedgerTxnsLeft != 2 {
		t.Errorf("Ledger txn count not parsed")
	}

	// Handle transaction
	m = Message{
		Type:        "transaction",
		LedgerIndex: 1234567,
		Transaction: &Transaction{},
	}
	c.handleMessage(&m)

	if c.currentLedgerTxnsLeft != 1 {
		t.Errorf("currentLedgerTxnsLeft was not decremented")
	}
	if len(c.currentLedger.Transactions) != 1 {
		t.Errorf("Expected 1 transaction but found %d", len(c.currentLedger.Transactions))
	}

	// Handle another transaction to finish the ledger
	c.handleMessage(&m)
	select {
	case ledger := <-c.Ledgers:
		if ledger.Hash != "DEADBEEF" {
			t.Error("Incorrect ledger returned")
		}
	default:
		t.Errorf("Ledger was not sent to channel")
	}
}

func TestHandleServerStatus(t *testing.T) {
	c := Connection{}
	m := Message{Type: "serverStatus", ServerStatus: "syncing"}
	c.handleMessage(&m)
}

func TestHandleServerInfo(t *testing.T) {
	c := Connection{}
	m := Message{Type: "response", Id: 3,
		Result: &Result{Info: &Info{
			HostID:       "FRED",
			BuildVersion: "1.0",
		}}}
	c.handleMessage(&m)
}

func TestHandleLedgerResponse(t *testing.T) {
	c := Connection{Ledgers: make(chan *Ledger, 1)}

	m := Message{
		Id:     2,
		Type:   "response",
		Status: "success",
		Result: &Result{
			Ledger: &Ledger{Hash: "8BADF00D"},
		},
	}
	c.handleMessage(&m)

	select {
	case ledger := <-c.Ledgers:
		if ledger.Hash != "8BADF00D" {
			t.Error("Incorrect ledger returned")
		}
	default:
		t.Errorf("Ledger was not sent to channel")
	}
}

func TestHandleUnknownMessageType(t *testing.T) {
	c := Connection{}
	m := Message{Type: "foo", Status: "bar"}
	c.handleMessage(&m)

	select {
	case <-c.t.Dying():
		c.t.Done()
		err := c.t.Wait()
		if err.Error() != "Unknown response: type=foo id=0, status=bar, error=" {
			t.Errorf("Killed with incorrect error: %s", err.Error())
		}

	default:
		t.Errorf("Ledger was not killed")
	}
}
