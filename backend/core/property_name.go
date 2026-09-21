package core

import (
	"encoding/json"
	"strings"
	"unicode"

	"noyo/core/store"
)

// Monitored-point display names
// -----------------------------
// Plugins that raise an alarm about a single telemetry point identify it by its
// point identifier (a TSL property identifier, e.g. "AI_OUTPUT_CURRENT"). An
// identifier is machine-facing: it must never be the text an operator reads,
// because a raw code is unreadable in any language and carries no localizable
// content at all. The human readable label lives in the product TSL
// (products.config.tsl.properties[].name) and may carry an explicit per-language
// pair through name_zh / name_en — the convention already used by the built-in
// GB28181 product model.
//
// Resolution ladder, per language:
//
//	zh: name_zh  ->  name (Chinese)  ->  name (bilingual payload)  ->  prettified identifier
//	en: name_en  ->  name (Latin)    ->  name (bilingual payload)  ->  prettified identifier
//
// The rule guarantees two things that matter for a mixed-language console:
// neither language ever renders the other language's characters, and a raw
// underscore identifier never reaches the operator. When a product author
// supplies name_en, the English console automatically shows the proper English
// label instead of the prettified fallback.

// PrettifyPointCode turns a point identifier into readable text by dropping the
// separators: "AI_OUTPUT_CURRENT" -> "AI OUTPUT CURRENT".
func PrettifyPointCode(code string) string {
	code = strings.TrimSpace(code)
	if code == "" {
		return ""
	}
	replacing := strings.NewReplacer("_", " ", "-", " ", ".", " ")
	return strings.Join(strings.Fields(replacing.Replace(code)), " ")
}

// hasCJK reports whether the text contains any CJK character. Used to decide
// whether a single-language product label may be reused for the English side.
func hasCJK(value string) bool {
	for _, r := range value {
		if unicode.Is(unicode.Han, r) || unicode.Is(unicode.Hiragana, r) || unicode.Is(unicode.Katakana, r) {
			return true
		}
	}
	return false
}

// splitBilingualPayload accepts a {"zh":..,"en":..} payload (the wire format
// produced by BilingualText) and returns both sides.
func splitBilingualPayload(value string) (zh, en string, ok bool) {
	text := strings.TrimSpace(value)
	if !strings.HasPrefix(text, "{") || !strings.Contains(text, "\"") {
		return "", "", false
	}
	var payload map[string]string
	if err := json.Unmarshal([]byte(text), &payload); err != nil {
		return "", "", false
	}
	zh = strings.TrimSpace(payload[BilingualTextKeyZh])
	en = strings.TrimSpace(payload[BilingualTextKeyEn])
	if zh == "" && en == "" {
		return "", "", false
	}
	return zh, en, true
}

func propertyStringField(property map[string]any, key string) string {
	if property == nil {
		return ""
	}
	value, ok := property[key].(string)
	if !ok {
		return ""
	}
	return strings.TrimSpace(value)
}

// firstNonEmptyText returns the first value with content. It deliberately does
// not reuse firstNonBlank (alarm_center_service.go), whose fallback is the
// hardcoded word "Alarm" and would leak into point names.
func firstNonEmptyText(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}

// pointDisplayNameFromProperty resolves one TSL property entry into its
// per-language display name. It always returns two non-empty strings whenever
// the identifier itself is non-empty.
func pointDisplayNameFromProperty(property map[string]any, identifier string) (string, string) {
	fallback := PrettifyPointCode(identifier)
	if fallback == "" {
		fallback = strings.TrimSpace(identifier)
	}

	name := propertyStringField(property, "name")
	payloadZh, payloadEn, isPayload := splitBilingualPayload(name)

	zh := firstNonEmptyText(propertyStringField(property, "name_zh"), payloadZh)
	en := firstNonEmptyText(propertyStringField(property, "name_en"), payloadEn)

	// `name` is the product-authored label. It is the Chinese copy whenever the
	// author wrote it in Chinese, and is only reusable for English when it is
	// already Latin text — otherwise the English console falls back to the
	// prettified identifier rather than leaking Chinese characters.
	if !isPayload {
		if zh == "" {
			zh = name
		}
		if en == "" && name != "" && !hasCJK(name) {
			en = name
		}
	}

	return firstNonEmptyText(zh, fallback), firstNonEmptyText(en, fallback)
}

func loadProductTSLProperties(productCode string) []map[string]any {
	code := strings.TrimSpace(productCode)
	if code == "" {
		return nil
	}
	product, err := store.GetProduct(code)
	if err != nil || product == nil || strings.TrimSpace(product.Config) == "" {
		return nil
	}
	var config map[string]any
	if err := json.Unmarshal([]byte(product.Config), &config); err != nil {
		return nil
	}
	tsl := config
	if nested, ok := config["tsl"].(map[string]any); ok {
		tsl = nested
	}
	raw, ok := tsl["properties"].([]any)
	if !ok {
		return nil
	}
	properties := make([]map[string]any, 0, len(raw))
	for _, item := range raw {
		if property, ok := item.(map[string]any); ok {
			properties = append(properties, property)
		}
	}
	return properties
}

// ResolvePointDisplayName maps a monitored point identifier onto the label the
// product author configured for it, in both languages. It never returns an empty
// pair unless the identifier is empty: when the point is absent from the product
// TSL the prettified identifier is used, so a raw code still never reaches the UI.
func ResolvePointDisplayName(productCode, identifier string) (string, string) {
	identifier = strings.TrimSpace(identifier)
	if identifier == "" {
		return "", ""
	}
	for _, property := range loadProductTSLProperties(productCode) {
		if !strings.EqualFold(propertyStringField(property, "identifier"), identifier) {
			continue
		}
		return pointDisplayNameFromProperty(property, identifier)
	}
	fallback := PrettifyPointCode(identifier)
	if fallback == "" {
		fallback = identifier
	}
	return fallback, fallback
}

// LocalizeAlarmPointCode rewrites a point identifier embedded in an alarm title
// into its display name for every language the title carries. A single-language
// title is promoted to a bilingual payload so the historical row can be rendered
// in either console language; a non-payload JSON title is left untouched.
func LocalizeAlarmPointCode(title, identifier, zh, en string) string {
	identifier = strings.TrimSpace(identifier)
	if identifier == "" || title == "" || !strings.Contains(title, identifier) {
		return title
	}
	if zh == "" && en == "" {
		return title
	}
	if zh == "" {
		zh = en
	}
	if en == "" {
		en = zh
	}

	if payloadZh, payloadEn, ok := splitBilingualPayload(title); ok {
		var payload map[string]string
		_ = json.Unmarshal([]byte(strings.TrimSpace(title)), &payload)
		payload[BilingualTextKeyZh] = strings.ReplaceAll(payloadZh, identifier, zh)
		payload[BilingualTextKeyEn] = strings.ReplaceAll(payloadEn, identifier, en)
		if encoded, err := json.Marshal(payload); err == nil {
			return string(encoded)
		}
		return title
	}

	if strings.HasPrefix(strings.TrimSpace(title), "{") {
		// JSON-looking but not a bilingual payload: refuse to nest it.
		return title
	}

	return BilingualText(
		strings.ReplaceAll(title, identifier, zh),
		strings.ReplaceAll(title, identifier, en),
	)
}
