# ripple

A Go library for tracking transactions on the [ripple](https://ripple.com) network.

## Features

This library allows you to subscribe to transactions starting at any arbitrary ledger
index. Since the ripple API doesn't currently support this functionality, this library
will request individual ledgers until it has caught up with the ledger stream.

Ledgers are emitted in order, and contain all of their transactions. This makes it
ideal for robustly tracking transactions. Your application must simply persist the
latest ledger index as it consumes them. When your application restarts, it can
request to start exactly where it left off.

## Usage

package main

import "fmt"
import "github.com/lukecyca/ripple"

func main() {
    var r = ripple.NewMonitor(5008254)

    for {
        ledger := <-r.Ledgers
        fmt.Printf("Ledger %s with %d transactions:\n", ledger.Index, len(ledger.Transactions))
        for _, txn := range ledger.Transactions {
            fmt.Printf("  %s %s\n", txn.Hash, txn.TransactionType)
        }
    }
}
