package main

import (
	"log"
	// "io"
	"strings"
	"os"
	"time"
	"github.com/o3labs/neo-transaction-watcher/neotx"
	"github.com/o3labs/neo-transaction-watcher/neotx/network"
	"github.com/streadway/amqp"
	"encoding/json"
	"net"
	"net/http"
	"sort"
	"sync"
)

type Handler struct {
}
var connectedToNEONode = false
var hasError = false
var wg = new(sync.WaitGroup) 
//implement the message protocol
func (h *Handler) OnReceive(tx neotx.TX) {
	log.Printf("%+v", tx)
	 if tx.Type.String() == "block" || tx.Type.String() == "tx" {
		log.Printf(" msg: %+v", tx.ID)
		go sendMsg("{ \"Type\":"+ "\"" + tx.Type.String() + "\","+ "\"ID\":" +"\"" + tx.ID +  "\"}") 
	 }
}

func (h *Handler) OnConnected(c network.Version) {
	log.Printf("connected %+v", c)
	connectedToNEONode = true
	hasError = false 
}

func (h *Handler) OnError(e error) {
	log.Printf("error %+v", e)
	// if e == io.EOF && connectedToNEONode == true {
	if connectedToNEONode == true {
		connectedToNEONode = false
		log.Printf("Disconnected from host. will try to connect in 5 seconds...")
		// for {
			
			time.Sleep(5 * time.Second)
			//we need to implement backoff and retry to reconnect here
			//if the error is EOF then we try to reconnect
			// wg.Add(1)
			// wg.Done() 
			
			hasError = true
		}
	// }
}

func startConnectToSeed() {
	const  PRIV_MAGIC = 56753
	const  TESTNET_MAGIC = 1953787457
	const  MAINET_MAGIC = 7630401
	const  TEST_PORT = 20333
	const  PRIV_PORT = 8333
	const MAIN_PORT = 10333
	const PRIV_ADDRESS = "118.27.35.112"
	// const TEST_ADDRESS = "118.27.35.112"
	const TEST_ADDRESS = "test1.cityofzion.io"
	const MAIN_ADDRESS =  "seed9.ngd.network"

	// var nodes = []string{"http://seed8.ngd.network:10332", "http://seed6.ngd.network:10332", "http://seed3.ngd.network:10332"}
	// var a = getBestNode(nodes)
	// log.Printf("getBestNode %+v", a)

	config := neotx.Config{
		Network:   network.NEONetworkMagic(MAINET_MAGIC),
		Port:      MAIN_PORT,
		// IPAddress: "seed4.neo.org",
		IPAddress: MAIN_ADDRESS,
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
		if hasError {
			startConnectToSeed()
			hasError = false
			return
		}
	}
}

func main() {
	// wg.Add(1)
	startConnectToSeed()
	// wg.Wait()
	// startConnectToSeed()
}

func failOnError(err error, msg string) {
	if err != nil {
		log.Fatalf("%s: %s", msg, err)
	}
}

