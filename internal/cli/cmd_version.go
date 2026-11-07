package cli

import (
	"io"

	"github.com/liteldev/LeviLauncher/internal/update"
)

const (
	repoURL    = "https://github.com/LiteLDev/LeviLauncher"
	issuesURL  = repoURL + "/issues"
	licenseSPD = "GPL-3.0-only"
)

type versionResult struct {
	Version string `json:"version"`
	Beta    bool   `json:"beta"`
	Repo    string `json:"repo"`
	Issues  string `json:"issues"`
	License string `json:"license"`
}

func init() {
	Register(&Command{
		Path:        "version",
		Summary:     "print version information",
		Examples:    []string{"version", "version -q", "--version"},
		Group:       GroupSystem,
		NoBootstrap: true,
		Run: func(c *Ctx) error {
			r := versionResult{
				Version: update.GetAppVersion(), Beta: update.IsBeta(),
				Repo: repoURL, Issues: issuesURL, License: licenseSPD,
			}
			return c.Render(r, []string{r.Version}, func(w io.Writer) error {
				build := ""
				if r.Beta {
					build = " (beta build)"
				}
				c.Printf("LeviLauncher %s%s\n", r.Version, build)
				return KV(w, "Repository", r.Repo, "Issues", r.Issues, "License", r.License)
			})
		},
	})
}
