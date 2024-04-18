package pluginLabeler

import (
	"fmt"
	"log"

	c "github.com/clubanderson/labeler/pkg/common"
	k "github.com/clubanderson/labeler/pkg/kube-helpers"

	"k8s.io/apimachinery/pkg/runtime/schema"
)

func PluginLabeler(p c.ParamsStruct, reflect bool) []string {
	// function must be exportable (capitalize first letter of function name) to be discovered by labeler
	if reflect {
		return []string{"label,string,label key and value to be applied to objects (usage: --label=app.kubernetes.io/part-of=sample)"}
	}

	if p.Params["labelKey"] != "" && p.Params["labelVal"] != "" && (p.Flags["upgrade"] || p.Flags["install"] || p.Flags["apply"] || p.Flags["create"] || p.Flags["replace"]) {
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
					labelCmd := fmt.Sprintf("kubectl label %v %v %v=%v\n", gvr.Resource, r.ObjectName, p.Params["labelKey"], p.Params["labelVal"])
					c.RunResults.DidNotLabel = append(c.RunResults.DidNotLabel, labelCmd)
				} else {
					err = k.SetLabel(r.Namespace, r.ObjectName, gvr, p)
				}
			} else {
				err = k.SetLabel(r.Namespace, r.ObjectName, gvr, p)
			}
			if err != nil {
				if p.Flags["l-debug"] {
					log.Println("labeler.go: error (setLabel):", err)
				}
				// return err
			}
		}
	}
	if len(c.RunResults.DidNotLabel) > 0 {
		log.Printf("\nlabeler.go: The following resources can be labeled at a later time:\n\n")
		for _, cmd := range c.RunResults.DidNotLabel {
			log.Printf("%v", cmd)
		}
	}
	log.Println()

	return []string{}
}
