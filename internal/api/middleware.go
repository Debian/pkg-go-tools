package api

import (
	"github.com/labstack/echo/v4"
	"net/http"
	"path"
	"strings"
	"time"
)

const repoKey = "repo"

// Middleware used to extract repo information from request and set them in echo context
func RepoInfoMiddleware() echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			repo := c.FormValue("repo")
			if repo == "" {
				return c.String(http.StatusBadRequest, `no "repo" parameter found`)
			}
			repo = path.Clean(repo)
			if strings.Contains(repo, "/") {
				return c.String(http.StatusBadRequest, `repo must not contain slashes`)
			}

			// Set repo name in echo context
			c.Set(repoKey, repo)

			return next(c)
		}
	}
}

// Middleware used to rate limit request
// TODO: use something better
type RateLimitMiddleware struct {
	rateLimit chan struct{}
}

func NewRateLimitMiddleware() *RateLimitMiddleware {
	return &RateLimitMiddleware{
		rateLimit: make(chan struct{}),
	}
}

func (rlm *RateLimitMiddleware) Middleware(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c echo.Context) error {

		// Wait for rateLimit to complete
		<-rlm.rateLimit

		return next(c)
	}
}

// Start the routine to update rateLimit
// Since blocking function, should be start in a separate goroutine
func (rlm *RateLimitMiddleware) Update(d time.Duration) {
	for range time.Tick(d) {
		rlm.rateLimit <- struct{}{}
	}
}
