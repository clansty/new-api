package ratio_setting

import (
	"fmt"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/types"
)

type ModelPurpose string

const (
	ModelPurposeChat       ModelPurpose = "chat"
	ModelPurposeImage      ModelPurpose = "image"
	ModelPurposeEmbedding  ModelPurpose = "embedding"
	ModelPurposeAudio      ModelPurpose = "audio"
	ModelPurposeRerank     ModelPurpose = "rerank"
	ModelPurposeModeration ModelPurpose = "moderation"
	ModelPurposeVideo      ModelPurpose = "video"
	ModelPurposeMusic      ModelPurpose = "music"
)

var validModelPurposes = map[ModelPurpose]struct{}{
	ModelPurposeChat: {}, ModelPurposeImage: {}, ModelPurposeEmbedding: {},
	ModelPurposeAudio: {}, ModelPurposeRerank: {}, ModelPurposeModeration: {},
	ModelPurposeVideo: {}, ModelPurposeMusic: {},
}

var defaultModelPurpose = map[string][]ModelPurpose{
	"gpt-image-1":                    {ModelPurposeImage},
	"gpt-image-2":                    {ModelPurposeImage},
	"dall-e":                         {ModelPurposeImage},
	"dall-e-2":                       {ModelPurposeImage},
	"dall-e-3":                       {ModelPurposeImage},
	"imagen-3.0-generate-002":        {ModelPurposeImage},
	"black-forest-labs/flux-1.1-pro": {ModelPurposeImage},
	"mj_imagine":                     {ModelPurposeImage},
	"mj_edits":                       {ModelPurposeImage},
	"mj_variation":                   {ModelPurposeImage},
	"mj_reroll":                      {ModelPurposeImage},
	"mj_blend":                       {ModelPurposeImage},
	"mj_modal":                       {ModelPurposeImage},
	"mj_zoom":                        {ModelPurposeImage},
	"mj_shorten":                     {ModelPurposeImage},
	"mj_high_variation":              {ModelPurposeImage},
	"mj_low_variation":               {ModelPurposeImage},
	"mj_pan":                         {ModelPurposeImage},
	"mj_inpaint":                     {ModelPurposeImage},
	"mj_custom_zoom":                 {ModelPurposeImage},
	"mj_describe":                    {ModelPurposeImage},
	"mj_upscale":                     {ModelPurposeImage},
	"swap_face":                      {ModelPurposeImage},
	"mj_upload":                      {ModelPurposeImage},
	"text-embedding-3-small":         {ModelPurposeEmbedding},
	"text-embedding-3-large":         {ModelPurposeEmbedding},
	"text-embedding-ada-002":         {ModelPurposeEmbedding},
	"gemini-embedding-001":           {ModelPurposeEmbedding},
	"text-embedding-004":             {ModelPurposeEmbedding},
	"text-embedding-v1":              {ModelPurposeEmbedding},
	"embedding-bert-512-v1":          {ModelPurposeEmbedding},
	"embedding_s1_v1":                {ModelPurposeEmbedding},
	"text-moderation-stable":         {ModelPurposeModeration},
	"text-moderation-latest":         {ModelPurposeModeration},
	"whisper-1":                      {ModelPurposeAudio},
	"tts-1":                          {ModelPurposeAudio},
	"tts-1-1106":                     {ModelPurposeAudio},
	"tts-1-hd":                       {ModelPurposeAudio},
	"tts-1-hd-1106":                  {ModelPurposeAudio},
	"gpt-4o-mini-tts":                {ModelPurposeAudio},
	"sora-2":                         {ModelPurposeVideo},
	"sora-2-pro":                     {ModelPurposeVideo},
	"veo-3.0-generate-001":           {ModelPurposeVideo},
	"veo-3.0-fast-generate-001":      {ModelPurposeVideo},
	"veo-3.1-generate-preview":       {ModelPurposeVideo},
	"veo-3.1-fast-generate-preview":  {ModelPurposeVideo},
	"mj_video":                       {ModelPurposeVideo},
	"suno_music":                     {ModelPurposeMusic},
	"suno_lyrics":                    {ModelPurposeMusic},
}

var modelPurposeMap = types.NewRWMap[string, []ModelPurpose]()

func ModelPurpose2JSONString() string {
	return modelPurposeMap.MarshalJSONString()
}

func ValidateModelPurposeJSONString(jsonStr string) error {
	_, err := parseModelPurposeJSONString(jsonStr)
	return err
}

func UpdateModelPurposeByJSONString(jsonStr string) error {
	parsed, err := parseModelPurposeJSONString(jsonStr)
	if err != nil {
		return err
	}
	modelPurposeMap.Clear()
	modelPurposeMap.AddAll(parsed)
	InvalidateExposedDataCache()
	return nil
}

func GetModelPurposeCopy() map[string][]ModelPurpose {
	return modelPurposeMap.ReadAll()
}

func IsModelPurposeAllowed(model string, purpose ModelPurpose) bool {
	model = strings.TrimSuffix(model, CompactModelSuffix)
	models := []string{model, FormatMatchingModelName(model)}
	for _, candidate := range models {
		purposes, configured := modelPurposeMap.Get(candidate)
		if !configured {
			continue
		}
		for _, configuredPurpose := range purposes {
			if configuredPurpose == purpose {
				return true
			}
		}
		return false
	}
	return true
}

func parseModelPurposeJSONString(jsonStr string) (map[string][]ModelPurpose, error) {
	var raw map[string][]string
	if err := common.UnmarshalJsonStr(jsonStr, &raw); err != nil {
		return nil, err
	}
	parsed := make(map[string][]ModelPurpose, len(raw))
	for model, rawPurposes := range raw {
		if strings.TrimSpace(model) == "" {
			return nil, fmt.Errorf("模型名称不能为空")
		}
		if len(rawPurposes) == 0 {
			return nil, fmt.Errorf("模型 %s 至少需要一个用途", model)
		}
		seen := make(map[ModelPurpose]struct{}, len(rawPurposes))
		purposes := make([]ModelPurpose, 0, len(rawPurposes))
		for _, rawPurpose := range rawPurposes {
			purpose := ModelPurpose(rawPurpose)
			if _, ok := validModelPurposes[purpose]; !ok {
				return nil, fmt.Errorf("模型 %s 包含无效用途 %s", model, rawPurpose)
			}
			if _, duplicated := seen[purpose]; duplicated {
				continue
			}
			seen[purpose] = struct{}{}
			purposes = append(purposes, purpose)
		}
		parsed[model] = purposes
	}
	return parsed, nil
}
