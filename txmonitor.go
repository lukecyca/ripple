package ripple

import (
	"errors"
	"launchpad.net/tomb"
	"log"
	"strconv"
	"time"
)

const Timeout = time.Minute

var URIs []string = []string{
	"wss://s_west.ripple.com:443",
	"wss://s_east.ripple.com:443",
	"wss://s1.ripple.com:443",
}

type Monitor struct {
	t       tomb.Tomb
	Ledgers chan *Ledger

	// Next ledger to anticipate
	nextLedgerIdx uint64

	// If we get a ledger greater than the one we are anticipating, we
	// keep it here until all ledgers in between have been filled
	outOfOrderBuffer map[uint64]*Ledger
}

func NewMonitor(startLedgerIdx uint64) *Monitor {
	m := &Monitor{
		Ledgers:          make(chan *Ledger),
		nextLedgerIdx:    startLedgerIdx,
		outOfOrderBuffer: make(map[uint64]*Ledger),
	}
	go m.Run()
	return m
}

func (m *Monitor) runOnServer(uri string) (err error) {
	var c *Connection
	var ledgerIdx uint64

	// Open a new connection to ripple
	c, err = Connect(uri)
	if err != nil {
		return err
	}
	go c.Monitor()
	defer func() {
		c.t.Kill(nil)
		c.t.Wait()
		c.conn.Close()
	}()

	reqNextLedgerTimer := time.AfterFunc(0, func() {
		c.GetLedger(m.nextLedgerIdx)
		log.Printf("Asked for ledger %d", m.nextLedgerIdx)
	})
	defer reqNextLedgerTimer.Stop()

	watchdogTimer := time.NewTimer(Timeout)

	for {
		select {
		case <-watchdogTimer.C:
			return errors.New("Timed out")

		case <-m.t.Dying():
			return nil

		case ledger := <-c.Ledgers:
			ledgerIdx, err = strconv.ParseUint(ledger.Index, 10, 64)

			switch {
			case ledgerIdx == m.nextLedgerIdx:
				// Ledger is the one we need next. Emit it.
				m.Ledgers <- ledger
				m.nextLedgerIdx++

				// If we already have any of the next ledgers, emit them now too.
				for ; m.outOfOrderBuffer[m.nextLedgerIdx] != nil; m.nextLedgerIdx++ {
					m.Ledgers <- m.outOfOrderBuffer[m.nextLedgerIdx]
					delete(m.outOfOrderBuffer, m.nextLedgerIdx)
				}

				// If we still have a gap, request the next ledger
				if len(m.outOfOrderBuffer) > 0 {
					reqNextLedgerTimer.Reset(500 * time.Millisecond)
				}

				watchdogTimer.Reset(Timeout)

			case ledgerIdx > m.nextLedgerIdx:
				// If a gap was just created, start filling it
				if len(m.outOfOrderBuffer) == 0 {
					reqNextLedgerTimer.Reset(100 * time.Millisecond)
				}

				// This is a later ledger. Stash it for now.
				m.outOfOrderBuffer[ledgerIdx] = ledger
				log.Printf("Stashed ledger %d", ledgerIdx)

			default:
				log.Printf("Received old ledger: %d", ledgerIdx)
			}
		}
	}
}

func (m *Monitor) Run() {
	uriIndex := 0
	defer m.t.Done()

	for {
		log.Printf("Connecting: %s", URIs[uriIndex])
		err := m.runOnServer(URIs[uriIndex])
		log.Printf("Connection failed: %s", err)

		uriIndex = (uriIndex + 1) % len(URIs)

		// If the tomb is marked dying, exit cleanly
		select {
		case <-m.t.Dying():
			return
		default:
			//pass
		}
	}
}
