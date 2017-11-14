package main

import (
	"crypto/sha512"
	"encoding/base64"
	"fmt"
	"github.com/go-redis/redis"
	"github.com/miekg/dns"
	"io/ioutil"
	"net"
	"os"
	"strings"
	"time"
)

// underscore means global
const (
	_passwordFile    = "/var/ear7h/edns/password.txt"
	_dnsPort         = ":4453" // docker should bind 53 to this port
	_masterAdminPort = ":4454" // http server for administration of dns
	_childProxyPort  = ":443"  // children's proxy port
	_timeout         = 120     // timeout in seconds
)

// password for authenticating the incoming blocks
var _password string
var _domain = "ear7h.net"

// dns require a trailing period
var _domainDot = "ear7h.net."

// additions or cleans which happened today
var _changes int

// ip address of this node, should be set as EAR7H_ROOT env
var _masterIP string

// client for the redis backend
var _store *redis.Client

// this is a map of names which will never get erased
var _hostWhitelist map[string]bool

func init() {
	// read the pasword from a file
	byt, err := ioutil.ReadFile(_passwordFile)
	if err != nil {
		panic(err)
	}
	_password = string(byt)

	// get the ip address from an environment variable
	_masterIP = os.Getenv("EAR7H_ROOT")

	// redis address
	var redisAddr = "localhost:6379"
	if os.Getenv("EAR7H_ENV") == "prod" {
		// docker
		redisAddr = "redis:6379"
	}

	// make the redis client and ping
	_store = redis.NewClient(&redis.Options{
		Addr:     redisAddr,
		Password: "",
		DB:       0,
	})
	pong, err := _store.Ping().Result()
	if pong != "PONG" || err != nil {
		fmt.Println(pong, err)
	}

	// add the root doamin and name servers to the whitelist
	_hostWhitelist = map[string]bool{
		_domainDot:          true,
		"ns1." + _domainDot: true,
		"ns2." + _domainDot: true,
	}

}

// Block is the standard communication structure between master and child nodes
type Block struct {
	Hostname  string    `json:"hostname"`
	Signature string    `json:"signature"`
	Timestamp time.Time `json:"timestamp"`
	Services  []string  `json:"services"`
	ip        string    // filled in by admin server
}

// array utility function, returns true if x is an element of arr
func in(x string, arr *[]string) (b bool) {
	for _, v := range *arr {
		if v == x {
			return true
		}
	}
	return
}

// alphanumeric plus dash
func validHostName(name string) bool {
	if name == "" {
		return false
	}
	for _, v := range name {
		if (v < 'a' || v > 'z') &&
			(v < 'A' || v > 'Z') &&
			(v < '0' || v > '9') &&
			(v != '-') && (v != '.') {
			return false
		}
	}

	return true
}

// verifies the block was signed with the correct password
func verifyBlock(b Block) (ok bool) {
	if b.Hostname == "" {
		fmt.Println("verification failed, no hostname")
		return
	}

	str := b.Hostname +
		_password +
		b.Timestamp.Format(time.RFC3339Nano) +
		strings.Join(b.Services, "")

	sum := sha512.Sum512([]byte(str))
	shouldSig := base64.StdEncoding.EncodeToString(sum[:])

	return b.Signature == shouldSig
}

// adds a request Block to the redis store
// fails silently
func addBlock(b Block) (ret []string) {
	_changes++
	if ok := verifyBlock(b); !ok {
		fmt.Println("verification failed")
		return
	}

	fmt.Println("adding: ", b)

	hostname := strings.ToLower(b.Hostname)

	// keep track of a records
	err := _store.SAdd("_hosts", hostname+"."+_domainDot).Err()
	if err != nil {
		return
	}

	// add A record for the node
	err = _store.Set(hostname+"."+_domainDot, b.ip, _timeout*time.Second).Err()
	if err != nil {
		fmt.Println(err)
		return
	}
	ret = []string{hostname + "." + _domainDot}

	// get all A records
	arr, err := _store.SMembers("_hosts").Result()
	if err != nil {
		return
	}

	// add CNAME records, making sure they don't overwrite
	// the A records
	for _, v := range b.Services {
		if in(v, &arr) || !validHostName(v) {
			fmt.Println(v, " invalid")
			continue
		}

		v = strings.ToLower(v)

		fmt.Println("adding service: ", v+"."+_domainDot)

		err = _store.Set(v+"."+_domainDot, hostname+"."+_domainDot, _timeout*time.Second).Err()
		if err != nil {
			fmt.Println(err)
		}
		ret = append(ret, v+"."+_domainDot)
	}
	return

}

