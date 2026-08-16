package bundle

import (
	"path/filepath"
	"sort"
	"testing"
)

// Every bundle.yaml in the repository MUST load under the strict decoder.
//
// This is the gate that makes bundle.yaml a source rather than documentation.
// Before it, the loader used a plain Unmarshal: an unmodelled key bound to
// nothing and failed silently, so `components.cklib.version` read like the
// authoritative pin while being compared to nothing at any point in the build.
func TestEveryBundleLoadsStrict(t *testing.T) {
	files, err := filepath.Glob("../../bundles/*/bundle.yaml")
	if err != nil {
		t.Fatal(err)
	}
	if len(files) == 0 {
		t.Fatal("no bundle.yaml found — the glob is wrong, not the repo")
	}
	sort.Strings(files)

	for _, f := range files {
		name := filepath.Base(filepath.Dir(f))
		t.Run(name, func(t *testing.T) {
			spec, err := Load(f)
			if err != nil {
				t.Fatalf("strict load failed:\n  %v", err)
			}
			if spec.Name == "" {
				t.Errorf("name is empty")
			}
			if spec.SpecVersion == "" {
				t.Errorf("spec_version is empty — a bundle must say which contract it conforms to")
			}
		})
	}
}

// A runtime input is a promise to the operator. These two fields are what make
// it different from documentation, so they are asserted rather than assumed.
func TestRuntimeInputsAreWellFormed(t *testing.T) {
	files, _ := filepath.Glob("../../bundles/*/bundle.yaml")
	for _, f := range files {
		spec, err := Load(f)
		if err != nil {
			continue // covered by TestEveryBundleLoadsStrict
		}
		for name, in := range spec.RuntimeInputs {
			where := filepath.Base(filepath.Dir(f)) + "." + name
			if in.ConsumedBy == "" {
				t.Errorf("%s: consumed_by is empty — an input nothing reads is not an input", where)
			}
			switch in.Form {
			case "scalar", "document":
			default:
				t.Errorf("%s: form must be scalar|document, got %q", where, in.Form)
			}
			if !in.NeverBaked {
				t.Errorf("%s: never_baked must be true — a baked value is not a runtime input", where)
			}
			if in.Form == "document" && in.MediaType == "" {
				t.Errorf("%s: a document input must declare media_type", where)
			}
			for _, d := range in.Delivery {
				if d != "file" && d != "env" {
					t.Errorf("%s: delivery must be file|env, got %q", where, d)
				}
			}
		}
	}
}
