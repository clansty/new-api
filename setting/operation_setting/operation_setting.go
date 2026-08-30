package operation_setting

import "strings"

var DemoSiteEnabled = false
var SelfUseModeEnabled = false

var AutomaticDisableKeywords = []string{
	"Your credit balance is too low",
	"This organization has been disabled.",
	"You exceeded your current quota",
	"Permission denied",
	"The security token included in the request is invalid",
	"Operation not allowed",
	"Your account is not authorized",
}

var AutomaticSkipRetryKeywords = []string{}

func AutomaticDisableKeywordsToString() string {
	return strings.Join(AutomaticDisableKeywords, "\n")
}

func AutomaticDisableKeywordsFromString(s string) {
	AutomaticDisableKeywords = parseKeywords(s)
}

func AutomaticSkipRetryKeywordsToString() string {
	return strings.Join(AutomaticSkipRetryKeywords, "\n")
}

func AutomaticSkipRetryKeywordsFromString(s string) {
	AutomaticSkipRetryKeywords = parseKeywords(s)
}

func parseKeywords(s string) []string {
	keywords := make([]string, 0)
	ak := strings.Split(s, "\n")
	for _, k := range ak {
		k = strings.TrimSpace(k)
		k = strings.ToLower(k)
		if k != "" {
			keywords = append(keywords, k)
		}
	}
	return keywords
}
