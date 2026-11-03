package languages

import (
	"testing"

	"github.com/lucap9056/magic-conch-shell/core/v3/structs"
)

func TestProtoRoundTrip(t *testing.T) {
	for _, lang := range []Language{EN, ZH} {
		got, ok := FromProto(ToProto(lang))
		if !ok || got != lang {
			t.Errorf("FromProto(ToProto(%q)) = (%q, %v)", lang, got, ok)
		}
	}
}

func TestFromProtoUnspecified(t *testing.T) {
	for _, lang := range []structs.Language{structs.Language_LANGUAGE_UNSPECIFIED, structs.Language(99)} {
		if got, ok := FromProto(lang); ok || got != EN {
			t.Errorf("FromProto(%v) = (%q, %v), want (en, false)", lang, got, ok)
		}
	}
}

func TestToProtoUnknown(t *testing.T) {
	if got := ToProto("fr"); got != structs.Language_LANGUAGE_UNSPECIFIED {
		t.Errorf("ToProto(fr) = %v, want UNSPECIFIED", got)
	}
}
