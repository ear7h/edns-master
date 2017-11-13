package main

import (
	"net/http"
	"github.com/gorilla/websocket"
	"net/url"
	"io"
)

var _wsUpgrader = websocket.Upgrader{
	ReadBufferSize: 1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) (ok bool) {
		return true
	},
}

func pipeWs(conn1, conn2 *websocket.Conn) {
	// conn1 -> conn2
	go func() {
		defer conn1.Close()
		defer conn2.Close()


		for {
			t, data, err := conn1.ReadMessage()
			if err != nil {
				break
			}

			err = conn2.WriteMessage(t, data)
			if err != nil {
				break
			}

		}

	}()

	// conn2 -> conn1
	go func() {
		defer conn1.Close()
		defer conn2.Close()


		for {
			t, data, err := conn2.ReadMessage()
			if err != nil {
				break
			}

			err = conn1.WriteMessage(t, data)
			if err != nil {
				break
			}

		}

	}()
}

func proxyWS(w http.ResponseWriter, r *http.Request) {

	down, err := _wsUpgrader.Upgrade(w, r, nil)
	if err != nil {
		http.Error(w, "could not upgrade connection", http.StatusInternalServerError)
		return
	}

	subdomain := r.Host[:len(r.Host) - len(_masterHost+_proxyPort) - 1]
	upstream := _localServices[subdomain]

	u := new(url.URL)
	*u = *r.URL
	u.Host = upstream

	up, res, err := websocket.DefaultDialer.Dial(u.String(), r.Header)
	if err != nil {
		down.Close()
		http.Error(w, "could not dial upstream websocket", http.StatusBadGateway)
		return
	}

	go pipeWs(up, down)

	for k, v := range res.Header {
		w.Header()[k] = v
	}

	io.Copy(w, res.Body)
	r.Body.Close()
}