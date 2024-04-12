package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
)

func (p ParamsStruct) PluginLabeler(reflect bool) []string {
	// function must be exportable (capitalize first letter of function name) to be discovered by labeler
	if reflect {
		return []string{"label,string,label key and value to be applied to objects (usage: --label=app.kubernetes.io/part-of=sample)"}
	}

	if p.params["labelKey"] != "" && p.params["labelVal"] != "" && (p.flags["upgrade"] || p.flags["install"] || p.flags["apply"] || p.flags["create"] || p.flags["replace"]) {
		for resource, val := range p.resources {
			_ = val
			gvr := schema.GroupVersionResource{
				Group:    resource.Group,
				Version:  resource.Version,
				Resource: resource.Resource,
			}
			var err error
			if gvr.Resource == "namespaces" {
				if resource.ObjectName == "" || resource.ObjectName == "default" {
					labelCmd := fmt.Sprintf("kubectl label %v %v %v=%v\n", gvr.Resource, resource.ObjectName, p.params["labelKey"], p.params["labelVal"])
					runResults.didNotLabel = append(runResults.didNotLabel, labelCmd)
				} else {
					err = p.setLabel(resource.Namespace, resource.ObjectName, gvr)
				}
			} else {
				err = p.setLabel(resource.Namespace, resource.ObjectName, gvr)
			}
			if err != nil {
				if p.flags["l-debug"] {
					log.Println("labeler.go: error (setLabel):", err)
				}
				// return err
			}
		}
	}
	if len(runResults.didNotLabel) > 0 {
		log.Printf("\nlabeler.go: The following resources can be labeled at a later time:\n\n")
		for _, cmd := range runResults.didNotLabel {
			log.Printf(cmd)
		}
	}
	log.Println()

	return []string{}
}

func (p ParamsStruct) setLabel(namespace, objectName string, gvr schema.GroupVersionResource) error {

	if flags.label == "" && p.params["labelKey"] == "" {
		if p.flags["l-debug"] {
			log.Println("labeler.go: no label provided")
		}
		return nil
	}
	if flags.label != "" {
		p.params["labelKey"], p.params["labelVal"] = strings.Split(flags.label, "=")[0], strings.Split(flags.label, "=")[1]
	}

	labels := map[string]string{
		p.params["labelKey"]: p.params["labelVal"],
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

	if p.flags["l-debug"] {
		log.Printf("labeler.go: patching object %v/%v/%v %q in namespace %q with %v=%v %q %q %q %v\n", gvr.Group, gvr.Version, gvr.Resource, objectName, namespace, p.params["labelKey"], p.params["labelVal"], gvr.Resource, gvr.Version, gvr.Group, string(patch))
	}
	if namespace == "" {
		_, err = p.DynamicClient.Resource(gvr).Patch(context.TODO(), objectName, types.MergePatchType, patch, metav1.PatchOptions{})
		if err != nil {
			if p.flags["l-debug"] {
				log.Printf("labeler.go: error patching object %v/%v/%v %q in namespace %q: %v\n", gvr.Group, gvr.Version, gvr.Resource, objectName, namespace, err)
			}
		}
	} else {
		_, err = p.DynamicClient.Resource(gvr).Namespace(namespace).Patch(context.TODO(), objectName, types.MergePatchType, patch, metav1.PatchOptions{})
		if err != nil {
			if p.flags["l-debug"] {
				log.Printf("labeler.go: error patching object %v/%v/%v %q in namespace %q: %v\n", gvr.Group, gvr.Version, gvr.Resource, objectName, namespace, err)
			}
		}
	}

	if err != nil {
		if namespace != "" {
			labelCmd := fmt.Sprintf("kubectl label %v %v %v=%v -n %q\n", gvr.Resource, objectName, p.params["labelKey"], p.params["labelVal"], namespace)
			runResults.didNotLabel = append(runResults.didNotLabel, labelCmd)
		} else {
			labelCmd := fmt.Sprintf("kubectl label %v %v %v=%v\n", gvr.Resource, objectName, p.params["labelKey"], p.params["labelVal"])
			runResults.didNotLabel = append(runResults.didNotLabel, labelCmd)
		}
		return err
	}

	log.Printf("  🏷️ labeled object %v/%v/%v %q in namespace %q with %v=%v\n", gvr.Group, gvr.Version, gvr.Resource, objectName, namespace, p.params["labelKey"], p.params["labelVal"])
	return nil
}
