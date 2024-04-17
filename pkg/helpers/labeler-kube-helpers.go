package helpers

import (
	"fmt"

	c "github.com/clubanderson/labeler/pkg/common"
	"gopkg.in/yaml.v3"
)

func addNamespaceToResources(p c.ParamsStruct) error {
	p.Params["namespaceArg"] = ""
	if p.Params["namespace"] != "" {
		p.Params["namespaceArg"] = p.Params["namespace"]
	} else if p.Params["n"] != "" {
		p.Params["namespaceArg"] = p.Params["n"]
	}
	if p.Params["namespaceArg"] == "" {
		p.Params["namespaceArg"] = "default"
	}

	resource := c.ResourceStruct{
		Group:      "",
		Version:    "v1",
		Resource:   "namespaces",
		Namespace:  "",
		ObjectName: p.Params["namespaceArg"],
	}
	namespaceYAML := c.Namespace{
		APIVersion: "v1",
		Kind:       "namespace",
		Metadata: c.Metadata{
			Name: p.Params["namespaceArg"],
		},
	}
	yamlData, err := yaml.Marshal(namespaceYAML)
	if err != nil {
		fmt.Println("Error marshaling YAML:", err)
		return err
	}

	p.Resources[resource] = yamlData
	return nil
}
