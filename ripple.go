package ripple

import (
	"code.google.com/p/go.net/websocket"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
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

func (t *Transaction) String() string {
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

type Ledger struct {
	Accepted     bool
	CloseTime    uint64 `json:"close_time"`
	Closed       bool
	Hash         string
	Index        string `json:"ledger_index"`
	ParentHash   string
	Transactions []*Transaction
}

type Result struct {
	Ledger *Ledger
}

type Message struct {
	Id           int
	Type         string
	Result       *Result
	Status       string
	Error        string
	LedgerHash   string `json:"ledger_hash"`
	LedgerIndex  uint64 `json:"ledger_index"`
	LedgerTime   uint64 `json:"ledger_time"`
	Transaction  *Transaction
	TxnCount     int    `json:"txn_count"`
	ServerStatus string `json:"server_status"`
}

type Connection struct {
	Ledgers chan *Ledger

	// Semaphore for number of outstanding ledger getters
	pastLedgerGetters chan int

	conn *websocket.Conn
}

func Connect(uri string) (c *Connection, err error) {
	c = &Connection{
		Ledgers: make(chan *Ledger),
	}

	// Connect to websocket server
	c.conn, err = websocket.Dial(uri, "", "http://localhost")
	if err != nil {
		return
	}

	// Subscribe to all transactions, ledgers, and server messages
	msg := "{\"command\":\"subscribe\",\"id\":1,\"streams\":[\"server\", \"ledger\", \"transactions\"]}"
	err = websocket.Message.Send(c.conn, msg)
	if err != nil {
		return
	}

	// Wait for ack
	var m Message
	err = websocket.JSON.Receive(c.conn, &m)
	if err != nil {
		return
	}

	if m.Type != "response" || m.Status != "success" {
		err = errors.New("Failed to subscribe")
		return
	}

	return
}

func (c *Connection) GetLedger(idx uint64) {
	// Requests a single ledger from the server

	msg := fmt.Sprintf("{\"command\":\"ledger\",\"id\":2,\"ledger_index\":%d,\"transactions\":1,\"expand\":1}", idx)
	err := websocket.Message.Send(c.conn, msg)
	if err != nil {
		panic("Could not get ledger: " + err.Error())
	}
}

func (c *Connection) Monitor() {
	var currentLedgerIndex uint64
	var currentLedgerTxnsLeft int
	var currentLedger *Ledger

	for {
		var err error
		var m Message
		err = websocket.JSON.Receive(c.conn, &m)
		if err != nil {
			if err == io.EOF {
				// graceful shutdown by server
				break
			}
			fmt.Println("Could not receive: " + err.Error())
			break
		}

		switch m.Type {
		case "ledgerClosed":
			if currentLedgerTxnsLeft != 0 {
				log.Panicf("Missing %d transactions from ledger %d\n", currentLedgerTxnsLeft, currentLedgerIndex)
			}

			currentLedgerIndex = m.LedgerIndex
			currentLedgerTxnsLeft = m.TxnCount
			currentLedger = &Ledger{
				CloseTime: m.LedgerTime,
				Closed:    true,
				Hash:      m.LedgerHash,
				Index:     strconv.FormatUint(m.LedgerIndex, 10),
			}

		case "transaction":
			if m.LedgerIndex != currentLedgerIndex {
				log.Panicf("Received out-of-order transaction from ledger %d\n", m.LedgerIndex)
			}
			if currentLedgerTxnsLeft <= 0 {
				log.Printf("Error: Received extra txn: %s", m.Transaction.Hash)
			}

			currentLedger.Transactions = append(currentLedger.Transactions, m.Transaction)

			currentLedgerTxnsLeft--
			if currentLedgerTxnsLeft == 0 {
				c.Ledgers <- currentLedger
			}

		case "serverStatus":
			log.Printf("Server Status: %s\n", m.ServerStatus)

		case "response":
			if m.Id == 2 && m.Status == "success" {
				c.Ledgers <- m.Result.Ledger
			} else {
				log.Printf("Error: Unknown response ID: %d, status: %s\n", m.Id, m.Status)
			}

		case "error":
			log.Printf("Error: %s\n", m.Error)

		default:
			log.Printf("Error: Unknown message type: %s\n", m.Type)
		}
	}
}
