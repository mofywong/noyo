package aiassets

import (
	_ "embed"
	"encoding/json"
)

//go:embed recognition_catalog.json
var recognitionCatalogJSON []byte

type recognitionDefinition struct {
	ID                    string   `json:"id"`
	CapabilityID          string   `json:"capabilityId"`
	Category              string   `json:"category"`
	Icon                  string   `json:"icon"`
	Name                  Text     `json:"name"`
	Description           Text     `json:"description"`
	ResourceKind          string   `json:"resourceKind"`
	Dependencies          []string `json:"dependencies"`
	OptionalResourceKinds []string `json:"optionalResourceKinds"`
}

var recognitionDefinitions = func() []recognitionDefinition {
	var catalog struct {
		Items []recognitionDefinition `json:"items"`
	}
	if err := json.Unmarshal(recognitionCatalogJSON, &catalog); err != nil {
		panic("invalid compiled recognition catalog: " + err.Error())
	}
	return catalog.Items
}()

// YOLORecognitionCapabilities binds the shared user-facing catalog to compiled
// resource packages. Missing builds expose the same items with an explicit reason.
func YOLORecognitionCapabilities(packagesByKind map[string]string, unsupportedReason string) []Capability {
	ids := make(map[string]string, len(recognitionDefinitions))
	for _, item := range recognitionDefinitions {
		ids[item.ID] = item.CapabilityID
	}
	result := make([]Capability, 0, len(recognitionDefinitions))
	for _, item := range recognitionDefinitions {
		capability := Capability{ID: item.CapabilityID, Provider: "yolo_pro", SceneType: item.ID,
			Category: item.Category, Icon: item.Icon, Title: item.Name, Description: item.Description,
			UnsupportedReason: unsupportedReason}
		if unsupportedReason == "" {
			if id := packagesByKind[item.ResourceKind]; id != "" {
				capability.Packages = []string{id}
			} else {
				capability.UnsupportedReason = "algorithm_not_built"
			}
			for _, dependency := range item.Dependencies {
				capability.Dependencies = append(capability.Dependencies, ids[dependency])
			}
			for _, kind := range item.OptionalResourceKinds {
				if id := packagesByKind[kind]; id != "" {
					capability.OptionalPackages = append(capability.OptionalPackages, id)
				}
			}
		}
		result = append(result, capability)
	}
	return result
}
