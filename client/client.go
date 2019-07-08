package client

import (
	"bufio"
	"goVncTCP/tools"
	"net"
)

type Client struct {
	node          tools.Node
	outcomingData chan string
	reader        *bufio.Reader
	writer        *bufio.Writer
	connection    net.Conn
	errorChannel  *chan tools.Node
}

func NewClient(node tools.Node, connection net.Conn, errorChannel *chan tools.Node) (client *Client) {
	//

	client = new(Client)
	client.node = node
	client.outcomingData = make(chan string)
	client.reader = bufio.NewReader(connection)
	client.writer = bufio.NewWriter(connection)
	client.connection = connection
	client.errorChannel = errorChannel

	return client
}

func (client *Client) write() {
	//

	for data := range client.outcomingData {
		//

		_, err := client.writer.WriteString(data)
		if err != nil {
			//

			client.connection.Close()
			*client.errorChannel <- client.node
			return
		}

		err = client.writer.Flush()
		if err != nil {
			//

			client.connection.Close()
			*client.errorChannel <- client.node
			return
		}
	}
}

func (client *Client) Write(data string) {
	//

	client.outcomingData <- data
}

func (client *Client) Start() {
	//

	go client.write()
}

func (client *Client) CloseConnection() {
	//

	client.connection.Close()
}
