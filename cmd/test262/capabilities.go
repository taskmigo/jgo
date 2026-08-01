package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

var unsupportedReasonCodes = map[string]struct{}{
	"unsupported-feature":         {},
	"unsupported-annex-b":         {},
	"unsupported-intl402":         {},
	"unsupported-host-capability": {},
	"unsupported-proposal":        {},
	"unsupported-module-kind":     {},
	"unsupported-concurrency":     {},
	"unsupported-bigint":          {},
}

func readCapabilityManifest(selectionPath string, selection manifest) (capabilityManifest, error) {
	if selection.CapabilityManifest == "" {
		return capabilityManifest{}, errors.New("invalid selection manifest: missing capabilityManifest")
	}
	path := selection.CapabilityManifest
	if !filepath.IsAbs(path) {
		path = filepath.Join(filepath.Dir(selectionPath), path)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return capabilityManifest{}, fmt.Errorf("read capability manifest: %w", err)
	}
	var capabilities capabilityManifest
	if err := json.Unmarshal(data, &capabilities); err != nil {
		return capabilityManifest{}, fmt.Errorf("parse capability manifest: %w", err)
	}
	if capabilities.SchemaVersion != 1 {
		return capabilityManifest{}, fmt.Errorf("unsupported capability manifest schema: %d", capabilities.SchemaVersion)
	}
	seen := make(map[string]struct{}, len(capabilities.Supported)+len(capabilities.Unsupported))
	for _, feature := range capabilities.Supported {
		if feature == "" {
			return capabilityManifest{}, errors.New("capability manifest contains an empty supported feature")
		}
		if _, exists := seen[feature]; exists {
			return capabilityManifest{}, fmt.Errorf("duplicate capability: %s", feature)
		}
		seen[feature] = struct{}{}
	}
	for _, unsupported := range capabilities.Unsupported {
		if unsupported.Feature == "" {
			return capabilityManifest{}, errors.New("capability manifest contains an empty unsupported feature")
		}
		if _, exists := unsupportedReasonCodes[unsupported.Reason]; !exists {
			return capabilityManifest{}, fmt.Errorf("invalid unsupported reason %q for %s", unsupported.Reason, unsupported.Feature)
		}
		if _, exists := seen[unsupported.Feature]; exists {
			return capabilityManifest{}, fmt.Errorf("duplicate capability: %s", unsupported.Feature)
		}
		seen[unsupported.Feature] = struct{}{}
	}
	return capabilities, nil
}

func unsupportedReason(capabilities capabilityManifest, feature string) string {
	for _, unsupported := range capabilities.Unsupported {
		if unsupported.Feature == feature {
			return unsupported.Reason
		}
	}
	return "unsupported-feature"
}

func supportsCapability(capabilities capabilityManifest, feature string) bool {
	for _, supported := range capabilities.Supported {
		if supported == feature {
			return true
		}
	}
	return false
}
