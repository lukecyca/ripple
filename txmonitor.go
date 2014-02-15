package ripple

import (
	"log"
	"strconv"
	"time"
)

type Monitor struct {
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

func (m *Monitor) Run() {
	var c *Connection
	var err error
	var ledgerIdx uint64

	// Open a new connection to ripple
	c, err = Connect("wss://s_west.ripple.com:443")
	if err != nil {
		log.Panicln(err)
	}
	go c.Monitor()

	for {
		select {
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

				// If we have still have a gap, request the next ledger
				if len(m.outOfOrderBuffer) > 0 {
					time.Sleep(500 * time.Millisecond)
					c.GetLedger(m.nextLedgerIdx)
					log.Printf("Asked for ledger %d", m.nextLedgerIdx)
				}

			case ledgerIdx > m.nextLedgerIdx:
				// This is a later ledger. Stash it for now.
				m.outOfOrderBuffer[ledgerIdx] = ledger
				log.Printf("Stashed ledger %d", ledgerIdx)

				// If a gap was just created, start filling it
				if len(m.outOfOrderBuffer) == 1 {
					c.GetLedger(m.nextLedgerIdx)
					log.Printf("Asked for ledger %d", m.nextLedgerIdx)
				}

			}
		}
	}
}
