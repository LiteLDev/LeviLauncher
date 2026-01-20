package client

import (
	"fmt"
	"net/url"
)

type ApiQueryParams map[string]any

type ApiQueryOption func(ApiQueryParams)

func PageIndex(index int64) ApiQueryOption {
	return func(q ApiQueryParams) {
		q["index"] = fmt.Sprintf("%d", index)
	}
}

func PageSize(size int64) ApiQueryOption {
	return func(q ApiQueryParams) {
		q["pageSize"] = fmt.Sprintf("%d", size)
	}
}

type MinecraftVersionsQueryOption ApiQueryOption

func WithSortDescending(descending bool) MinecraftVersionsQueryOption {
	return MinecraftVersionsQueryOption(func(m ApiQueryParams) {
		m["sortDescending"] = descending
	})
}

type MinecraftModLoadersQueryOption ApiQueryOption

func WithMinecraftVersion(version string) MinecraftModLoadersQueryOption {
	return MinecraftModLoadersQueryOption(func(m ApiQueryParams) {
		m["version"] = version
	})
}

func WithIncludeAll(b bool) MinecraftModLoadersQueryOption {
	return MinecraftModLoadersQueryOption(func(m ApiQueryParams) {
		m["includeAll"] = b
	})
}

type ModsQueryOption ApiQueryOption

func WithGameID(g string) ModsQueryOption {
	return ModsQueryOption(func(m ApiQueryParams) {
		m["gameId"] = g
	})
}

func WithModsSeatchFilter(q string) ModsQueryOption {
	return ModsQueryOption(func(m ApiQueryParams) {
		m["searchFilter"] = q
	})
}

func WithModsModLoaderType(mlt int) ModsQueryOption {
	return ModsQueryOption(func(m ApiQueryParams) {
		if mlt == 0 {
			return
		}
		m["modLoaderType"] = mlt
	})
}

func WithModsGameVersion(gv string) ModsQueryOption {
	return ModsQueryOption(func(m ApiQueryParams) {
		m["gameVersion"] = gv
	})
}

func WithModsGameID(id string) ModsQueryOption {
	return ModsQueryOption(func(m ApiQueryParams) {
		m["gameId"] = id
	})
}

func WithModsClassID(cid string) ModsQueryOption {
	return ModsQueryOption(func(m ApiQueryParams) {
		m["classId"] = cid
	})
}

func WithModsCategoryID(cid string) ModsQueryOption {
	return ModsQueryOption(func(m ApiQueryParams) {
		m["categoryId"] = cid
	})
}

func WithModsCategoryIDs(cids string) ModsQueryOption {
	return ModsQueryOption(func(m ApiQueryParams) {
		m["categoryIds"] = cids
	})
}

func WithModsSortField(field int) ModsQueryOption {
	return ModsQueryOption(func(m ApiQueryParams) {
		m["sortField"] = fmt.Sprintf("%d", field)
	})
}

func WithModsSortOrder(order string) ModsQueryOption {
	return ModsQueryOption(func(m ApiQueryParams) {
		m["sortOrder"] = order
	})
}

func WithModsPageSize(pageSize int64) ModsQueryOption {
	return ModsQueryOption(PageSize(pageSize))
}

func WithModsIndex(index int64) ModsQueryOption {
	return ModsQueryOption(PageIndex(index))
}

func (f ApiQueryParams) QueryString() string {
	v, _ := url.ParseQuery("")
	for key, value := range f {
		v.Set(key, fmt.Sprintf("%v", value))
	}
	return v.Encode()
}

type GetModsByIdsListRequest struct {
	ModIds       []int64 `json:"modIds"`
	FilterPcOnly bool    `json:"filterPcOnly"`
}
