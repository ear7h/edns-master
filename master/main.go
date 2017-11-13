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
	_dnsPort         = ":4453"
	_masterAdminPort = ":4454"
	_slavePort       = ":4443" // the slave proxy server
	_timeout         = 120     // timeout in seconds
)

var _password string
var _domain = "ear7h.net"
var _domainDot = "ear7h.net."
var _changes int
var _masterIP string
var _store *redis.Client
var _hostWhitelist map[string]bool

func init() {
	_password = "asd"

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

	_hostWhitelist = map[string]bool{
		_domainDot: true,
		"ns1."+_domainDot : true,
		"ns2."+_domainDot : true,
	}

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
		if (v < 'a' || v > 'z') &&
			(v < 'A' || v > 'Z') &&
			(v < '0' || v > '9') &&
			(v != '-') && (v != '.') {
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

func clean() {
	_changes++

	for k := range _hostWhitelist {
		err := _store.SAdd("_hosts", k).Err()
		err = _store.Set(k, _masterIP, _timeout*time.Second).Err()
		if err != nil {
			panic(err)
		}
	}

	arr, err := _store.SMembers("_hosts").Result()
	// should not be redis.Nil
	if err != nil {
		panic(err)
	}

	for _, v := range arr {
		if _hostWhitelist[v] {
			continue
		}

		ip, err := _store.Get(v).Result()
		if err != nil {
			continue
		}

		c, err := net.DialTimeout("tcp", ip+_slavePort, 5 * time.Second)
		if err != nil {
			fmt.Println("cleaning: ", v)
			_store.SRem(v)
			_store.Expire(v, 0)
			continue
		}
		c.Close()
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
			time.Sleep((_timeout * 9 / 10) * time.Second)
		}
	}()

	go func() {
		for {
			time.Sleep(24 * time.Hour)
			_changes = 0
		}
	}()

	<-make(chan struct{}, 1)
}
