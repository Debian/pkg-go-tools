package api

import (
	"fmt"
	"github.com/labstack/echo/v4"
	"net/http"
	"salsa.debian.org/go-team/ci/config"
)

// configRepo configures the specified repo underneath go-team/packages with
// go-team-wide settings (CI, webhooks, etc.).
func ConfigRepoHandler(ac *AppContext) echo.HandlerFunc {
	return func(c echo.Context) error {
		repo := c.Get(repoKey).(string)

		// TODO: log repo names in an auditlog

		p, err := ac.GetGoProject(repo)
		if err != nil {
			return err
		}

		if err := config.All(p); err != nil {
			return c.String(http.StatusInternalServerError, err.Error())
		}

		return c.NoContent(http.StatusOK)
	}
}

// createRepo creates a new repository underneath go-team/packages on
// salsa.debian.org.
func CreateRepoHandler(ac *AppContext) echo.HandlerFunc {
	return func(c echo.Context) error {
		if !ac.repoCreation {
			return c.String(http.StatusForbidden, "repository creation is disabled by the administrator; please see the mailing list")
		}

		// Get repo name from echo context
		repo := c.Get(repoKey).(string)

		// TODO: log repo names in an auditlog

		// Create the repository on Salsa
		if _, err := ac.CreateGoProject(repo); err != nil {
			return c.String(http.StatusInternalServerError, fmt.Sprintf("CreateProject(%q): %v", repo, err))
		}

		return c.NoContent(http.StatusCreated)
	}
}
