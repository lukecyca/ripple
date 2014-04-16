package ripple

import (
	"encoding/json"
	"fmt"
	"math/big"
	"strconv"
	"time"
)

type Time struct {
	time.Time
}

func (t *Time) UnmarshalJSON(b []byte) (err error) {

	s, err := strconv.ParseUint(string(b), 10, 64)
	if err != nil {
		return
	}

	t.Time = time.Unix(int64(s)+946684800, 0)

	return
}

type Amount struct {
	Currency string
	Issuer   string
	Value    big.Rat
}

func (a *Amount) UnmarshalJSON(b []byte) (err error) {

	// Try interpret as IOU
	var m map[string]string
	err = json.Unmarshal(b, &m)
	if err == nil {
		a.Currency = m["currency"]
		_, success := a.Value.SetString(m["value"])
		if !success {
			return fmt.Errorf("Could not interpret value: %s", m["value"])
		}
		a.Issuer = m["issuer"]
		return
	}

	// Try interpret as XRP in drips
	var s string
	err = json.Unmarshal(b, &s)
	if err == nil {
		dripValue, success := a.Value.SetString(s)
		if !success {
			return fmt.Errorf("Could not interpret value: %s", s)
		}
		a.Value.Quo(dripValue, big.NewRat(1000000, 1))
		a.Currency = "XRP"
		return
	}

	return fmt.Errorf("Could not unmarshal amount: %s", b)
}

type Meta struct {
	TransactionResult string
}

type Transaction struct {
	Account         string
	Amount          Amount
	Date            Time
	Destination     string
	DestinationTag  uint32
	Fee             string
	Flags           uint32
	Hash            string
	SendMax         *Amount
	Sequence        uint32
	SingingPubKey   string
	TransactionType string
	TxnSignature    string

	Meta *Meta `json:"metaData"`
}

type Ledger struct {
	Accepted     bool
	CloseTime    Time `json:"close_time"`
	Closed       bool
	Hash         string
	Index        string `json:"ledger_index"`
	ParentHash   string
	Transactions []*Transaction
}

type Info struct {
	BuildVersion string `json:"build_version"`
	HostID       string
	Peers        int
}

type Result struct {
	Ledger *Ledger
	Info   *Info
}

type Message struct {
	Id           int
	Type         string
	Result       *Result
	Status       string
	Error        string
	LedgerHash   string `json:"ledger_hash"`
	LedgerIndex  uint64 `json:"ledger_index"`
	LedgerTime   Time   `json:"ledger_time"`
	Transaction  *Transaction
	TxnCount     int    `json:"txn_count"`
	ServerStatus string `json:"server_status"`
	Validated    bool
	Meta         *Meta `json:"meta"`
}
