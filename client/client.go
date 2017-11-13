/*
this package controls the slave via local http calls
 */
package client

import (
	"strconv"
	"net/http"
	"encoding/json"
	"bytes"
	"net"
	"strings"
	"io"
	"os"
)

/*
calls

port - returns new port number
register [name] - returns port number
	can be used with other commands ie `testserver $(edns register testserver)
 */

const (
	_slaveAdminPort  = ":4455"
)

type Request struct {
	Name string `json:"name"`
	Port uint `json:"port"`
}

// Listen registers the service to a random port and returns a listener
func Listen(name, ip string) net.Listener {
	l, err := net.Listen("tcp", ":0")
	if err != nil {
		panic(err)
	}

	addr := l.Addr().String()
	port64, err := strconv.ParseUint(addr[strings.LastIndex(addr, ":") + 1:], 10, 64)

	if err != nil {
		panic(err)
	}

	p := uint(port64)

	Register(name, p, ip)

	return l
}

func Register(name string, port uint, ip string) {
	req := Request{
		Name: name,
		Port: port,
	}

	byt, err := json.Marshal(req)
	if err != nil {
		panic(err)
	}

	if ip == "" {ip = "127.0.0.1"}


	res, err := http.Post("http://"+ip+_slaveAdminPort, "text/json", bytes.NewReader(byt))
	if err != nil {
		panic(err)
	}

	if res.StatusCode != http.StatusOK {
		io.Copy(os.Stdout, res.Body)
		panic(res.Status)
	}
}