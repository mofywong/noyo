package core

import (
	"fmt"
	"strings"
)

const maxCascadeGatewayCodeLength = 64

// BuildCascadeGatewayCode returns the globally unique cascade routing code for
// a physical gateway within its platform tenant and project scope.
func BuildCascadeGatewayCode(tenantID, projectID uint, serialNumber string) (string, error) {
	serialNumber = strings.TrimSpace(serialNumber)
	if tenantID == 0 {
		return "", fmt.Errorf("platform tenant ID is required")
	}
	if projectID == 0 {
		return "", fmt.Errorf("platform project ID is required")
	}
	if serialNumber == "" {
		return "", fmt.Errorf("gateway SN is required")
	}

	code := fmt.Sprintf("%d_%d_%s", tenantID, projectID, serialNumber)
	if len(code) > maxCascadeGatewayCodeLength {
		return "", fmt.Errorf("gateway code exceeds %d characters", maxCascadeGatewayCodeLength)
	}
	return code, nil
}
