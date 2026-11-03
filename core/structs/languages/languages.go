package languages

import (
	"regexp"
	"strings"
)

type Language = string

const (
	EN Language = "en"
	ZH Language = "zh"
)

var connectSignRegexp = regexp.MustCompile(`[-_~~\s]+`)

func Parse(s string) (Language, bool) {
	clean := strings.ToLower(connectSignRegexp.ReplaceAllString(s, ""))

	switch clean {
	case string(EN), "eng", "english":
		return EN, true
	case string(ZH), "chinese", "zhtw", "zhcn", "zhhk":
		return ZH, true
	default:
		return EN, false
	}
}
