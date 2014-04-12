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
	"wss://s-west.ripple.com:443",
	"wss://s-east.ripple.com:443",
	"wss://s1.ripple.com:443",
}

type LedgerEmitter interface {
	Ledgers() chan *Ledger
}

type Monitor struct {
	t       tomb.Tomb
	ledgers chan *Ledger

	// Next ledger to anticipate
	nextLedgerIdx uint64

	// If we get a ledger greater than the one we are anticipating, we
	// keep it here until all ledgers in between have been filled
	outOfOrderBuffer map[uint64]*Ledger
}

func NewMonitor(startLedgerIdx uint64) *Monitor {
	m := &Monitor{
		ledgers:          make(chan *Ledger),
		nextLedgerIdx:    startLedgerIdx,
		outOfOrderBuffer: make(map[uint64]*Ledger),
	}
	go m.loop()
	return m
}

func (m *Monitor) loop() {
	uriIndex := 0
	defer m.t.Done()

	for {
		log.Printf("Connecting: %s", URIs[uriIndex])
		err := m.handleConnection(URIs[uriIndex])
		log.Printf("Connection failed: %s", err)

		uriIndex = (uriIndex + 1) % len(URIs)

		select {
		case <-m.t.Dying():
			// If the tomb is marked dying, exit cleanly
			return
		default:
			// Wait a bit before reconnecting to another server
			time.Sleep(5 * time.Second)
		}
	}
}

func (m Monitor) Ledgers() chan *Ledger {
	return m.ledgers
}

func (m *Monitor) handleConnection(uri string) (err error) {
	var c *Connection
	var ledgerIdx uint64

	// Open a new connection to ripple
	c, err = NewConnection(uri)
	if err != nil {
		return err
	}
	defer func() {
		c.t.Kill(nil)
		c.t.Wait()
	}()

	reqNextLedgerTimer := time.AfterFunc(0, func() {
		c.GetLedger(m.nextLedgerIdx)
	})
	defer reqNextLedgerTimer.Stop()

	watchdogTimer := time.NewTimer(Timeout)

	for {
		select {
		case <-watchdogTimer.C:
			return errors.New("Timed out")

		case <-m.t.Dying():
			// We are exiting cleanly
			return nil

		case ledger, ok := <-c.Ledgers:
			if !ok {
				// Ripple connection is dead
				c.t.Kill(nil)
				return c.t.Wait()
			}

			ledgerIdx, err = strconv.ParseUint(ledger.Index, 10, 64)
			if err != nil {
				log.Printf("Error parsing ledger index: %s", err)
				continue
			}

			switch {
			case ledgerIdx == m.nextLedgerIdx:
				// Ledger is the one we need next. Emit it.
				m.ledgers <- ledger
				m.nextLedgerIdx++

				// If we already have any of the next ledgers, emit them now too.
				for ; m.outOfOrderBuffer[m.nextLedgerIdx] != nil; m.nextLedgerIdx++ {
					m.ledgers <- m.outOfOrderBuffer[m.nextLedgerIdx]
					delete(m.outOfOrderBuffer, m.nextLedgerIdx)
				}

				// If we still have a gap, request the next ledger
				if len(m.outOfOrderBuffer) > 0 {
					reqNextLedgerTimer.Reset(100 * time.Millisecond)
				}

				watchdogTimer.Reset(Timeout)

			case ledgerIdx > m.nextLedgerIdx:
				// If a gap was just created, start filling it
				if len(m.outOfOrderBuffer) == 0 {
					reqNextLedgerTimer.Reset(100 * time.Millisecond)
				}

				// This is a later ledger. Stash it for now.
				m.outOfOrderBuffer[ledgerIdx] = ledger

			default:
				log.Printf("Received old ledger: %d", ledgerIdx)
			}
		}
	}
}
