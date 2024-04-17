package labeler

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
	Manifests []map[string]interface{} `yaml:"manifests"`
}

type Manifest struct {
	YAML string `yaml:"-"`
}

func (p ParamsStruct) PluginCreateMW(reflect bool) []string {
	// function must be exportable (capitalize first letter of function name) to be discovered by labeler
	if reflect {
		return []string{"l-mw-name,string,name for the manifestwork object", "l-mw-create,flag,create/apply the manifestwork object"}
	}
	type PluginFunction struct {
		pluginCreateMW string `triggerKey:"l-mw"`
	}
	n := "change-me"
	nArg := "l-mw-name"
	g := "work.open-cluster-management.io"
	v := "v1"
	k := "ManifestWork"
	r := "manifestworks"

	gvk := schema.GroupVersionKind{
		Group:   g,
		Version: v,
		Kind:    k,
	}

	if p.params[nArg] != "" {
		n = p.params[nArg]
	}

	manifestWork := ManifestWork{
		APIVersion: gvk.Group + "/" + gvk.Version,
		Kind:       gvk.Kind,
		Metadata: mwMetadata{
			Name: n,
		},
		Spec: mwSpec{
			Workload: Workload{
				Manifests: []map[string]interface{}{},
			},
		},
	}
	// need a loop to fill in the manifests with the objects from debug run of kubectl or helm

	for _, yamlData := range p.resources {
		var obj map[string]interface{}
		err := yaml.Unmarshal(yamlData, &obj)
		if err != nil {
			log.Printf("Error unmarshaling YAML: %v", err)
			continue
		}
		manifestWork.Spec.Workload.Manifests = append(manifestWork.Spec.Workload.Manifests, obj)
	}

	yamlData, err := yaml.Marshal(manifestWork)
	if err != nil {
		fmt.Println("Error marshaling YAML:", err)
		return []string{}
	}

	if p.flags["l-mw-create"] {
		log.Printf("  🚀 Attempting to create %v object %q in namespace %q", k, n, p.params["namespaceArg"])
		p.createObjForPlugin(gvk, yamlData, n, r, p.params["namespaceArg"])
	} else {
		fmt.Printf("%v", string(yamlData))
	}
	return []string{}
}
