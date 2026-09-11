//go:build windows

package xbox

import (
	"fmt"
	"net/url"
	"strconv"
)

type profileResponse struct {
	ProfileUsers []struct {
		ID       string `json:"id"`
		Settings []struct {
			ID    string `json:"id"`
			Value string `json:"value"`
		} `json:"settings"`
	} `json:"profileUsers"`
}

func (p profileResponse) setting(id string) string {
	if len(p.ProfileUsers) == 0 {
		return ""
	}
	for _, s := range p.ProfileUsers[0].Settings {
		if s.ID == id {
			return s.Value
		}
	}
	return ""
}

func profileSettings(s *session, xuid uint64, settings string) (profileResponse, error) {
	var pr profileResponse
	u := fmt.Sprintf("https://profile.xboxlive.com/users/xuid(%d)/profile/settings?settings=%s", xuid, settings)
	if err := getJSON(u, "3", s.authHeader(), &pr); err != nil {
		return pr, err
	}
	return pr, nil
}

// fnv1_64 matches the FNV-1 (multiply then xor) hash used to key gamertags.
func fnv1_64(b []byte) uint64 {
	const offset = 14695981039346656037
	const prime = 1099511628211
	h := uint64(offset)
	for _, c := range b {
		h *= prime
		h ^= uint64(c)
	}
	return h
}

func GetGamertagByXuid(xuid uint64) (string, error) {
	s, err := ensureSession()
	if err != nil {
		return "", err
	}
	pr, err := profileSettings(s, xuid, "Gamertag")
	if err != nil {
		return "", err
	}
	gamertag := pr.setting("Gamertag")
	if gamertag == "" {
		return "", fmt.Errorf("ERR_XBL_GAMERTAG_NOT_FOUND")
	}
	hash := fnv1_64([]byte(strconv.FormatUint(xuid, 10)))
	return gamertag + "|" + strconv.FormatUint(hash, 10), nil
}

var gamerPicturePixels = map[int]int{0: 64, 1: 208, 2: 424, 3: 1080}

func GetLocalUserGamerPicture(size int) ([]byte, error) {
	s, err := ensureSession()
	if err != nil {
		return nil, err
	}
	xuid, err := strconv.ParseUint(s.xuid, 10, 64)
	if err != nil {
		return nil, err
	}
	pr, err := profileSettings(s, xuid, "GameDisplayPicRaw")
	if err != nil {
		return nil, err
	}
	raw := pr.setting("GameDisplayPicRaw")
	if raw == "" {
		return nil, fmt.Errorf("ERR_XBL_GAMERPIC_NOT_FOUND")
	}
	return getBytes(withImageSize(raw, size), "")
}

func withImageSize(raw string, size int) string {
	px, ok := gamerPicturePixels[size]
	if !ok {
		px = gamerPicturePixels[1]
	}
	u, err := url.Parse(raw)
	if err != nil {
		return raw
	}
	q := u.Query()
	q.Set("format", "png")
	q.Set("w", strconv.Itoa(px))
	q.Set("h", strconv.Itoa(px))
	u.RawQuery = q.Encode()
	return u.String()
}
