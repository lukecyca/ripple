package ripple

import (
	"testing"
)

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
