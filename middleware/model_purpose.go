package middleware

import (
	"strings"

	relayconstant "github.com/QuantumNous/new-api/relay/constant"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
)

func requestModelPurpose(path string, relayMode int) ratio_setting.ModelPurpose {
	if strings.Contains(path, ":embedContent") || strings.Contains(path, ":batchEmbedContents") {
		return ratio_setting.ModelPurposeEmbedding
	}

	switch relayMode {
	case relayconstant.RelayModeImagesGenerations,
		relayconstant.RelayModeImagesEdits,
		relayconstant.RelayModeMidjourneyImagine,
		relayconstant.RelayModeMidjourneyDescribe,
		relayconstant.RelayModeMidjourneyBlend,
		relayconstant.RelayModeMidjourneyChange,
		relayconstant.RelayModeMidjourneySimpleChange,
		relayconstant.RelayModeMidjourneyAction,
		relayconstant.RelayModeMidjourneyModal,
		relayconstant.RelayModeMidjourneyShorten,
		relayconstant.RelayModeSwapFace,
		relayconstant.RelayModeMidjourneyUpload,
		relayconstant.RelayModeMidjourneyEdits:
		return ratio_setting.ModelPurposeImage
	case relayconstant.RelayModeEmbeddings:
		return ratio_setting.ModelPurposeEmbedding
	case relayconstant.RelayModeAudioSpeech,
		relayconstant.RelayModeAudioTranscription,
		relayconstant.RelayModeAudioTranslation:
		return ratio_setting.ModelPurposeAudio
	case relayconstant.RelayModeRerank:
		return ratio_setting.ModelPurposeRerank
	case relayconstant.RelayModeModerations:
		return ratio_setting.ModelPurposeModeration
	case relayconstant.RelayModeVideoSubmit, relayconstant.RelayModeMidjourneyVideo:
		return ratio_setting.ModelPurposeVideo
	case relayconstant.RelayModeSunoSubmit:
		return ratio_setting.ModelPurposeMusic
	case relayconstant.RelayModeChatCompletions,
		relayconstant.RelayModeCompletions,
		relayconstant.RelayModeEdits,
		relayconstant.RelayModeResponses,
		relayconstant.RelayModeRealtime,
		relayconstant.RelayModeGemini,
		relayconstant.RelayModeResponsesCompact,
		relayconstant.RelayModeClaudeMessages,
		relayconstant.RelayModeAlphaSearch:
		return ratio_setting.ModelPurposeChat
	default:
		return ""
	}
}
