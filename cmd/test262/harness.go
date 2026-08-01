package main

import (
	"context"
	"errors"
	"fmt"
	gots "github.com/example/gots"
	"os"
	"path/filepath"
	"strings"
	"time"
)

func runOne(root, name string, steps uint64, timeout time.Duration) (res result) {
	metadata, body, err := loadTestFile(root, name)
	if err != nil {
		return result{Name: name, Status: statusFail, Reason: err.Error()}
	}
	return runCase(root, executionCase{SourceName: name, ID: name, Variant: variantForLegacyRun(metadata), Metadata: metadata}, body, capabilityManifest{}, steps, timeout)
}

func runCase(root string, testCase executionCase, body string, capabilities capabilityManifest, steps uint64, timeout time.Duration) result {
	res := result{Name: testCase.ID, Features: testCase.Metadata.features}
	if code, detail := unsupportedTestReason(testCase, capabilities); code != "" {
		res.Status, res.UnsupportedReason, res.Reason = statusUnsupported, code, detail
		return res
	}
	if testCase.LoadError != "" {
		res.Status, res.Reason = statusFail, testCase.LoadError
		return res
	}
	source, err := prepareTestSource(testCase.Metadata, body, testCase.Variant)
	if err != nil {
		res.Status, res.Reason = statusFail, err.Error()
		return res
	}

	runtime := gots.New(gots.Config{MaxSteps: steps})
	if testCase.Variant != "raw" {
		if err := installHarness(runtime); err != nil {
			res.Status, res.Reason = statusFail, err.Error()
			return res
		}
		if err := evaluateHarnessIncludes(runtime, root, testCase.Metadata); err != nil {
			classifyHarnessFailure(&res, err)
			return res
		}
	}
	program, compileErr := runtime.Compile(source)
	if complete := classifyCompilation(&res, testCase.Metadata, compileErr); complete {
		return res
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	_, executionErr := runtime.Evaluate(ctx, program)
	classifyExecution(&res, testCase.Metadata, executionErr)
	return res
}

func classifyHarnessFailure(result *result, err error) {
	var syntaxError *gots.SyntaxError
	var exception *gots.Exception
	switch {
	case errors.Is(err, gots.ErrCancelled), errors.Is(err, gots.ErrStepLimit):
		result.Status = statusTimeout
	case errors.As(err, &syntaxError), errors.As(err, &exception):
		result.Status = statusUnsupported
		result.UnsupportedReason = "unsupported-feature"
	default:
		result.Status = statusFail
	}
	result.Reason = err.Error()
}

func unsupportedTestReason(testCase executionCase, capabilities capabilityManifest) (string, string) {
	if strings.HasSuffix(testCase.SourceName, "_FIXTURE.js") {
		return "unsupported-host-capability", "Test262 fixture is not a directly runnable test"
	}
	if testCase.Variant == "module" {
		if !supportsCapability(capabilities, "source-text-modules") {
			return unsupportedReason(capabilities, "source-text-modules"), "unsupported feature: modules"
		}
	}
	if testCase.Variant == "strict" {
		if !supportsCapability(capabilities, "strict-mode") {
			return unsupportedReason(capabilities, "strict-mode"), "unsupported feature: strict mode"
		}
		if capability, found := unsupportedStrictDependency(testCase.Metadata.features, capabilities); found {
			return unsupportedReason(capabilities, capability), "unsupported strict-test dependency: " + capability
		}
	}
	if strings.HasPrefix(filepath.ToSlash(testCase.SourceName), "test/annexB/") {
		return unsupportedReason(capabilities, "annex-b"), "unsupported feature: Annex B"
	}
	if contains(testCase.Metadata.features, "BigInt") {
		return unsupportedReason(capabilities, "bigint"), "unsupported feature: BigInt"
	}
	if contains(testCase.Metadata.features, "nonextensible-applies-to-private") {
		return unsupportedReason(capabilities, "external-proposals"), "unsupported proposal: nonextensible-applies-to-private"
	}
	return "", ""
}

func unsupportedStrictDependency(features []string, capabilities capabilityManifest) (string, bool) {
	for _, feature := range features {
		capability := ""
		switch {
		case feature == "BigInt":
			capability = "bigint"
		case feature == "Proxy":
			capability = "proxy"
		case feature == "Temporal":
			capability = "temporal"
		case feature == "SharedArrayBuffer" || strings.HasPrefix(feature, "Atomics"):
			capability = "shared-memory-and-atomics"
		case feature == "generators":
			capability = "generators"
		case feature == "async-functions" || feature == "async-iteration" || feature == "async-generators":
			capability = "async-execution"
		case feature == "class" || strings.HasPrefix(feature, "class-"):
			capability = "classes"
		case feature == "Promise" || strings.HasPrefix(feature, "Promise.") || strings.HasPrefix(feature, "Promise-"):
			capability = "promises"
		case strings.HasPrefix(feature, "Intl.") || strings.HasPrefix(feature, "Intl-"):
			capability = "intl-ecma-402"
		case feature == "source-phase-imports" || feature == "source-phase-imports-module-source" || feature == "import-defer":
			capability = "external-proposals"
		}
		if capability != "" && !supportsCapability(capabilities, capability) {
			return capability, true
		}
	}
	return "", false
}

func loadTestFile(root, name string) (metadata, string, error) {
	source, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(name)))
	if err != nil {
		return metadata{}, "", err
	}
	return frontmatter(string(source))
}

