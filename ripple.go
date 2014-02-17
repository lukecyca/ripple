package ripple

import (
	"code.google.com/p/go.net/websocket"
	"encoding/json"
	"fmt"
	"launchpad.net/tomb"
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

type Ledger struct {
	Accepted     bool
	CloseTime    uint64 `json:"close_time"`
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
	LedgerTime   uint64 `json:"ledger_time"`
	Transaction  *Transaction
	TxnCount     int    `json:"txn_count"`
	ServerStatus string `json:"server_status"`
}

type Connection struct {
	Ledgers               chan *Ledger
	conn                  *websocket.Conn
	t                     tomb.Tomb
	currentLedgerIndex    uint64
	currentLedgerTxnsLeft int
	currentLedger         *Ledger
}

func NewConnection(uri string) (c *Connection, err error) {
	c = &Connection{
		Ledgers: make(chan *Ledger),
	}

	// Connect to websocket server
	c.conn, err = websocket.Dial(uri, "", "http://localhost")
	if err != nil {
		return
	}

	c.conn.SetDeadline(time.Now().Add(Timeout))

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
		err = fmt.Errorf("Failed to subscribe: %s", m.Error)
		return
	}

	// Get server info
	msg = "{\"command\":\"server_info\",\"id\":3}"
	err = websocket.Message.Send(c.conn, msg)
	if err != nil {
		return
	}

	go c.loop()
	return
}

func (c *Connection) GetLedger(idx uint64) (err error) {
	// Requests a single ledger from the server

	c.conn.SetDeadline(time.Now().Add(Timeout))
	msg := fmt.Sprintf("{\"command\":\"ledger\",\"id\":2,\"ledger_index\":%d,\"transactions\":1,\"expand\":1}", idx)
	err = websocket.Message.Send(c.conn, msg)
	return
}

func (c *Connection) loop() {
	defer c.t.Done()
	defer close(c.Ledgers)
	defer c.conn.Close()

	for {
		var err error
		var m Message
		c.conn.SetDeadline(time.Now().Add(Timeout))
		err = websocket.JSON.Receive(c.conn, &m)
		if err != nil {
			c.t.Kill(err)
		}
		c.handleMessage(&m)

		// If the tomb is marked dying, exit cleanly
		select {
		case <-c.t.Dying():
			return
		default:
			//pass
		}
	}
}

func (c *Connection) handleMessage(m *Message) {
	switch {
	case m.Type == "ledgerClosed":
		c.currentLedgerIndex = m.LedgerIndex
		c.currentLedgerTxnsLeft = m.TxnCount
		c.currentLedger = &Ledger{
			CloseTime: m.LedgerTime,
			Closed:    true,
			Hash:      m.LedgerHash,
			Index:     strconv.FormatUint(m.LedgerIndex, 10),
		}

	case m.Type == "transaction":
		if m.LedgerIndex == c.currentLedgerIndex {
			c.currentLedger.Transactions = append(c.currentLedger.Transactions, m.Transaction)

			c.currentLedgerTxnsLeft--
			if c.currentLedgerTxnsLeft == 0 {
				c.Ledgers <- c.currentLedger
			}
		}

	case m.Type == "serverStatus":
		log.Printf("Rippled Server Status: %s", m.ServerStatus)

	case m.Type == "response" && m.Id == 2 && m.Status == "success":
		c.Ledgers <- m.Result.Ledger

	case m.Type == "response" && m.Id == 3 && m.Status == "success":
		log.Printf(
			"Connected to hostid %s (%s)",
			m.Result.Info.HostID,
			m.Result.Info.BuildVersion,
		)

	default:
		c.t.Kill(fmt.Errorf(
			"Unknown response: type=%s id=%d, status=%s, error=%s",
			m.Type,
			m.Id,
			m.Status,
			m.Error,
		))

	}
}
