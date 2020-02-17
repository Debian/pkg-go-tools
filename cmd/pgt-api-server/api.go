// Binary pgt-api-server exposes functionality for use by Debian go-team members.
package main

import (
	"flag"
	"github.com/Debian/pkg-go-tools/internal/api"
	"github.com/labstack/echo/v4"
	"golang.org/x/crypto/acme/autocert"
	"os"
	"time"

	gitlab "github.com/xanzy/go-gitlab"
)

const (
	salsaUrl = "https://salsa.debian.org/api/v4"
	host     = "pgt-api-server.debian.net"
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

// Return a configured gitlab client that point to Salsa
func salsaClient() *gitlab.Client {
	cl := gitlab.NewClient(nil, os.Getenv("SALSA_TOKEN"))
	_ = cl.SetBaseURL(salsaUrl)
	return cl
}

func main() {
	flag.Parse()

	e := echo.New()

	// TODO Configure logger

	// Configure automatic certificate generation
	e.AutoTLSManager.Cache = autocert.DirCache(*certCacheDir)
	e.AutoTLSManager.HostPolicy = autocert.HostWhitelist(host)

	// Create app context
	ac := api.NewAppContext(salsaClient(), *repoCreation)

	// TODO: prometheus metrics
	// can be integrated using https://echo.labstack.com/middleware/prometheus

	// Create rate limit middleware and register it for all endpoints
	rlm := api.NewRateLimitMiddleware()
	e.Use(rlm.Middleware)
	go rlm.Update(1 * time.Second)

	// Register endpoints
	e.POST("/v1/createrepo", api.CreateRepoHandler(ac), api.RepoInfoMiddleware())
	e.POST("/v1/configrepo", api.ConfigRepoHandler(ac), api.RepoInfoMiddleware())

	e.Logger.Infof("Listening on %s", *listen)

	e.Logger.Fatal(e.StartAutoTLS(*listen))
}
