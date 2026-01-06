package lang

import (
	_ "embed"

	json "github.com/goccy/go-json"
	"github.com/liteldev/LeviLauncher/internal/types"
)

var (
	//go:embed language_names.json
	langNames []byte
)

func GetLanguageNames() []types.LanguageJson {

	var rawLanguageNames [][]interface{}

	err := json.Unmarshal(langNames, &rawLanguageNames)
	if err != nil {
		return []types.LanguageJson{}
	}

	var LanguageNames []types.LanguageJson

	for _, v := range rawLanguageNames {
		LanguageNames = append(LanguageNames, types.LanguageJson{
			Code:     v[0].(string),
			Language: v[1].(string),
		})
	}
	return LanguageNames
}
