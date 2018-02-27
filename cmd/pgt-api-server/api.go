// Binary pgt-api-server exposes functionality for use by Debian go-team members.
package main

import (
	"crypto/tls"
	"flag"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"golang.org/x/crypto/acme/autocert"

	gitlab "github.com/xanzy/go-gitlab"
)

var (
	listen = flag.String("listen",
		":4039",
		"[host]:port to listen on")

	listenChallenge = flag.String("listen_challenge",
		":80",
		"[host]:port to listen on for the ACME challenge")

	certCacheDir = flag.String("cert_cache_dir",
		"/var/cache/pgt-api-server",
		"LetsEncrypt certificate cache directory")

	repoCreation = flag.Bool("repo_creation",
		true,
		"Whether /v1/createrepo is enabled. Can be flipped quickly to limit abuse, should it happen.")
)

const group = "go-team/packages"

var salsa = salsaClient()

func salsaClient() *gitlab.Client {
	cl := gitlab.NewClient(nil, os.Getenv("SALSA_TOKEN"))
	cl.SetBaseURL("https://salsa.debian.org/api/v4")
	return cl
}

var rateLimit = make(chan struct{})

// internalServerError returns a non-nil error from handler as a HTTP 500 error.
func internalServerError(handler func(http.ResponseWriter, *http.Request) error) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := handler(w, r); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
	})
}

// TODO: misnomer: rename or make an actual apache log
func apacheLog(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		src := r.Header.Get("X-Forwarded-For")
		if src == "" ||
			(!strings.HasPrefix(r.RemoteAddr, "[::1]:") &&
				!strings.HasPrefix(r.RemoteAddr, "127.0.0.1:") &&
				!strings.HasPrefix(r.RemoteAddr, "172.17.")) {
			src = r.RemoteAddr
		}

		start := time.Now()
		h.ServeHTTP(w, r)
		log.Printf("%s %s %s: %v", src, r.Method, r.URL, time.Since(start))
	})
}

func main() {
	log.SetFlags(log.LstdFlags | log.Lshortfile)
	flag.Parse()

	go func() {
		for range time.Tick(1 * time.Second) {
			rateLimit <- struct{}{}
		}
	}()

	// TODO: prometheus metrics

	m := &autocert.Manager{
		Cache:      autocert.DirCache(*certCacheDir),
		Prompt:     autocert.AcceptTOS,
		HostPolicy: autocert.HostWhitelist("pgt-api-server.debian.net"),
	}
	go func() { log.Fatal(http.ListenAndServe(*listenChallenge, m.HTTPHandler(nil))) }()

	// Trigger certificate creation so that we can use the cached certificate in
	// the frontend webserver.
	_, err := m.GetCertificate(&tls.ClientHelloInfo{ServerName: "pgt-api-server.debian.net"})
	if err != nil {
		log.Fatalf("GetCertificate: %v", err)
	}

	http.Handle("/v1/createrepo", apacheLog(internalServerError(createRepo)))
	http.Handle("/v1/configrepo", apacheLog(internalServerError(configRepo)))
	log.Printf("listening on %s", *listen)
	http.ListenAndServe(*listen, nil)
}
