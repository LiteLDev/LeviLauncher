package packages

import (
	"fmt"
	"sync"
)

type PackType int

const (
	PackTypeInvalid       PackType = 0
	PackTypeAddon         PackType = 1
	PackTypeCached        PackType = 2
	PackTypeCopyProtected PackType = 3
	PackTypeBehavior      PackType = 4
	PackTypePersonaPiece  PackType = 5
	PackTypeResources     PackType = 6
	PackTypeSkins         PackType = 7
	PackTypeWorldTemplate PackType = 8
)

type SemVersion struct {
	Major int `json:"major"`
	Minor int `json:"minor"`
	Patch int `json:"patch"`
}

func (v SemVersion) String() string {
	return fmt.Sprintf("%d.%d.%d", v.Major, v.Minor, v.Patch)
}

type BaseGameVersion struct {
	SemVersion
}

type MinEngineVersion struct {
	SemVersion
}

type PackIdVersion struct {
	UUID     string     `json:"uuid"`
	Version  SemVersion `json:"version"`
	PackType PackType   `json:"pack_type"`
}

type PackManifest struct {
	Identity                PackIdVersion    `json:"identity"`
	PackType                PackType         `json:"pack_type"`
	RequiredBaseGameVersion BaseGameVersion  `json:"required_base_game_version"`
	MinEngineVersion        MinEngineVersion `json:"min_engine_version"`
	Name                    string           `json:"name"`
	Description             string           `json:"description"`
	Location                string           `json:"location"`
	PackIconLocation        string           `json:"pack_icon_location"`
}

type Pack struct {
	Manifest PackManifest `json:"manifest"`
	Path     string       `json:"path"`
}

type PackManager struct {
	mu    sync.RWMutex
	packs map[string][]Pack
}

func NewPackManager() *PackManager {
	return &PackManager{
		packs: make(map[string][]Pack),
	}
}

type ResourceLocation string

func (r ResourceLocation) String() string {
	return string(r)
}