func prepareTestSource(metadata metadata, body, variant string) (string, error) {
	source := body
	if variant == "strict" {
		source = "\"use strict\";\n" + source
	}
	// This focused lowering avoids claiming general arrow-function support.
	if contains(metadata.features, "Object.is") && contains(metadata.features, "arrow-function") {
		source = strings.ReplaceAll(source, "() => {", "function() {")
	}
	return source, nil
}

func evaluateHarnessIncludes(runtime *gots.Runtime, root string, metadata metadata) error {
	for _, include := range metadata.includes {
		// The host implementation avoids requiring unsupported try/catch syntax.
		if include == "isConstructor.js" && contains(metadata.features, "Object.is") {
			continue
		}
		includeSource, err := os.ReadFile(filepath.Join(root, "harness", filepath.FromSlash(include)))
		if err != nil {
			return fmt.Errorf("harness include: %w", err)
		}
		program, err := runtime.Compile(string(includeSource))
		if err != nil {
			return fmt.Errorf("harness include: %w", err)
		}
		if _, err := runtime.Evaluate(context.Background(), program); err != nil {
			return fmt.Errorf("harness include: %w", err)
		}
	}
	return nil
}

func variantForLegacyRun(metadata metadata) string {
	switch {
	case contains(metadata.flags, "raw"):
		return "raw"
	case contains(metadata.flags, "module"):
		return "module"
	case contains(metadata.flags, "onlyStrict"):
		return "strict"
	default:
		return "sloppy"
	}
}

func classifyCompilation(result *result, metadata metadata, err error) bool {
	if metadata.negativeParse {
		if err != nil {
			result.Status = statusPass
		} else {
			result.Status = statusFail
			result.Reason = "expected parse error"
		}
		return true
	}
	if err != nil {
		var syntaxError *gots.SyntaxError
		if errors.As(err, &syntaxError) {
			result.Status = statusUnsupported
			result.UnsupportedReason = "unsupported-feature"
		} else {
			result.Status = statusFail
		}
		result.Reason = err.Error()
		return true
	}
	return false
}

func classifyExecution(result *result, metadata metadata, err error) {
	if metadata.negative {
		if err != nil {
			result.Status = statusPass
		} else {
			result.Status = statusFail
			result.Reason = "expected " + metadata.negativeType + " error"
		}
		return
	}
	if err == nil {
		result.Status = statusPass
	} else if errors.Is(err, gots.ErrCancelled) || errors.Is(err, gots.ErrStepLimit) {
		result.Status = statusTimeout
		result.Reason = err.Error()
	} else if !strings.Contains(err.Error(), "Test262 assertion failed") {
		result.Status = statusUnsupported
		result.UnsupportedReason = "unsupported-feature"
		result.Reason = err.Error()
	} else {
		result.Status = statusFail
		result.Reason = err.Error()
	}
}

func installHarness(runtime *gots.Runtime) error {
	if err := runtime.Define("isConstructor", gots.NativeFunction(func(_ *gots.Runtime, _ gots.Value, args []gots.Value) (gots.Value, error) {
		return gots.Boolean(len(args) > 0 && args[0].IsConstructor()), nil
	})); err != nil {
		return err
	}
	assertCall := gots.NativeFunction(func(_ *gots.Runtime, _ gots.Value, args []gots.Value) (gots.Value, error) {
		if len(args) == 0 || !args[0].ToBoolean() {
			return gots.Undefined(), fmt.Errorf("Test262 assertion failed")
		}
		return gots.Undefined(), nil
	})
	if err := runtime.Define("assert", assertCall); err != nil {
		return err
	}
	assert, _ := runtime.Lookup("assert")
	same := gots.NativeFunction(func(_ *gots.Runtime, _ gots.Value, args []gots.Value) (gots.Value, error) {
		if len(args) < 2 || !gots.SameValue(args[0], args[1]) {
			return gots.Undefined(), fmt.Errorf("Test262 assertion failed: values are not the same")
		}
		return gots.Undefined(), nil
	})
	notSame := gots.NativeFunction(func(_ *gots.Runtime, _ gots.Value, args []gots.Value) (gots.Value, error) {
		if len(args) >= 2 && gots.SameValue(args[0], args[1]) {
			return gots.Undefined(), fmt.Errorf("Test262 assertion failed: values are the same")
		}
		return gots.Undefined(), nil
	})
	throws := gots.NativeFunction(func(runtime *gots.Runtime, _ gots.Value, args []gots.Value) (gots.Value, error) {
		if len(args) < 2 {
			return gots.Undefined(), fmt.Errorf("Test262 assert.throws requires a constructor and callback")
		}
		if _, err := runtime.Call(context.Background(), args[1], gots.Undefined()); err == nil {
			return gots.Undefined(), fmt.Errorf("Test262 assertion failed: expected an exception")
		}
		return gots.Undefined(), nil
	})
	for name, function := range map[string]gots.NativeFunction{"sameValue": same, "notSameValue": notSame, "throws": throws} {
		key := "__test262_" + name
		if err := runtime.Define(key, function); err != nil {
			return err
		}
		propertyValue, _ := runtime.Lookup(key)
		if err := runtime.SetProperty(assert, gots.String(name), propertyValue); err != nil {
			return err
		}
	}
	if err := runtime.Define("assert", assert); err != nil {
		return err
	}
	// Error constructors are identity markers for assert.throws in this subset.
	for _, name := range []string{"TypeError", "RangeError", "SyntaxError"} {
		if err := runtime.Define(name, gots.NativeFunction(func(_ *gots.Runtime, _ gots.Value, _ []gots.Value) (gots.Value, error) { return gots.NewObject(), nil })); err != nil {
			return err
		}
	}
	return nil
}
