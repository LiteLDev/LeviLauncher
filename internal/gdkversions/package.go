package gdkversions

import (
	"fmt"
	"net/url"
	"path"
	"strconv"
	"strings"
	"time"
)

// Wire contract and content IDs follow:
// https://github.com/LiteLDev/minecraft-windows-gdk-version-db/blob/main/getLatestVersion.ts
type packageResponse struct {
	PackageFound bool
	PackageFiles []struct {
		FileName     string
		CdnRootPaths []string
		RelativeURL  string `json:"RelativeUrl"`
	}
}

func (p packageResponse) version(channel string) (Version, error) {
	if !p.PackageFound {
		return Version{}, fmt.Errorf("package not found")
	}
	for _, file := range p.PackageFiles {
		parts := strings.Split(file.FileName, "_")
		identity := "Microsoft.MinecraftUWP"
		if channel == "Preview" {
			identity = "Microsoft.MinecraftWindowsBeta"
		}
		if len(parts) != 5 || !strings.EqualFold(parts[0], identity) ||
			!strings.EqualFold(parts[2], "x64") || !strings.EqualFold(parts[4], "8wekyb3d8bbwe.msixvc") {
			continue
		}
		numbers, ok := numericVersion(parts[1])
		if !ok {
			continue
		}
		relative, err := url.Parse(file.RelativeURL)
		if err != nil || relative.IsAbs() || relative.Host != "" || relative.RawQuery != "" || relative.Fragment != "" ||
			path.Base(relative.Path) != file.FileName {
			continue
		}
		var urls []string
		seen := map[string]bool{}
		for _, root := range file.CdnRootPaths {
			base, err := url.Parse(strings.TrimSpace(root))
			if err != nil || !validDownloadURL(base) || base.RawQuery != "" || base.Fragment != "" {
				continue
			}
			// The service returns CDN prefixes; keep any prefix path when joining.
			candidate := strings.TrimRight(base.String(), "/") + "/" + strings.TrimLeft(file.RelativeURL, "/")
			if !seen[candidate] {
				seen[candidate] = true
				urls = append(urls, candidate)
			}
		}
		if len(urls) == 0 {
			continue
		}
		// The third package component packs the game's patch and revision:
		// 1.26.4501.0 -> 1.26.45.01, matching historical_versions.json.
		version := fmt.Sprintf("%s %d.%d.%d.%02d", channel, numbers[0], numbers[1], numbers[2]/100, numbers[2]%100)
		// The database timestamps discovery, not package availability. Follow the
		// same rule so the download page's timestamp sort puts new entries first.
		// FileHash is not the database's verified MD5; leave MD5 empty.
		return Version{Version: version, URLs: urls, Timestamp: time.Now().Unix()}, nil
	}
	return Version{}, fmt.Errorf("no downloadable x64 Minecraft MSIXVC package")
}

func validDownloadURL(u *url.URL) bool {
	return (u.Scheme == "https" || u.Scheme == "http") && u.Hostname() != "" && u.User == nil
}

func numericVersion(raw string) ([4]uint64, bool) {
	var numbers [4]uint64
	parts := strings.Split(raw, ".")
	if len(parts) != len(numbers) {
		return numbers, false
	}
	for i, part := range parts {
		if part == "" || strings.IndexFunc(part, func(r rune) bool { return r < '0' || r > '9' }) >= 0 {
			return numbers, false
		}
		value, err := strconv.ParseUint(part, 10, 16)
		if err != nil {
			return numbers, false
		}
		numbers[i] = value
	}
	return numbers, true
}

func versionKey(version string) string {
	parts := strings.Fields(version)
	if len(parts) == 2 {
		if numbers, ok := numericVersion(parts[1]); ok {
			return fmt.Sprintf("%s %d.%d.%d.%d", strings.ToLower(parts[0]), numbers[0], numbers[1], numbers[2], numbers[3])
		}
	}
	return strings.ToLower(strings.TrimSpace(version))
}
