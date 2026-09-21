package core

import (
	"encoding/json"
	"fmt"
	"strings"
)

// BilingualTextKeyZh / BilingualTextKeyEn are the locale keys carried by every
// BilingualText payload. They intentionally match the key set already used by
// plugin metadata and schema field titles (map[string]string{"en":..., "zh":...})
// so that clients keep a single localization convention.
const (
	BilingualTextKeyZh = "zh"
	BilingualTextKeyEn = "en"
)

// BilingualText encodes server-generated, user-visible copy so that the client
// can render it in whichever language the operator selected.
//
// Why this exists
//
// The platform display language is a per-browser choice (the frontend stores it
// in localStorage under "lang"); the backend never learns it. A producer that
// persists a single-language sentence therefore guarantees a mixed-language UI:
// the sentence is shown as-is in the "wrong" language forever, and the row
// cannot be repaired later because the other language was never recorded.
// Historical example: the AI predictive maintenance alarm stored
// "AI predictive maintenance: <point>" and rendered in English inside the
// Chinese console.
//
// Contract
//
//   - Producers of user-visible copy MUST use BilingualText instead of a bare
//     literal. Plain strings stay supported for legacy rows and for text typed
//     by an operator (rule names, work-order titles, product/point names).
//   - Wire format is a JSON object stored in the same plain text column that
//     used to hold the sentence:
//     {"zh":"中文文案","en":"English copy"}
//   - Clients resolve it at render time. The reference implementation is
//     noyo/frontend/src/utils/serverText.js (resolveServerText).
//
// An empty side falls back to the other side, so a partially translated payload
// still renders as readable text rather than an empty cell.
func BilingualText(zh, en string) string {
	zh = strings.TrimSpace(zh)
	en = strings.TrimSpace(en)
	if zh == "" && en == "" {
		return ""
	}
	if zh == "" {
		zh = en
	}
	if en == "" {
		en = zh
	}
	payload, err := json.Marshal(map[string]string{
		BilingualTextKeyZh: zh,
		BilingualTextKeyEn: en,
	})
	if err != nil {
		return zh
	}
	return string(payload)
}

// BilingualTextf is the formatting variant of BilingualText. Each language gets
// its own template so word order can differ between Chinese and English.
func BilingualTextf(zhTemplate, enTemplate string, args ...any) string {
	return BilingualText(fmt.Sprintf(zhTemplate, args...), fmt.Sprintf(enTemplate, args...))
}
