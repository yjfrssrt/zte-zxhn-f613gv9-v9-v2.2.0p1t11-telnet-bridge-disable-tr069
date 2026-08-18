package main

import (
	"bytes"
	"context"
	"crypto/tls"
	"flag"
	"io"
	"log"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

const maxBodyBytes = 2 << 20

type compatProxy struct {
	upstream      *url.URL
	trimAESSuffix bool
	client        *http.Client
}

func main() {
	listenAddr := flag.String("listen", "127.0.0.1:18080", "loopback listener")
	upstreamText := flag.String("upstream", "https://192.168.1.1:443", "fixed ZTE HTTPS upstream")
	trimAESSuffix := flag.Bool("trim-aes-suffix", false, "forward only complete AES blocks for non-aligned webFacEntry responses")
	flag.Parse()

	if *listenAddr != "127.0.0.1:18080" {
		log.Fatalf("refusing non-loopback listener %q", *listenAddr)
	}

	upstream, err := url.Parse(*upstreamText)
	if err != nil || upstream.Scheme != "https" || upstream.Hostname() != "192.168.1.1" || upstream.Path != "" {
		log.Fatalf("upstream must be https://192.168.1.1[:port]: %v", err)
	}

	transport := &http.Transport{
		Proxy:               nil,
		DisableCompression:  true,
		ForceAttemptHTTP2:   false,
		TLSHandshakeTimeout: 5 * time.Second,
		DialContext: (&net.Dialer{
			Timeout:   5 * time.Second,
			KeepAlive: 15 * time.Second,
		}).DialContext,
		TLSClientConfig: &tls.Config{
			MinVersion:         tls.VersionTLS10,
			InsecureSkipVerify: true, // Fixed private upstream with a self-signed certificate.
		},
	}

	proxy := &compatProxy{
		upstream:      upstream,
		trimAESSuffix: *trimAESSuffix,
		client: &http.Client{
			Transport: transport,
			Timeout:   12 * time.Second,
		},
	}

	server := &http.Server{
		Addr:              *listenAddr,
		Handler:           proxy,
		ReadHeaderTimeout: 5 * time.Second,
		IdleTimeout:       15 * time.Second,
		BaseContext: func(net.Listener) context.Context {
			return context.Background()
		},
	}

	log.Printf("listening on http://%s; fixed upstream %s", *listenAddr, upstream.String())
	log.Fatal(server.ListenAndServe())
}

func (p *compatProxy) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" && r.URL.Path != "/webFac" && r.URL.Path != "/webFacEntry" {
		http.Error(w, "path rejected", http.StatusNotFound)
		return
	}

	requestBody, err := io.ReadAll(io.LimitReader(r.Body, maxBodyBytes+1))
	if err != nil || len(requestBody) > maxBodyBytes {
		http.Error(w, "request body rejected", http.StatusBadRequest)
		return
	}

	target := *p.upstream
	target.Path = r.URL.Path
	target.RawQuery = r.URL.RawQuery
	request, err := http.NewRequestWithContext(r.Context(), r.Method, target.String(), bytes.NewReader(requestBody))
	if err != nil {
		http.Error(w, "request creation failed", http.StatusBadGateway)
		return
	}
	copyHeaders(request.Header, r.Header)
	request.Host = p.upstream.Host
	request.Header.Set("Host", p.upstream.Host)
	request.Header.Set("Connection", "close")

	response, err := p.client.Do(request)
	if err != nil {
		log.Printf("%s %s: upstream ended connection: %v", r.Method, r.URL.Path, err)
		closeWithoutResponse(w)
		return
	}
	defer response.Body.Close()

	responseBody, err := io.ReadAll(io.LimitReader(response.Body, maxBodyBytes+1))
	if err != nil || len(responseBody) > maxBodyBytes {
		closeWithoutResponse(w)
		return
	}

	forwardBody := responseBody
	if p.trimAESSuffix && r.URL.Path == "/webFacEntry" && len(responseBody) >= 16 && len(responseBody)%16 != 0 {
		forwardBody = responseBody[:len(responseBody)-len(responseBody)%16]
	}

	log.Printf("%s %s -> %d raw_len=%d forwarded_len=%d", r.Method, r.URL.Path, response.StatusCode, len(responseBody), len(forwardBody))
	copyHeaders(w.Header(), response.Header)
	w.Header().Del("Transfer-Encoding")
	w.Header().Set("Content-Length", strconv.Itoa(len(forwardBody)))
	w.WriteHeader(response.StatusCode)
	_, _ = w.Write(forwardBody)
}

func copyHeaders(destination, source http.Header) {
	for key, values := range source {
		if isHopByHopHeader(key) {
			continue
		}
		for _, value := range values {
			destination.Add(key, value)
		}
	}
}

func isHopByHopHeader(key string) bool {
	switch strings.ToLower(key) {
	case "connection", "keep-alive", "proxy-authenticate", "proxy-authorization", "te", "trailer", "transfer-encoding", "upgrade":
		return true
	default:
		return false
	}
}

func closeWithoutResponse(w http.ResponseWriter) {
	hijacker, ok := w.(http.Hijacker)
	if !ok {
		return
	}
	connection, _, err := hijacker.Hijack()
	if err == nil {
		_ = connection.Close()
	}
}

func init() {
	log.SetFlags(log.Ldate | log.Ltime | log.Lmicroseconds)
	log.SetPrefix("zte-https-compat: ")
}
