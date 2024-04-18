package pluginAnnotator

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"

	c "github.com/clubanderson/labeler/pkg/common"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
)

func PluginAnnotator(p c.ParamsStruct, reflect bool) []string {
	// function must be exportable (capitalize first letter of function name) to be discovered by labeler
	if reflect {
		return []string{"l-annotation,string,annotation key and value to be applied to objects (usage: --annotation=creator='John Doe')"}
	}

	if p.Params["l-annotation"] != "" {
		if strings.Contains(p.Params["l-annotation"], ",") {
			// multiple annotations
			annotations := strings.Split(p.Params["l-annotation"], ",")
			for _, a := range annotations {
				p.Params["annotationKey"] = strings.Split(a, "=")[0]
				p.Params["annotationVal"] = strings.Split(a, "=")[1]
				annotator(p)
			}
		} else {
			p.Params["annotationKey"] = strings.Split(p.Params["l-annotation"], "=")[0]
			p.Params["annotationVal"] = strings.Split(p.Params["l-annotation"], "=")[1]
			annotator(p)
		}
	}
	return []string{}
}

func annotator(p c.ParamsStruct) {
	log.Printf("pluginAnnotator.go: p.Params[\"annotationKey\"] = %v\n", p.Params["annotationKey"])
	if p.Params["annotationKey"] != "" && p.Params["annotationVal"] != "" && (p.Flags["upgrade"] || p.Flags["install"] || p.Flags["apply"] || p.Flags["create"] || p.Flags["replace"]) {
		for r, v := range p.Resources {
			_ = v
			gvr := schema.GroupVersionResource{
				Group:    r.Group,
				Version:  r.Version,
				Resource: r.Resource,
			}
			var err error
			if gvr.Resource == "namespaces" {
				if r.ObjectName == "" || r.ObjectName == "default" {
					labelCmd := fmt.Sprintf("kubectl annotate %v %v %v=%v\n", gvr.Resource, r.ObjectName, p.Params["annotationKey"], p.Params["annotationVal"])
					c.RunResults.DidNotAnnotate = append(c.RunResults.DidNotAnnotate, labelCmd)
				} else {
					err = setAnnotation(r.Namespace, r.ObjectName, gvr, p)
				}
			} else {
				err = setAnnotation(r.Namespace, r.ObjectName, gvr, p)
			}
			if err != nil {
				if p.Flags["l-debug"] {
					log.Println("labeler.go: error (setAnnotation):", err)
				}
				// return err
			}
		}
	}
	if len(c.RunResults.DidNotAnnotate) > 0 {
		log.Printf("\nlabeler.go: The following resources can be annotated at a later time:\n\n")
		for _, cmd := range c.RunResults.DidNotAnnotate {
			log.Printf("%v", cmd)
		}
	}
	log.Println()
}

func setAnnotation(namespace, objectName string, gvr schema.GroupVersionResource, p c.ParamsStruct) error {

	if c.Flags.Annotation == "" && p.Params["annotationKey"] == "" {
		if p.Flags["l-debug"] {
			log.Println("labeler.go: no annotation provided")
		}
		return nil
	}
	if c.Flags.Annotation != "" {
		p.Params["annotationKey"], p.Params["annotationVal"] = strings.Split(c.Flags.Annotation, "=")[0], strings.Split(c.Flags.Annotation, "=")[1]
	}

	annotations := map[string]string{
		p.Params["annotationKey"]: p.Params["annotationVal"],
	}

	// serialize annotations to JSON
	patch, err := json.Marshal(map[string]interface{}{
		"metadata": map[string]interface{}{
			"annotations": annotations,
		},
	})
	if err != nil {
		return err
	}

	if p.Flags["l-debug"] {
		log.Printf("labeler.go: patching object %v/%v/%v %q in namespace %q with %v=%v %q %q %q %v\n", gvr.Group, gvr.Version, gvr.Resource, objectName, namespace, p.Params["annotationKey"], p.Params["annotationVal"], gvr.Resource, gvr.Version, gvr.Group, string(patch))
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
			annotationCmd := fmt.Sprintf("kubectl annotate %v %v %v=%v -n %q\n", gvr.Resource, objectName, p.Params["annotationKey"], p.Params["annotationVal"], namespace)
			c.RunResults.DidNotLabel = append(c.RunResults.DidNotAnnotate, annotationCmd)
		} else {
			annotationCmd := fmt.Sprintf("kubectl annotate %v %v %v=%v\n", gvr.Resource, objectName, p.Params["annotationKey"], p.Params["annotationVal"])
			c.RunResults.DidNotLabel = append(c.RunResults.DidNotAnnotate, annotationCmd)
		}
		return err
	}

	log.Printf("  🏷️ annotated object %v/%v/%v %q in namespace %q with %v=%v\n", gvr.Group, gvr.Version, gvr.Resource, objectName, namespace, p.Params["annotationKey"], p.Params["annotationVal"])
	return nil
}
