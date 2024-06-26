package pluginOCMcreator

import (
	"encoding/json"
	"fmt"
	"log"

	c "github.com/clubanderson/labeler/pkg/common"

	"gopkg.in/yaml.v3"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

type ManifestWork struct {
	APIVersion string     `json:"apiVersion"`
	Kind       string     `json:"kind"`
	Metadata   mwMetadata `json:"metadata"`
	Spec       mwSpec     `json:"spec"`
}

type mwMetadata struct {
	Name string `json:"name"`
}

type mwSpec struct {
	Workload Workload `json:"workload"`
}

type Workload struct {
	Manifests []map[string]interface{} `json:"manifests"`
}

// Remove the unused type declaration
// type manifest struct {
// 	YAML string `yaml:"-"`
// }

func PluginCreateMW(p c.ParamsStruct, reflect bool) []string {
	// function must be exportable (capitalize first letter of function name) to be discovered by labeler
	if reflect {
		return []string{"l-mw-name,string,name for the manifestwork object", "l-mw-create,flag,create/apply the manifestwork object", "l-mw-namespace,string,namespace to apply the manifestwork object"}
	}
	// type PluginFunction struct {
	// 	pluginCreateMW string `triggerKey:"l-mw"`
	// }
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

	if p.Params[nArg] != "" {
		n = p.Params[nArg]
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

	for _, workloadYamlData := range p.Resources {
		var obj map[string]interface{}
		err := yaml.Unmarshal(workloadYamlData, &obj)
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
	// log.Printf("yamlData: \n%v", string(yamlData))

	if p.Flags["l-mw-create"] {
		log.Printf("  🚀 attempting to create %v object %q in namespace %q", k, n, p.Params["namespaceArg"])
		// log.Printf("%v %v %v %v %v %v", gvk.Group, gvk.Version, gvk.Kind, n, r, p.Params["namespaceArg"])
		objectJSON, err := json.Marshal(manifestWork)
		if err != nil {
			fmt.Println("Error marshaling JSON:", err)
			return []string{}
		}
		// log.Printf("objectJSON: \n%v", string(objectJSON))
		err = p.CreateObjForPlugin(gvk, yamlData, n, r, p.Params["namespaceArg"], objectJSON)
		if err != nil {
			log.Printf("  🔴 failed to create %v object %q in namespace %v.\n", r, n, p.Params["namespaceArg"])
		} else {
			log.Printf("  🟢 successfully created %v object %q in namespace %v.\n", r, n, p.Params["namespaceArg"])
		}
	} else {
		fmt.Printf("%v", string(yamlData))
	}
	return []string{}
}
