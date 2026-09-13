package uwpdownload

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"io"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

//go:embed versions.json
var catalogJSON string

var versionPattern = regexp.MustCompile(`^\d{1,5}\.\d{1,5}\.\d{1,5}\.\d{1,5}$`)
var updateIDPattern = regexp.MustCompile(`(?i)^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)

type Version struct {
	Version     string `json:"version"`
	UUID        string `json:"uuid"`
	Type        string `json:"type"`
	PackageType string `json:"packageType"`
}

func ValidChannel(channel string) bool {
	return channel == "release" || channel == "beta" || channel == "preview"
}

func Filename(version, channel string) (string, error) {
	if !versionPattern.MatchString(version) || !ValidChannel(channel) {
		return "", fmt.Errorf("ERR_UWP_INVALID_VERSION")
	}
	label := map[string]string{"release": "Release", "beta": "Beta", "preview": "Preview"}[channel]
	return "Minecraft-UWP-" + label + "-" + version + ".appx", nil
}

// ParseCatalog preserves the old Beta channel: it uses the retail package
// identity, unlike the distinct Preview package introduced later.
func ParseCatalog(r io.Reader) ([]Version, error) {
	var rows [][]json.RawMessage
	decoder := json.NewDecoder(io.LimitReader(r, 4<<20))
	if err := decoder.Decode(&rows); err != nil {
		return nil, fmt.Errorf("decode UWP catalog: %w", err)
	}
	if err := decoder.Decode(new(any)); err != io.EOF {
		return nil, fmt.Errorf("unexpected trailing catalog data")
	}
	result := make([]Version, 0, len(rows))
	seen := make(map[string]bool)
	for i, row := range rows {
		if len(row) != 3 {
			return nil, fmt.Errorf("invalid UWP catalog row %d", i)
		}
		var version, id string
		var kind int
		if json.Unmarshal(row[0], &version) != nil || json.Unmarshal(row[1], &id) != nil || json.Unmarshal(row[2], &kind) != nil || !versionPattern.MatchString(version) || !updateIDPattern.MatchString(id) || kind < 0 || kind > 2 {
			return nil, fmt.Errorf("invalid UWP catalog row %d", i)
		}
		channel := []string{"release", "beta", "preview"}[kind]
		key := channel + ":" + version
		if seen[key] {
			continue
		}
		seen[key] = true
		result = append(result, Version{Version: version, UUID: strings.ToLower(id), Type: channel, PackageType: "uwp"})
	}
	sort.SliceStable(result, func(i, j int) bool {
		a, b := strings.Split(result[i].Version, "."), strings.Split(result[j].Version, ".")
		for part := range a {
			av, _ := strconv.Atoi(a[part])
			bv, _ := strconv.Atoi(b[part])
			if av != bv {
				return av > bv
			}
		}
		return result[i].Type < result[j].Type
	})
	return result, nil
}

func LoadCatalog() ([]Version, error) {
	return ParseCatalog(strings.NewReader(catalogJSON))
}
