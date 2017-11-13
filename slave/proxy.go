package main

import (
	"net/http"
	"net/http/httputil"
	"golang.org/x/crypto/acme/autocert"
	"context"
	"fmt"
	"crypto/tls"
	"strings"
)

var _reverseProxy = httputil.ReverseProxy{
	Director: func(r *http.Request) {
		upstream, ok := _localServices[r.Host]
		if !ok {
			fmt.Println(r.Host)
			fmt.Println(_localServices)
		}
		r.URL.Scheme = "http"
		r.URL.Host = upstream
	},
}

var _tlsManager = autocert.Manager{
	Cache: autocert.DirCache("/var/ear7h/edns/certs"),
	Prompt: autocert.AcceptTOS,
	HostPolicy: func(_ context.Context, host string) error {


		subdomain := host[:len(host) - len(_masterHost) - 1]
		subdomain = strings.ToLower(subdomain)

		if _, ok := _localServices[subdomain]; !ok {
			return fmt.Errorf("acme/autocert: host not configured")
		}

		return nil
	},
}

	//redirect http
func serveRedirect() error {
	return http.ListenAndServe(":80", makeRedirectHandler())
}

func makeRedirectHandler() http.HandlerFunc{
	return func(w http.ResponseWriter, r *http.Request) {
		target := "https://" + r.Host + r.URL.Path
		if len(r.URL.RawQuery) > 0 {
			target += "?" + r.URL.RawQuery
		}
		fmt.Println("redirecting to: ", target)
		http.Redirect(w, r, target, http.StatusPermanentRedirect)
	}
}

func serveProxy() error {

	s := &http.Server{
		Addr: _proxyPort,
		TLSConfig: &tls.Config{GetCertificate: _tlsManager.GetCertificate},
		Handler: makeProxyHandler(),
	}

	return s.ListenAndServeTLS("","")
}

func makeProxyHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Proto == "wss" {
			proxyWS(w, r)
		}
		_reverseProxy.ServeHTTP(w, r)
	}
}
