package main

import (
	"bufio"
	"flag"
	"fmt"
	"log"
	"net"
	"os"
	"strconv"
	"strings"
)

const (
	defaultHost           = "127.0.0.1"
	defaultPort           = 5201
	defaultReadBufferSize = 16384
)

var (
	host           string
	port           int
	ReadBufferSize int
	ReadBuffer     []byte
	disconnect     = false
)

func init() {
	flag.StringVar(&host, "h", defaultHost, "proxy host(ip address)")
	flag.IntVar(&port, "p", defaultPort, "proxy port")
	flag.IntVar(&ReadBufferSize, "readBufferSize", defaultReadBufferSize, "client read buffer size")
	flag.Parse()
	ReadBuffer = make([]byte, ReadBufferSize)
}

func printHelp() {
	fmt.Println("-----help-----")
	fmt.Println("info\t" + "show EasyProxy infomation")
	fmt.Println("AddServer [addr] [weight] [probe]\t" + "add server to proxy")
	fmt.Println("DeleteServer [addr]\t" + "delete server from proxy")
	fmt.Println("GetServer [addr]\t" + "get specified server information")
	fmt.Println("Exists [addr]\t" + "query specified server exists or not")
	fmt.Println("SetWeight [addr]\t" + "set the weight of specified server")
	fmt.Println("Shutdown\t" + "shutdown server gracefully")
	fmt.Println("save\t" + "save proxy current server list to disk")
	fmt.Println("-h / -help \t" + "display help")
	fmt.Println("-q / -quit \t" + "exit client")
}

func main() {
	var err error
	connAddr := host + ":" + strconv.Itoa(port)
	conn, err := net.Dial("tcp", connAddr)
	if err != nil {
		log.Fatal("connect proxy error: ", err)
	}

	defer conn.Close()

	inputReader := bufio.NewReader(os.Stdin)

	for {
		if !disconnect {
			fmt.Print(connAddr + "> ")
		} else {
			conn, err = net.Dial("tcp", connAddr)
			if err != nil {
				fmt.Print(connAddr + "(disconnect)> ")
			} else {
				fmt.Print(connAddr + "> ")
				disconnect = false
			}
		}

		input, _ := inputReader.ReadString('\n')
		input = strings.Trim(input, "\r\n")
		input = strings.ToLower(input)

		if input == "-q" || input == "-quit" {
			fmt.Println("Bye,Have a good day!")
			break
		}

		if input == "-h" || input == "-help" {
			printHelp()
			continue
		}

		if disconnect {
			continue
		}

		_, err = conn.Write([]byte(input))
		if err != nil {
			fmt.Println("write to EH-Proxy failed, err:", err)
			disconnect = true
			continue
		}
		n, err := conn.Read(ReadBuffer)
		if err != nil {
			fmt.Println("receive from EH-Proxy failed, err:", err)
			disconnect = true
			continue
		}
		fmt.Println(string(ReadBuffer[:n]))
	}

}
