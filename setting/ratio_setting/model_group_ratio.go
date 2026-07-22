package ratio_setting

import (
	"strings"

	"github.com/QuantumNous/new-api/types"
)

var modelIgnoreGroupSpecialRatioMap = types.NewRWMap[string, bool]()

func ModelIgnoreGroupSpecialRatio2JSONString() string {
	return modelIgnoreGroupSpecialRatioMap.MarshalJSONString()
}

func UpdateModelIgnoreGroupSpecialRatioByJSONString(jsonStr string) error {
	return types.LoadFromJsonString(modelIgnoreGroupSpecialRatioMap, jsonStr)
}

func ShouldIgnoreGroupSpecialRatio(modelName string) bool {
	modelName = FormatMatchingModelName(modelName)
	if ignore, ok := modelIgnoreGroupSpecialRatioMap.Get(modelName); ok {
		return ignore
	}
	if strings.HasSuffix(modelName, CompactModelSuffix) {
		ignore, _ := modelIgnoreGroupSpecialRatioMap.Get(CompactWildcardModelKey)
		return ignore
	}
	return false
}

func GetModelIgnoreGroupSpecialRatioCopy() map[string]bool {
	return modelIgnoreGroupSpecialRatioMap.ReadAll()
}
