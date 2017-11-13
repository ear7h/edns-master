package main

import (
	"net/http"
	"net/http/httputil"
	"golang.org/x/crypto/acme/autocert"
	"context"
	"fmt"
	//"crypto/tls"
	"crypto/tls"
)

var _reverseProxy = httputil.ReverseProxy{
	Director: func(r *http.Request) {
		subdomain := r.Host[:len(r.Host) - len(_masterHost+_proxyPort) - 1]
		upstream := _localServices[subdomain]

		r.URL.Scheme = "http"
		r.URL.Host = upstream
	},
}

var _tlsManager = autocert.Manager{
	Cache: autocert.DirCache("/var/ear7h/edns/certs"),
	Prompt: autocert.AcceptTOS,
	HostPolicy: func(_ context.Context, host string) error {
		subdomain := host[:len(host) - len(_masterHost) - 1]
		if _, ok := _localServices[subdomain]; !ok {
			return fmt.Errorf("acme/autocert: host not configured")
		}

		return nil
	},
}

func serveProxy() error {
autocert.HostWhitelist()

	s := &http.Server{
		Addr: _proxyPort,
		TLSConfig: &tls.Config{GetCertificate: _tlsManager.GetCertificate},
		Handler: makeProxyHandler(),
	}

	return s.ListenAndServeTLS("","")


	//return http.ListenAndServe(_proxyPort, makeProxyHandler())
}

func makeProxyHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Proto == "ws" {
			proxyWS(w, r)
		}
		_reverseProxy.ServeHTTP(w, r)
	}
}
