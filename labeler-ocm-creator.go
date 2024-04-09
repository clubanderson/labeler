package main

import (
	"fmt"
	"log"

	"gopkg.in/yaml.v3"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

type ManifestWork struct {
	APIVersion string     `yaml:"apiVersion"`
	Kind       string     `yaml:"kind"`
	Metadata   mwMetadata `yaml:"metadata"`
	Spec       mwSpec     `yaml:"spec"`
}

type mwSpec struct {
	Workload Workload `yaml:"workload"`
}

type mwMetadata struct {
	Name string `yaml:"name"`
}

type Workload struct {
	Manifests []Manifest `yaml:"manifests"`
}

type Manifest struct {
	RawExtension `yaml:",inline"`
}

type RawExtension struct {
	Raw []byte `yaml:",inline"`
}

func (p ParamsStruct) createMW() {
	mwName := "change-me"
	mwGroup := "work.open-cluster-management.io"
	mwVersion := "v1"
	mwKind := "ManifestWork"

	gvk := schema.GroupVersionKind{
		Group:   mwGroup,
		Version: mwVersion,
		Kind:    mwKind,
	}

	if p.params["mw-name"] != "" {
		mwName = p.params["mw-name"]
	}

	manifestWork := ManifestWork{
		APIVersion: gvk.Group + "/" + gvk.Version,
		Kind:       gvk.Kind,
		Metadata: mwMetadata{
			Name: mwName,
		},
		Spec: mwSpec{
			Workload: Workload{
				Manifests: []Manifest{},
			},
		},
	}

	yamlData, err := yaml.Marshal(manifestWork)
	if err != nil {
		fmt.Println("Error marshaling YAML:", err)
		return
	}

	if p.flags["debug"] {
		log.Println("mw-ns flag:", p.params["mw-ns"])
	}

	if p.params["bp-wds"] != "" {
		log.Printf("  🚀 Attempting to create ManifestWork object %q in WDS namespace %q", mwName, p.params["mw-wds"])
		objResource := "manifestworks"
		p.createObjForPlugin(gvk, yamlData, mwName, objResource)
	} else {
		fmt.Println(string(yamlData))
	}
}
