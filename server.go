package main

import (
	"encoding/json"
	"goVncTCP/client"
	"goVncTCP/tools"
	"log"
	"net"
	"os"
	"runtime"
	"strconv"
	"time"

	"github.com/go-redis/redis"
	"github.com/tidwall/evio"
)

var redisdb *redis.Client
var clients map[tools.Node]*client.Client
var readChannel chan string
var errorChannel chan tools.Node
var localAddresses []string

func connectToRedis() (err error) {
	//

	redis_host := os.Getenv("REDIS_PORT_6379_TCP_ADDR")
	redis_port := os.Getenv("REDIS_PORT_6379_TCP_PORT")

	if len(redis_host) == 0 {
		//

		redis_host = "0.0.0.0"
	}

	if len(redis_port) == 0 {
		//

		redis_port = "6379"
	}

	redisdb = redis.NewClient(&redis.Options{
		Addr:         net.JoinHostPort(redis_host, redis_port),
		DialTimeout:  10 * time.Second,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
		PoolSize:     10,
		PoolTimeout:  30 * time.Second,
		DB:           1,
	})

	_, err = redisdb.Ping().Result()
	return
}

func getActiveNodes() (nodes []tools.Node, err error) {
	//

	answer := redisdb.ZRange(tools.NodesTable, 0, -1)
	if answer.Err() != nil {
		//

		err = answer.Err()
		return
	}

	for _, val := range answer.Val() {
		//

		var node tools.Node
		err = json.Unmarshal([]byte(val), &node)
		if err != nil {
			//

			log.Println("Can't marshak bytes. ", err)
			continue
		}

		if net.ParseIP(node.ADDRESS) == nil {
			//

			log.Println("Can't parse IP. ")
			continue
		}

		if tools.StringInSlice(node.ADDRESS, localAddresses) {
			//

			log.Println("Our local IP.")
			return
		}

		if len(node.PUBLICKEY) != 66 {
			//

			log.Println("Wrong publickey. ")
			continue
		}

		nodes = append(nodes, node)
	}

	return
}

func handleConnection(c evio.Conn, in []byte) (out []byte, action evio.Action) {
	//

	readChannel <- string(in)
	return
}

func createConnectionsWithNodes() (delay time.Duration, action evio.Action) {
	//

	log.Println("tick", time.Now().Unix())
	delay = 10 * time.Second
	nodes, _ := getActiveNodes()
	log.Println("NODES FROM SERVER: ", nodes)
	for key, client := range clients {
		//

		if !tools.NodeInNodes(key, nodes) {
			//

			log.Println("We don't need this node anymore")
			client.CloseConnection()
			delete(clients, key)
		}
	}

	for _, node := range nodes {
		//

		if _, ok := clients[node]; ok {
			//

			log.Println("Such node already exists")
			continue
		}

		servAddr := net.JoinHostPort(node.ADDRESS, tools.NodePort)
		tcpAddr, err := net.ResolveTCPAddr("tcp", servAddr)
		if err != nil {
			//

			log.Println("ResolveTCPAddr failed:", err.Error())
			continue
		}

		conn, err := net.DialTCP("tcp", nil, tcpAddr)
		if err != nil {
			//

			log.Println("Dial failed:", err.Error())
			continue
		}

		err = conn.SetKeepAlive(true)
		if err != nil {
			//

			log.Println("Set keep alive failed: ", err.Error())
			continue
		}

		err = conn.SetKeepAlivePeriod(3 * time.Second)
		if err != nil {
			//

			log.Println("Set keep alive timeout failed: ", err.Error())
			continue
		}

		log.Println("New connection")
		client := client.NewClient(node, conn, &errorChannel)
		client.Start()

		// Send Hello transaction with signature
		client.Write("HELLO! " + strconv.FormatInt(time.Now().Unix(), 10))
		clients[node] = client
	}

	return
}

func closeConnections() {
	//

	for _, client := range clients {
		//

		client.CloseConnection()
	}
}

func startServer() (err error) {
	//

	var events evio.Events
	events.NumLoops = runtime.NumCPU()
	events.Data = handleConnection
	events.Tick = createConnectionsWithNodes

	return evio.Serve(events, "tcp://"+net.JoinHostPort("0.0.0.0", tools.NodePort))
}

func readWorker() {
	// We get data, checking it validity, signature and resending to everyone
	// Also, here we chek for client errors

	for {
		//

		select {
		case data := <-readChannel:
			//

			log.Println(data)

		case node := <-errorChannel:
			//

			delete(clients, node)
		}
	}
}

func main() {
	//

	clients = make(map[tools.Node]*client.Client)
	readChannel = make(chan string, 1024)
	errorChannel = make(chan tools.Node)
	localAddresses = tools.GetLocalIps()

	if err := connectToRedis(); err != nil {
		//

		panic(err.Error())
	}

	defer redisdb.Close()

	go readWorker()

	if err := startServer(); err != nil {
		//

		panic(err.Error())
	}

	close(readChannel)
	close(errorChannel)
}
