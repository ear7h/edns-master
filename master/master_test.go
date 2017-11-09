package main

import (
	"bytes"
	"crypto/sha512"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"
)

func init() {
	_masterIP = "127.0.0.1"
}

func signBlock(b *Block) {
	//b.Hostname = _hostname
	b.Timestamp = time.Now()

	str := b.Hostname +
		_password +
		b.Timestamp.Format(time.RFC3339Nano) +
		strings.Join(b.Services, "")

	sum := sha512.Sum512([]byte(str))
	b.Signature = base64.StdEncoding.EncodeToString(sum[:])
}

func TestMain(t *testing.T) {
	go main()

	b := Block{
		Hostname: "testhost",
		Services: []string{"test-service"},
	}

	signBlock(&b)

	byt, err := json.Marshal(b)
	if err != nil {
		panic(err)
	}

	_, err = http.Post("http://"+_masterIP+_masterAdminPort, "text/json", bytes.NewReader(byt))
	if err != nil {
		panic(err)
	}

	resp, err := http.Get("http://" + _masterIP + _masterAdminPort)
	if err != nil {
		panic(err)
	}

	io.Copy(os.Stdout, resp.Body)
	resp.Body.Close()
}

func TestMainHang(t *testing.T) {
	main()
}
