package ripple

import (
	"code.google.com/p/go.net/websocket"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/big"
	"strconv"
)

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

type Transaction struct {
	Account         string
	Amount          Amount
	Date            int
	Destination     string
	DestinationTag  int
	Fee             string
	Flags           int
	Hash            string
	SendMax         *Amount
	Sequence        int
	SingingPubKey   string
	TransactionType string
	TxnSignature    string
}

type Message struct {
	LedgerHash  string `json:"ledger_hash"`
	LedgerIndex int    `json:"ledger_index"`
	Status      string
	Transaction *Transaction
	Type        string
}

func (m *Message) String() string {
	if m.Type == "transaction" {
		t := m.Transaction

		if t.TransactionType == "Payment" {
			return ("Payment " + t.Hash +
				" from " + t.Account + " to " +
				t.Destination + " " +
				strconv.Itoa(t.DestinationTag) + " " +
				t.Amount.Value.FloatString(5) + " " +
				t.Amount.Currency)
		}

		return t.TransactionType + " " + t.Hash
	}

	return m.Type
}

type Connection struct {
	Messages chan *Message

	conn *websocket.Conn
}

func (r *Connection) Connect(uri string) (err error) {

	// Connect to websocket server
	r.conn, err = websocket.Dial(uri, "", "http://localhost")
	if err != nil {
		return err
	}

	// Subscribe to all transactions
	msg := "{\"command\":\"subscribe\",\"id\":1,\"streams\":[\"transactions\"]}"
	err = websocket.Message.Send(r.conn, msg)
	if err != nil {
		return err
	}

	// Wait for ack
	var m Message
	err = websocket.JSON.Receive(r.conn, &m)
	if err != nil {
		return err
	}

	if m.Type != "response" || m.Status != "success" {
		return errors.New("Failed to subscribe")
	}

	return
}

func (r *Connection) Monitor() {
	for {
		var err error
		var m Message
		err = websocket.JSON.Receive(r.conn, &m)
		if err != nil {
			if err == io.EOF {
				// graceful shutdown by server
				break
			}
			fmt.Println("Could not receive: " + err.Error())
			break
		}
		r.Messages <- &m
	}
}
