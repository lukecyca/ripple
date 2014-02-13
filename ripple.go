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
	"time"
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
	Transactions []Transaction
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
	Transactions chan *Transaction

	conn *websocket.Conn
}

func (r *Connection) Connect(uri string) (err error) {

	// Connect to websocket server
	r.conn, err = websocket.Dial(uri, "", "http://localhost")
	if err != nil {
		return err
	}

	// Subscribe to all transactions
	msg := "{\"command\":\"subscribe\",\"id\":1,\"streams\":[\"server\", \"ledger\", \"transactions\"]}"
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

func (r *Connection) getPastLedgers(startLedger uint64, endLedger uint64) {
	// Asks the server for each ledger between start and end, inclusive

	if startLedger > endLedger {
		panic("Invalid ledger range")
	}
	log.Printf("Getting past ledgers [%d, %d]\n", startLedger, endLedger)

	for i := startLedger; i <= endLedger; i++ {
		msg := fmt.Sprintf("{\"command\":\"ledger\",\"id\":2,\"ledger_index\":%d,\"transactions\":1,\"expand\":1}", i)
		err := websocket.Message.Send(r.conn, msg)
		if err != nil {
			panic("Could not get ledger: " + err.Error())
		}

		time.Sleep(time.Second)
	}

	log.Printf("Completed ledgers [%d, %d]\n", startLedger, endLedger)
}

func (r *Connection) MonitorTransactionsFrom(startLedger uint64) {
	currentLedger := startLedger - 1
	currentLedgerTxnsLeft := 0

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

		switch m.Type {
		case "ledgerClosed":
			if m.LedgerIndex > currentLedger+1 {
				//fmt.Printf("Missing ledgers between %d and %d\n", currentLedger, m.LedgerIndex)
				//panic("")
				go r.getPastLedgers(currentLedger+1, m.LedgerIndex-1)
			}
			if currentLedgerTxnsLeft != 0 {
				log.Printf("Missing %d transactions from ledger %d\n", currentLedgerTxnsLeft, currentLedger)
				panic("")
			}
			fmt.Println(m.LedgerIndex)
			currentLedger = m.LedgerIndex
			currentLedgerTxnsLeft = m.TxnCount

		case "transaction":
			if m.LedgerIndex != currentLedger {
				log.Printf("Received out-of-order transaction from ledger %d\n", m.LedgerIndex)
				panic("")
			}
			currentLedgerTxnsLeft--
			r.Transactions <- m.Transaction

		case "serverStatus":
			log.Printf("SERVER: %s\n", m.ServerStatus)

		case "response":
			if m.Id == 2 && m.Status == "success" {
				fmt.Println(m.Result.Ledger.Index)
				for _, t := range m.Result.Ledger.Transactions {
					r.Transactions <- &t
				}
			} else {
				log.Printf("Unknown response ID: %d, status: %s\n", m.Id, m.Status)
			}

		case "error":
			log.Printf("Error: %s\n", m.Error)

		default:
			log.Printf("Unknown message type: %s\n", m.Type)
		}
	}
}
