package core

import (
	"encoding/json"
	"strings"
	"testing"
)

// The point-name ladder is the contract that keeps a raw TSL identifier such as
// AI_OUTPUT_CURRENT out of the operator UI and keeps each language free of the
// other language's characters. The frontend mirrors it in
// noyo/frontend/src/utils/propertyName.js — keep both in sync.

func TestPointDisplayNameFromProperty_PrefersConfiguredName(t *testing.T) {
	zh, en := pointDisplayNameFromProperty(map[string]any{"name": "应急回路总输出电流"}, "AI_OUTPUT_CURRENT")
	if zh != "应急回路总输出电流" {
		t.Fatalf("zh = %q, want the configured Chinese name", zh)
	}
	// No English label exists, so the English console must not receive Chinese.
	if en != "AI OUTPUT CURRENT" {
		t.Fatalf("en = %q, want the prettified identifier", en)
	}
}

func TestPointDisplayNameFromProperty_ExplicitLanguagePair(t *testing.T) {
	property := map[string]any{
		"name":    "Remote force emergency lighting",
		"name_zh": "远程强制启动应急照明",
	}
	zh, en := pointDisplayNameFromProperty(property, "DO_FORCE_EMERGENCY")
	if zh != "远程强制启动应急照明" {
		t.Fatalf("zh = %q", zh)
	}
	if en != "Remote force emergency lighting" {
		t.Fatalf("en = %q", en)
	}
}

func TestPointDisplayNameFromProperty_BilingualPayloadName(t *testing.T) {
	property := map[string]any{
		"name": BilingualText("蓄电池组电压", "Battery bank voltage"),
	}
	zh, en := pointDisplayNameFromProperty(property, "AI_BATTERY_VOLTAGE")
	if zh != "蓄电池组电压" || en != "Battery bank voltage" {
		t.Fatalf("zh = %q en = %q", zh, en)
	}
}

func TestPointDisplayNameFromProperty_LatinNameReusedForBothLanguages(t *testing.T) {
	zh, en := pointDisplayNameFromProperty(map[string]any{"name": "Battery bank voltage"}, "AI_BATTERY_VOLTAGE")
	if zh != "Battery bank voltage" || en != "Battery bank voltage" {
		t.Fatalf("zh = %q en = %q", zh, en)
	}
}

func TestPointDisplayNameFromProperty_NeverEmpty(t *testing.T) {
	for _, property := range []map[string]any{nil, {}, {"name": "   "}} {
		zh, en := pointDisplayNameFromProperty(property, "AI_OUTPUT_CURRENT")
		if zh == "" || en == "" {
			t.Fatalf("property %v produced zh=%q en=%q", property, zh, en)
		}
	}
}

func TestPrettifyPointCode(t *testing.T) {
	cases := map[string]string{
		"AI_OUTPUT_CURRENT": "AI OUTPUT CURRENT",
		"ai-output-current": "ai output current",
		"  a__b  ":          "a b",
		"":                  "",
	}
	for input, want := range cases {
		if got := PrettifyPointCode(input); got != want {
			t.Errorf("PrettifyPointCode(%q) = %q, want %q", input, got, want)
		}
	}
}

func TestLocalizeAlarmPointCode_BilingualTitle(t *testing.T) {
	title := BilingualTextf("AI 预测维护异常：%s", "AI predictive maintenance: %s", "AI_OUTPUT_CURRENT")
	localized := LocalizeAlarmPointCode(title, "AI_OUTPUT_CURRENT", "应急回路总输出电流", "AI OUTPUT CURRENT")

	var payload map[string]string
	if err := json.Unmarshal([]byte(localized), &payload); err != nil {
		t.Fatalf("localized title is not a bilingual payload: %v (%q)", err, localized)
	}
	if payload[BilingualTextKeyZh] != "AI 预测维护异常：应急回路总输出电流" {
		t.Errorf("zh side = %q", payload[BilingualTextKeyZh])
	}
	if payload[BilingualTextKeyEn] != "AI predictive maintenance: AI OUTPUT CURRENT" {
		t.Errorf("en side = %q", payload[BilingualTextKeyEn])
	}
	if strings.Contains(localized, "AI_OUTPUT_CURRENT") {
		t.Errorf("raw identifier survived localization: %q", localized)
	}
}

func TestLocalizeAlarmPointCode_LegacySingleLanguageTitle(t *testing.T) {
	// Historical rows hold one language only. The rewrite still has to remove the
	// raw identifier from both sides; the sentence itself is translated later by
	// the client's legacy table (noyo/frontend/src/utils/serverText.js), which is
	// anchored to the exact sentences the platform used to emit.
	localized := LocalizeAlarmPointCode(
		"AI predictive maintenance: AI_OUTPUT_CURRENT",
		"AI_OUTPUT_CURRENT",
		"应急回路总输出电流",
		"AI OUTPUT CURRENT",
	)
	if strings.Contains(localized, "AI_OUTPUT_CURRENT") {
		t.Fatalf("raw identifier survived localization: %q", localized)
	}
	var payload map[string]string
	if err := json.Unmarshal([]byte(localized), &payload); err != nil {
		t.Fatalf("legacy title was not promoted to a bilingual payload: %v (%q)", err, localized)
	}
	if payload[BilingualTextKeyZh] != "AI predictive maintenance: 应急回路总输出电流" {
		t.Errorf("zh side = %q", payload[BilingualTextKeyZh])
	}
	if payload[BilingualTextKeyEn] != "AI predictive maintenance: AI OUTPUT CURRENT" {
		t.Errorf("en side = %q", payload[BilingualTextKeyEn])
	}
	// The English side must not carry Chinese characters.
	if hasCJK(payload[BilingualTextKeyEn]) {
		t.Errorf("en side contains CJK: %q", payload[BilingualTextKeyEn])
	}
}

func TestLocalizeAlarmPointCode_LeavesUnrelatedTitlesAlone(t *testing.T) {
	cases := []struct {
		name       string
		title      string
		identifier string
	}{
		{"identifier absent", "AI 预测维护异常：应急回路总输出电流", "AI_OUTPUT_CURRENT"},
		{"empty identifier", "AI predictive maintenance: A", ""},
		{"non payload json", `{"unexpected":"shape"}`, "unexpected"},
	}
	for _, tc := range cases {
		if got := LocalizeAlarmPointCode(tc.title, tc.identifier, "名称", "Name"); got != tc.title {
			t.Errorf("%s: got %q, want %q", tc.name, got, tc.title)
		}
	}
}
