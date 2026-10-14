package api

import (
	"strings"
	"testing"
)

func TestValidateSceneMeta(t *testing.T) {
	tests := []struct {
		name        string
		sceneName   string
		description string
		wantErr     bool
	}{
		{"empty name and description", "", "", false},
		{"name at limit", strings.Repeat("a", 64), "", false},
		{"name over limit", strings.Repeat("a", 65), "", true},
		{"description at limit", "", strings.Repeat("d", 256), false},
		{"description over limit", "", strings.Repeat("d", 257), true},
		{"both at limit", strings.Repeat("a", 64), strings.Repeat("d", 256), false},
		{"cjk name at limit", strings.Repeat("压", 64), "", false},
		{"cjk name over limit", strings.Repeat("压", 65), "", true},
		{"cjk description at limit", "", strings.Repeat("测", 256), false},
		{"cjk description over limit", "", strings.Repeat("测", 257), true},
		// Regression: 64 UTF-16 code units (frontend maxlength) with CJK chars
		// must pass; len() in bytes would be far above 64.
		{"mixed cjk name accepted", "Mock API 电商全链路压测 V2（新版功能演示）1111111111222222222222222222222222222", "", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateSceneMeta(tt.sceneName, tt.description)
			if (err != nil) != tt.wantErr {
				t.Fatalf("validateSceneMeta(%d runes name, %d runes description) error = %v, wantErr %v",
					len([]rune(tt.sceneName)), len([]rune(tt.description)), err, tt.wantErr)
			}
		})
	}
}

func TestValidateNodeNameRunes(t *testing.T) {
	if err := validateNodeName(strings.Repeat("节", 50)); err != nil {
		t.Fatalf("50 cjk runes should pass, got: %v", err)
	}
	if err := validateNodeName(strings.Repeat("节", 51)); err == nil {
		t.Fatal("51 cjk runes should fail")
	}
}
