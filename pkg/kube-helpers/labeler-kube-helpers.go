package kubeHelpers

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	c "github.com/clubanderson/labeler/pkg/common"
	"gopkg.in/yaml.v3"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
)

func AddNamespaceToResources(p c.ParamsStruct) error {
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

func SetLabel(namespace, objectName string, gvr schema.GroupVersionResource, p c.ParamsStruct) error {

	if c.Flags.Label == "" && p.Params["labelKey"] == "" {
		if p.Flags["l-debug"] {
			log.Println("labeler.go: no label provided")
		}
		return nil
	}
	if c.Flags.Label != "" {
		p.Params["labelKey"], p.Params["labelVal"] = strings.Split(c.Flags.Label, "=")[0], strings.Split(c.Flags.Label, "=")[1]
	}

	labels := map[string]string{
		p.Params["labelKey"]: p.Params["labelVal"],
	}

	// serialize labels to JSON
	patch, err := json.Marshal(map[string]interface{}{
		"metadata": map[string]interface{}{
			"labels": labels,
		},
	})
	if err != nil {
		return err
	}

	if p.Flags["l-debug"] {
		log.Printf("labeler.go: patching object %v/%v/%v %q in namespace %q with %v=%v %q %q %q %v\n", gvr.Group, gvr.Version, gvr.Resource, objectName, namespace, p.Params["labelKey"], p.Params["labelVal"], gvr.Resource, gvr.Version, gvr.Group, string(patch))
	}
	if namespace == "" {
		_, err = p.DynamicClient.Resource(gvr).Patch(context.TODO(), objectName, types.MergePatchType, patch, metav1.PatchOptions{})
		if err != nil {
			if p.Flags["l-debug"] {
				log.Printf("labeler.go: error patching object %v/%v/%v %q in namespace %q: %v\n", gvr.Group, gvr.Version, gvr.Resource, objectName, namespace, err)
			}
		}
	} else {
		_, err = p.DynamicClient.Resource(gvr).Namespace(namespace).Patch(context.TODO(), objectName, types.MergePatchType, patch, metav1.PatchOptions{})
		if err != nil {
			if p.Flags["l-debug"] {
				log.Printf("labeler.go: error patching object %v/%v/%v %q in namespace %q: %v\n", gvr.Group, gvr.Version, gvr.Resource, objectName, namespace, err)
			}
		}
	}

	if err != nil {
		if namespace != "" {
			labelCmd := fmt.Sprintf("kubectl label %v %v %v=%v -n %q\n", gvr.Resource, objectName, p.Params["labelKey"], p.Params["labelVal"], namespace)
			c.RunResults.DidNotLabel = append(c.RunResults.DidNotLabel, labelCmd)
		} else {
			labelCmd := fmt.Sprintf("kubectl label %v %v %v=%v\n", gvr.Resource, objectName, p.Params["labelKey"], p.Params["labelVal"])
			c.RunResults.DidNotLabel = append(c.RunResults.DidNotLabel, labelCmd)
		}
		return err
	}

	log.Printf("  🏷️ labeled object %v/%v/%v %q in namespace %q with %v=%v\n", gvr.Group, gvr.Version, gvr.Resource, objectName, namespace, p.Params["labelKey"], p.Params["labelVal"])
	return nil
}

func LabelResources(p c.ParamsStruct) error {
	for r, v := range p.Resources {
		_ = v
		gvr := schema.GroupVersionResource{
			Group:    r.Group,
			Version:  r.Version,
			Resource: r.Resource,
		}
		err := SetLabel(r.Namespace, r.ObjectName, gvr, p)
		if err != nil {
			log.Println("labeler.go: error (setLabel):", err)
			return err
		}
	}
	return nil
}
