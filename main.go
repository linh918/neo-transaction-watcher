package main

import (
	"log"
	"os"
	"time"
	"io"
	"github.com/o3labs/neo-transaction-watcher/neotx"
	"github.com/o3labs/neo-transaction-watcher/neotx/network"
)

type Handler struct {
}
var connectedToNEONode = false
//implement the message protocol
func (h *Handler) OnReceive(tx neotx.TX) {
	log.Printf("%+v", tx)
}

func (h *Handler) OnConnected(c network.Version) {
	log.Printf("connected %+v", c)
	connectedToNEONode = true
}

func (h *Handler) OnError(e error) {
	log.Printf("error %+v", e)
	if e == io.EOF && connectedToNEONode == true {
		connectedToNEONode = false
		log.Printf("Disconnected from host. will try to connect in 5 seconds...")
		for {
			time.Sleep(5 * time.Second)
			//we need to implement backoff and retry to reconnect here
			//if the error is EOF then we try to reconnect
			go startConnectToSeed()
		}
	}
}


func startConnectToSeed() {
	const  PRIV_MAGIC = 56753
const  TESTNET_MAGIC = 1953787457
const  MAINET_MAGIC = 7630401
	config := neotx.Config{
		Network:   network.NEONetworkMagic(TESTNET_MAGIC),
		Port:      20333,
		IPAddress: "seed4.neo.org",
		// IPAddress: "127.0.0.1",
	}
	client := neotx.NewClient(config)
	handler := &Handler{}
	client.SetDelegate(handler)

	err := client.Start()
	if err != nil {
		log.Printf("%v", err)
		os.Exit(-1)
	}

	for {

	}
}

func main() {

	startConnectToSeed()
}