func sendMsg(msg string) {
	log.Printf("%s", msg)	
	const QUEUE_NEO_NOTIFICATION = "queue_neo_notification"
	conn, err := amqp.Dial("amqp://test:test@127.0.0.1:5762")
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

func getBestNode(list []string) *SeedNodeResponse {
	commaSeparated := strings.Join(list, ",")
	return SelectBestSeedNode(commaSeparated)
}



//******************************************** utils *****************************************************************************8

type customTransport struct {
	rtp       http.RoundTripper
	dialer    *net.Dialer
	connStart time.Time
	connEnd   time.Time
	reqStart  time.Time
	reqEnd    time.Time
}

func newTransport() *customTransport {

	tr := &customTransport{
		dialer: &net.Dialer{
			Timeout:   1 * time.Second, //keep timeout low
			KeepAlive: 1 * time.Second,
		},
	}
	tr.rtp = &http.Transport{
		Proxy:                 http.ProxyFromEnvironment,
		Dial:                  tr.dial,
		TLSHandshakeTimeout:   1 * time.Second,
		ResponseHeaderTimeout: 1 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
	}
	return tr
}

func (tr *customTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	tr.reqStart = time.Now()
	resp, err := tr.rtp.RoundTrip(r)
	tr.reqEnd = time.Now()
	return resp, err
}

func (tr *customTransport) dial(network, addr string) (net.Conn, error) {
	tr.connStart = time.Now()
	cn, err := tr.dialer.Dial(network, addr)
	tr.connEnd = time.Now()
	return cn, err
}

func (tr *customTransport) ReqDuration() time.Duration {
	return tr.Duration() - tr.ConnDuration()
}

func (tr *customTransport) ConnDuration() time.Duration {
	return tr.connEnd.Sub(tr.connStart)
}

func (tr *customTransport) Duration() time.Duration {
	return tr.reqEnd.Sub(tr.reqStart)
}

type BlockCountResponse struct {
	Jsonrpc      string `json:"jsonrpc"`
	ID           int    `json:"id"`
	Result       int    `json:"result"`
	ResponseTime int64  `json:"-"`
}

func fetchSeedNode(url string) *BlockCountResponse {
	//instead of using default http client. we use a transport one here.
	//because we need to mearure the time. Request, Response and total duration.
	//to select the best node among the nodes that has the highest blockcount by picking the least latency node.
	log.Printf("  url %v", url)
	transport := newTransport()
	client := http.Client{Transport: transport}
	payload := strings.NewReader(" {\"jsonrpc\": \"2.0\", \"method\": \"getblockcount\", \"params\": [], \"id\": 1}")
	res, err := client.Post(url, "application/json", payload)
	log.Printf("  err %v", err)
	if err != nil || res == nil {
		return nil
	}
	defer res.Body.Close()
	blockResponse := BlockCountResponse{}
	log.Printf("  blockResponse %v", blockResponse)
	
	err = json.NewDecoder(res.Body).Decode(&blockResponse)
	if err != nil {
		return nil
	}
	blockResponse.ResponseTime = transport.ReqDuration().Nanoseconds()
	return &blockResponse
}

type FetchSeedRequest struct {
	Response *BlockCountResponse
	URL      string
}

type SeedNodeResponse struct {
	URL          string
	BlockCount   int
	ResponseTime int64 //milliseconds
}

type NodeList struct {
	URL []string
}

//go mobile bind does not support slice parameters...yet
//https://github.com/golang/go/issues/12113

func SelectBestSeedNode(commaSeparatedURLs string) *SeedNodeResponse {
	urls := strings.Split(commaSeparatedURLs, ",")
	ch := make(chan *FetchSeedRequest, len(urls))
	fetchedList := []string{}
	wg := sync.WaitGroup{}
	listHighestNodes := []SeedNodeResponse{}
	for _, url := range urls {
		go func(url string) {
			res := fetchSeedNode(url)
			ch <- &FetchSeedRequest{res, url}
		}(url)
	}
	wg.Add(1)

loop:
	for {
		select {
		case request := <-ch:
			if request.Response != nil {
				listHighestNodes = append(listHighestNodes, SeedNodeResponse{
					URL:          request.URL,
					BlockCount:   request.Response.Result,
					ResponseTime: request.Response.ResponseTime / int64(time.Millisecond),
				})
			}

			fetchedList = append(fetchedList, request.URL)
			if len(fetchedList) == len(urls) {

				// if len(listHighestNodes) == 0 {
				// 	continue
				// }
				wg.Done()
				break loop
			}
		}
	}
	//wait for the operation
	wg.Wait()
	//using sort.SliceStable to sort min response time first
	sort.SliceStable(listHighestNodes, func(i, j int) bool {
		return listHighestNodes[i].ResponseTime < listHighestNodes[j].ResponseTime
	})
	//using sort.SliceStable to sort block count and preserve the sorted position
	sort.SliceStable(listHighestNodes, func(i, j int) bool {
		return listHighestNodes[i].BlockCount > listHighestNodes[j].BlockCount
	})
	log.Printf("  listHighestNodes %v", len(listHighestNodes))
	if len(listHighestNodes) == 0 {
		return nil
	}
	return &listHighestNodes[0]
}
