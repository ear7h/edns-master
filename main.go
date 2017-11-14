/*
this file is the cli app of the client
 */
package main

import (
	"flag"
	"fmt"
	"strconv"
	"strings"
	"github.com/ear7h/edns/client"
)

var name string
var nodeIp string
var port uint
var getPort bool

func init() {
	flag.StringVar(&name,"n", "", "name of service")

	flag.StringVar(&nodeIp,"l", "127.0.0.1", "ip address of local node")

	flag.UintVar(&port,"p", 0, "port for the service")

	flag.BoolVar(&getPort, "g", false, "if this flag is set, this program will automatically find a port and write it to stdout")
}

func main() {
	flag.Parse()


	if flag.NArg() == 2 {
		name = flag.Arg(0)
		port64, err := strconv.ParseUint(flag.Arg(1), 10, 64)
		if err != nil {
			flag.Usage()
			return
		}

		port = uint(port64)
	}

	if name == "" {
		fmt.Println("must specify name")
		flag.Usage()
		return
	}

	if getPort {
		l := client.Listen(name, nodeIp)

		addr := l.Addr().String()
		addr = addr[:strings.LastIndex(addr, ":")]
		fmt.Println(addr)
		l.Close()
	} else {
		client.Register(name, port, nodeIp)
	}


	fmt.Println("name: ", name)
	fmt.Println("port: ", port)
	fmt.Println("getPort: ", getPort)
}