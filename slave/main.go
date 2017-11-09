package main

import (
	"time"
	"strings"
	"encoding/base64"
	"crypto/sha512"
	"os"
	"sync"
	"fmt"
	"github.com/ear7h/edns/client"
	"net"
	"net/http"
	"encoding/json"
	"bytes"
	"io/ioutil"
)

const(
	_masterAdminPort = ":4454"
	_slaveAdminPort = ":4455"
	_proxyPort = ":4443"
	_timeout       = 120     // timeout in seconds
)

var _hostname string
var _masterHost string
var _password string
var _portMin = 8080
var _portMax = 8090 // exclusive
// subdomain : portString
var _localServices = map[string]string{}
var regLock sync.Mutex

func init() {
	var err error
	_hostname, err = os.Hostname()
	if err != nil {
		panic(err)
	}

	_masterHost = "ear7h.net"
	_password = "asd"
}

type Block struct {
	Hostname string `json:"hostname"`
	Signature string `json:"signature"`
	Timestamp time.Time `json:"timestamp"`
	Services []string `json:"services"`
	ip string // filled in by admin server
}

func signBlock(b *Block) {
	b.Hostname = _hostname
	b.Timestamp = time.Now()

	str := b.Hostname +
		_password +
		b.Timestamp.Format(time.RFC3339Nano) +
		strings.Join(b.Services, "")

	sum := sha512.Sum512([]byte(str))
	b.Signature = base64.StdEncoding.EncodeToString(sum[:])
}

func register(r client.Request) (resBody []byte, err error) {
	regLock.Lock()

	for k, v := range _localServices {
		if r.Addr == v {
			err = fmt.Errorf("Address %s in use by %s", v, k)
			regLock.Unlock()
			return
		}
	}

	_localServices[r.Name] = r.Addr
	regLock.Unlock()

	b := Block{
		Services: []string{r.Name},
	}

	signBlock(&b)

	byt, err := json.Marshal(b)
	if err != nil {
		return
	}

	res, err := http.Post(_masterHost + _masterAdminPort, "text/json", bytes.NewReader(byt))
	if err != nil {
		return
	}
	resBody, _ = ioutil.ReadAll(res.Body)
	res.Body.Close()

	if res.StatusCode != http.StatusOK {
		err = fmt.Errorf("%d response: %s", res.StatusCode, string(resBody))

		regLock.Lock()
		delete(_localServices, r.Name)
		regLock.Unlock()
	}

	return

}

func clean() {
	regLock.Lock()
	defer regLock.Unlock()

	for k, v := range _localServices {
		conn, err := net.Dial("tcp", v)
		if err != nil {
			delete(_localServices, k)
			continue
		}
		conn.Close()
	}
}

func post() {
	regLock.Lock()
	services := make([]string, len(_localServices))
	i := 0
	for k := range _localServices {
		services[i] = k
		i++
	}
	regLock.Unlock()

	b := Block{
		Services: services,
	}

	signBlock(&b)

	byt, err := json.Marshal(b)
	if err != nil {
		fmt.Println(err)
	}

	_, err = http.Post("http://"+_masterHost + _masterAdminPort, "text/json", bytes.NewReader(byt))
	if err != nil {
		fmt.Println(err)
	}
}

func main() {

	go func() {
		for {
			clean()
			post()
			time.Sleep(_timeout * time.Second)
		}
	}()

	panic(serveAdmin())
}