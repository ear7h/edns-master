package main

import (
	"encoding/json"
	"io/ioutil"
	"net/http"
	"strings"
	"fmt"
)

func removePort(addr string) string {
	return addr[:strings.LastIndex(addr, ":")]
}

func serveAdmin() error {
	return http.ListenAndServe(_masterAdminPort, makeAdminHandler())
}

func makeAdminHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			arr, err := _store.Keys("*").Result()
			if err != nil {
				http.Error(w, "error fetching data from redis", http.StatusInternalServerError)
				return
			}

			byt, err := json.Marshal(arr)
			if err != nil {
				http.Error(w, "error marshaling response", http.StatusInternalServerError)
				return
			}
			w.WriteHeader(http.StatusOK)
			w.Write(byt)

		case http.MethodPost:

			byt, err := ioutil.ReadAll(r.Body)
			if err != nil {
				http.Error(w, "could not read body", http.StatusInternalServerError)
				return
			}
			r.Body.Close()

			blk := new(Block)

			err = json.Unmarshal(byt, blk)
			if err != nil {
				http.Error(w, "could not unmarshal json", http.StatusBadRequest)
				return
			}

			blk.ip = removePort(r.RemoteAddr)

			addedRecords := addBlock(*blk)

			byt, err = json.Marshal(addedRecords)
			if err != nil {
				http.Error(w, "couldn't marshal response", http.StatusResetContent)
				return
			}

			w.WriteHeader(http.StatusOK)
			w.Write(byt)
		}
	}
}
