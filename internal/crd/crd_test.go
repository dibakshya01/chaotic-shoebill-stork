package crd

import (
	"os"
	"path/filepath"
	"testing"

	"gopkg.in/yaml.v3"
)

// crdDoc is the subset of a CRD manifest we assert on.
type crdDoc struct {
	APIVersion string `yaml:"apiVersion"`
	Kind       string `yaml:"kind"`
	Spec       struct {
		Group string `yaml:"group"`
		Names struct {
			Kind string `yaml:"kind"`
		} `yaml:"names"`
		Versions []struct {
			Name    string `yaml:"name"`
			Served  bool   `yaml:"served"`
			Storage bool   `yaml:"storage"`
			Schema  struct {
				OpenAPIV3Schema struct {
					Properties struct {
						Spec struct {
							Properties map[string]any `yaml:"properties"`
						} `yaml:"spec"`
					} `yaml:"properties"`
				} `yaml:"openAPIV3Schema"`
			} `yaml:"schema"`
		} `yaml:"versions"`
	} `yaml:"spec"`
}

func TestCRDManifestsAreValid(t *testing.T) {
	dir := filepath.Join("..", "..", "operator", "config", "crd")
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read crd dir: %v", err)
	}
	foundKinds := map[string]bool{}
	for _, e := range entries {
		if e.IsDir() || filepath.Ext(e.Name()) != ".yaml" {
			continue
		}
		b, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			t.Fatal(err)
		}
		var doc crdDoc
		if err := yaml.Unmarshal(b, &doc); err != nil {
			t.Fatalf("%s: invalid YAML: %v", e.Name(), err)
		}
		if doc.APIVersion != "apiextensions.k8s.io/v1" || doc.Kind != "CustomResourceDefinition" {
			t.Errorf("%s: not a v1 CRD (apiVersion=%q kind=%q)", e.Name(), doc.APIVersion, doc.Kind)
		}
		if doc.Spec.Group != Group {
			t.Errorf("%s: group = %q, want %q", e.Name(), doc.Spec.Group, Group)
		}
		if len(doc.Spec.Versions) == 0 {
			t.Fatalf("%s: no versions", e.Name())
		}
		v := doc.Spec.Versions[0]
		if v.Name != Version || !v.Served || !v.Storage {
			t.Errorf("%s: version %q served=%v storage=%v (want %s/true/true)", e.Name(), v.Name, v.Served, v.Storage, Version)
		}
		if len(v.Schema.OpenAPIV3Schema.Properties.Spec.Properties) == 0 {
			t.Errorf("%s: spec schema has no properties", e.Name())
		}
		foundKinds[doc.Spec.Names.Kind] = true
	}
	for _, k := range Kinds {
		if !foundKinds[k] {
			t.Errorf("missing CRD for kind %q", k)
		}
	}
}