// given the name of a CNAME or A/AAAA record, return
// the matching resource records
// in the case of cname records, the pointed A/AAAA are returned also
func query(name string) (rr []dns.RR) {
	name = strings.ToLower(name)

	// get A records
	arr, err := _store.SMembers("_hosts").Result()
	if err != nil {
		return
	}

aRecord:

	if in(name, &arr) {
		ip, err := _store.Get(name).Result()
		if err != nil {
			return
		}
		ttl, err := _store.TTL(name).Result()
		if err != nil {
			return
		}

		rr = append(rr, &dns.A{
			Hdr: dns.RR_Header{
				Name:   name,
				Rrtype: dns.TypeA,
				Class:  dns.ClassINET,
				Ttl:    uint32(ttl.Seconds()),
			},
			A: net.ParseIP(ip),
		}, &dns.AAAA{
			Hdr: dns.RR_Header{
				Name:   name,
				Rrtype: dns.TypeAAAA,
				Class:  dns.ClassINET,
				Ttl:    uint32(ttl.Seconds()),
			},
			AAAA: net.ParseIP(ip),
		})
		return
	}

	target, err := _store.Get(name).Result()
	if err != nil || !in(target, &arr) {
		fmt.Println("redis miss: [name, target, err]", name, target, err)
		_store.Del(name)
		return
	}

	ttl, err := _store.TTL(name).Result()
	if err != nil {
		fmt.Println("[name, err]", name, err)
		return
	}

	rr = append(rr, &dns.CNAME{
		Hdr: dns.RR_Header{
			Name:   name,
			Rrtype: dns.TypeCNAME,
			Class:  dns.ClassINET,
			Ttl:    uint32(ttl.Seconds()),
		},
		Target: target,
	})

	name = target
	goto aRecord

}

// cleans the redis server
func clean() {
	_changes++

	// whitelisted hosts are all served by this program
	// therefore they can be A records (ip string value in redis)
	for k := range _hostWhitelist {
		err := _store.SAdd("_hosts", k).Err()
		err = _store.Set(k, _masterIP, _timeout*time.Second).Err()
		if err != nil {
			panic(err)
		}
	}

	// get a list of a records
	hosts, err := _store.SMembers("_hosts").Result()
	// should not be redis.Nil
	if err != nil {
		panic(err)
	}

	// only the _hosts set needs to be updated
	// as the regular keys expire
	for _, host := range hosts {
		// skip the whitelist
		if _hostWhitelist[host] {
			continue
		}

		// if the host doesn't exist, it timed out
		// and so have the CNAMEs pointing to it
		err := _store.Get(host).Err()
		if err == redis.Nil {
			fmt.Println("cleaning: ", host)
			_store.SRem(host)
		}

	}
}

func main() {

	if _masterIP == "" {
		fmt.Println("EAR7H_ROOT not specified, exiting")
		os.Exit(1)
	}

	go func() {
		panic(serveAdmin())
	}()

	go func() {
		panic(serveDNS())
	}()

	go func() {
		for {
			clean()
			time.Sleep(_timeout * time.Second)
		}
	}()

	go func() {
		for {
			// sleep until tomorrow, reset the changes
			// and do it all again
			time.Sleep(
				time.Until(
					time.Now().
						Add(24 * time.Hour).
						Truncate(24 * time.Hour),
				),
			)

			_changes = 0
		}
	}()

	<-make(chan struct{}, 1)
}
