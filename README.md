# ripple

A Go library for talking to [rippled](https://github.com/ripple/rippled) servers.

## Features

* Subscribes to transactions
* Parses them into appropriate data structures
* That's all (patches welcome)

## Usage

    package main

    import "fmt"
    import "github.com/lukecyca/ripple"

    func main() {
        var r = ripple.Connection{Messages: make(chan *ripple.Message)}
        r.Connect("wss://s_west.ripple.com:443")
        go r.Monitor()

        for {
            m := <- r.Messages
            fmt.Println(m.String())
        }
    }
