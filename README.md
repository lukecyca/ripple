# ripple-go

A Go library for talking to [rippled](https://github.com/ripple/rippled) servers.

## Features

* Subscribes to transactions
* Parses them into appropriate data structures
* That's all (patches welcome)

## Usage

    package "main"

    import "github.com/lukecyca/ripple-go"

    func main() {
        var r = Connection{Messages: make(chan *Message)}
        r.Connect("wss://s_west.ripple.com:443")
        go r.Monitor()

        for {
            m := <- r.Messages
            fmt.Println(m.String())
        }
    }
