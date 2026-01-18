package types

type SearchPackagesResponse struct {
	PageIndex  int           `json:"pageIndex"`
	TotalPages int           `json:"totalPages"`
	Items      []PackageItem `json:"items"`
}

type PackageItem struct {
	Identifier  string   `json:"identifier"`
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Author      string   `json:"author"`
	Tags        []string `json:"tags"`
	AvatarURL   string   `json:"avatarUrl"`
	ProjectURL  string   `json:"projectUrl"`
	Hotness     int      `json:"hotness"`
	Updated     string   `json:"updated"`
}

type GetPackageResponse struct {
	PackageItem
	Contributors []Contributor `json:"contributors"`
	Versions     []Version     `json:"versions"`
}

type Version struct {
	Version                    string `json:"version"`
	ReleasedAt                 string `json:"releasedAt"`
	Source                     string `json:"source"`
	PackageManager             string `json:"packageManager"`
	PlatformVersionRequirement string `json:"platformVersionRequirement"`
}

type Contributor struct {
	Username      string `json:"username"`
	Contributions int    `json:"contributions"`
}

type ResponseErr struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}
