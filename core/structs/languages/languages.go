package languages

import (
	"regexp"
	"strings"

	"github.com/lucap9056/magic-conch-shell/core/v3/structs"
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

func FromProto(lang structs.Language) (Language, bool) {
	switch lang {
	case structs.Language_LANGUAGE_EN:
		return EN, true
	case structs.Language_LANGUAGE_ZH:
		return ZH, true
	default:
		return EN, false
	}
}

func ToProto(lang Language) structs.Language {
	switch lang {
	case EN:
		return structs.Language_LANGUAGE_EN
	case ZH:
		return structs.Language_LANGUAGE_ZH
	default:
		return structs.Language_LANGUAGE_UNSPECIFIED
	}
}
