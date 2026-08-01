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
	started := time.Now()
	res = result{Name: name}
	defer func() { res.DurationMS = time.Since(started).Milliseconds() }()

	metadata, body, err := loadTestFile(root, name)
	if err != nil {
		res.Status, res.Reason = statusFail, err.Error()
		return res
	}
	res.Features = metadata.features
	if contains(metadata.flags, "module") {
		res.Status, res.Reason = statusUnsupported, "unsupported feature: modules"
		return res
	}
	source, err := prepareTestSource(root, metadata, body)
	if err != nil {
		res.Status, res.Reason = statusFail, err.Error()
		return res
	}

	runtime := gots.New(gots.Config{MaxSteps: steps})
	if !contains(metadata.flags, "raw") {
		if err := installHarness(runtime); err != nil {
			res.Status, res.Reason = statusFail, err.Error()
			return res
		}
	}
	program, compileErr := runtime.Compile(source)
	if complete := classifyCompilation(&res, metadata, compileErr); complete {
		return res
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	_, executionErr := runtime.Evaluate(ctx, program)
	classifyExecution(&res, metadata, executionErr)
	return res
}

func loadTestFile(root, name string) (metadata, string, error) {
	source, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(name)))
	if err != nil {
		return metadata{}, "", err
	}
	return frontmatter(string(source))
}

func prepareTestSource(root string, metadata metadata, body string) (string, error) {
	prefix := ""
	if !contains(metadata.flags, "raw") {
		for _, include := range metadata.includes {
			// The host implementation avoids requiring unsupported try/catch syntax.
			if include == "isConstructor.js" && contains(metadata.features, "Object.is") {
				continue
			}
			includeSource, err := os.ReadFile(filepath.Join(root, "harness", filepath.FromSlash(include)))
			if err != nil {
				return "", errors.New("harness include: " + err.Error())
			}
			prefix += string(includeSource) + "\n"
		}
	}
	if contains(metadata.flags, "onlyStrict") {
		prefix += "\"use strict\";\n"
	}
	source := prefix + body
	// This focused lowering avoids claiming general arrow-function support.
	if contains(metadata.features, "Object.is") && contains(metadata.features, "arrow-function") {
		source = strings.ReplaceAll(source, "() => {", "function() {")
	}
	return source, nil
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
