package main

import (
	"crypto/sha512"
	"encoding/base64"
	"fmt"
	"github.com/go-redis/redis"
	"github.com/miekg/dns"
	"net"
	"os"
	"strings"
	"time"
)

// underscore means global
const (
	_dnsPort       = ":4453"
	_masterAdminPort = ":4454"
	_slavePort     = ":4443" // the slave proxy server
	_timeout       = 120     // timeout in seconds
)

var _password string
var _domain = "ear7h.net"
var _domainDot = "ear7h.net."
var _start = time.Now()
var _masterIP string
var _store *redis.Client

func init() {
	_masterIP = os.Getenv("EAR7H_ROOT")

	var redisAddr = "localhost:6379"
	if os.Getenv("EAR7H_ENV") == "prod" {
		redisAddr = "redis:6379"
	}

	_store = redis.NewClient(&redis.Options{
		Addr:     redisAddr,
		Password: "",
		DB:       0,
	})

	pong, err := _store.Ping().Result()
	if pong != "PONG" || err != nil {
		fmt.Println(pong, err)
	}

	_password = "asd"
}

type Block struct {
	Hostname  string    `json:"hostname"`
	Signature string    `json:"signature"`
	Timestamp time.Time `json:"timestamp"`
	Services  []string  `json:"services"`
	ip        string    // filled in by admin server
}

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
	for _, v := range name {
		if (v < 'a' || v > 'z') && (v < 'A' || v > 'Z') && (v < '0' || v > '9') && (v != '-') {
			return false
		}
	}

	return true
}

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
func addBlock(b Block) (ret []string) {
	if bok := verifyBlock(b); !bok {
		return
	}


	// keep track of a records
	err := _store.SAdd("_hosts", b.Hostname+"."+_domainDot).Err()
	if err != nil {
		return
	}


	// get A records
	arr, err := _store.SMembers("_hosts").Result()
	if err != nil {
		return
	}

	// add A record for the node
	_store.Set(b.Hostname+"."+_domainDot, b.ip, _timeout*time.Second)
	ret = []string{b.Hostname+"."+_domainDot}

	// add CNAME records, making sure they don't overwrite
	// the A records
	for _, v := range b.Services {
		if in(v, &arr) || !validHostName(v) {
			continue
		}
		_store.Set(v+"."+_domainDot, b.Hostname+_domainDot, _timeout*time.Second)
		ret = append(ret, v+"."+_domainDot)
	}
	return

}

func query(name string) (rr []dns.RR) {
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
		_store.Del(name)
		return
	}

	ttl, err := _store.TTL(name).Result()
	if err != nil {
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

func clean() {
	_store.SAdd("_hosts", _domainDot)
	arr, err := _store.SMembers("_hosts").Result()
	// should not be redis.Nil
	if err != nil {
		panic(err)
	}

	for _, v := range arr {
		ip, err := _store.Get(v).Result()
		if err != nil {
			continue
		}

		c, err := net.Dial("tcp", ip+_slavePort)
		if err != nil {
			fmt.Println("cleaning: ", v)
			_store.SRem(v)
			_store.Expire(v, 0)
			continue
		}
		c.Close()
	}

	err = _store.Set(_domainDot, _masterIP, _timeout*time.Second).Err()
	if err != nil {
		panic(err)
	}
}

func main() {

	if _masterIP == "" {
		fmt.Println("EAR7H_ROOT not specified, exiting")
		os.Exit(1)
	}

	hang := make(chan bool, 1)
	go func() {
		panic(serveAdmin())
	}()

	go func() {
		panic(serveDNS())
	}()

	go func() {
		for {
			clean()
			time.Sleep((_timeout * 9 / 10) * time.Second)
		}
	}()

	panic(<- hang)
}
