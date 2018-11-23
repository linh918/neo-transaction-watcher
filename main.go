package main

import (
	"log"
	"io"
	"os"
	"time"
	"github.com/o3labs/neo-transaction-watcher/neotx"
	"github.com/o3labs/neo-transaction-watcher/neotx/network"
	"github.com/streadway/amqp"
)

type Handler struct {
}
var connectedToNEONode = false
//implement the message protocol
func (h *Handler) OnReceive(tx neotx.TX) {
	log.Printf("%+v", tx)
	 if tx.Type.String() == "block"  || tx.Type.String() == "tx" {
		log.Printf(" msg: %+v", tx.ID)
		go sendMsg("{ \"Type\":"+ "\"" + tx.Type.String() + "\","+ "\"ID\":" +"\"" + tx.ID +  "\"}") 
	 }
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
	const  TEST_PORT = 20333
	const  PRIV_PORT = 8333
	const MAIN_PORT = 10333
	const PRIV_ADDRESS = "localhost"
	const MAIN_ADDRESS =  "seed1.aphelion-neo.com"

	config := neotx.Config{
		Network:   network.NEONetworkMagic(PRIV_MAGIC),
		Port:      PRIV_PORT,
		// IPAddress: "seed4.neo.org",
		IPAddress: PRIV_ADDRESS,
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

func failOnError(err error, msg string) {
	if err != nil {
		log.Fatalf("%s: %s", msg, err)
	}
}

func sendMsg(msg string) {
	log.Printf("%s", msg)	
	const QUEUE_NEO_NOTIFICATION = "queue_neo_notification"
	conn, err := amqp.Dial("amqp://deposit_neo:password@127.0.0.1:5672/")
	failOnError(err, "Failed to connect to RabbitMQ")
	defer conn.Close()
	ch, err := conn.Channel()
	failOnError(err, "Failed to open a channel")
	defer ch.Close()
	
	q, err := ch.QueueDeclare(
		QUEUE_NEO_NOTIFICATION, // name
		true,   // durable
		false,   // delete when unused
		false,   // exclusive
		false,   // no-wait
		nil,     // arguments
	)
	failOnError(err, "Failed to declare a queue")
 
	body :=  msg
	err = ch.Publish(
		"",     // exchange
		q.Name, // routing key
		false,  // mandatory
		false,  // immediate
		amqp.Publishing{
			ContentType: "text/plain",
			Body:        []byte(body),
		})
	log.Printf(" [x] Sent %s", body)
	failOnError(err, "Failed to publish a message")
}
