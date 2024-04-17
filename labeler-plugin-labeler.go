package main

import (
	"fmt"
	"log"
	"reflect"

	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

type ParamsStruct struct {
	homeDir       string
	path          string
	originalCmd   string
	kubeconfig    string
	ClientSet     *kubernetes.Clientset
	RestConfig    *rest.Config
	DynamicClient *dynamic.DynamicClient
	flags         map[string]bool
	params        map[string]string
	resources     map[ResourceStruct][]byte
	pluginArgs    map[string][]string
	pluginPtrs    map[string]reflect.Value
}

type resultsStruct struct {
	didNotLabel []string
}

var runResults resultsStruct

type ResourceStruct struct {
	Group      string
	Version    string
	Resource   string
	Namespace  string
	ObjectName string
}

type PluginImpl struct{}

func (pi PluginImpl) Run() []string {
	log.Println("Plugin is running")
	return []string{"label,string,label key and value to be applied to objects (usage: --label=app.kubernetes.io/part-of=sample)"}
}

func (p ParamsStruct) PluginLabeler(reflect bool) []string {
	// function must be exportable (capitalize first letter of function name) to be discovered by labeler
	if reflect {
		return []string{"label,string,label key and value to be applied to objects (usage: --label=app.kubernetes.io/part-of=sample)"}
	}

	if p.params["labelKey"] != "" && p.params["labelVal"] != "" && (p.flags["upgrade"] || p.flags["install"] || p.flags["apply"] || p.flags["create"] || p.flags["replace"]) {
		for r, v := range p.resources {
			_ = v
			gvr := schema.GroupVersionResource{
				Group:    r.Group,
				Version:  r.Version,
				Resource: r.Resource,
			}
			var err error
			if gvr.Resource == "namespaces" {
				if r.ObjectName == "" || r.ObjectName == "default" {
					labelCmd := fmt.Sprintf("kubectl label %v %v %v=%v\n", gvr.Resource, r.ObjectName, p.params["labelKey"], p.params["labelVal"])
					runResults.didNotLabel = append(runResults.didNotLabel, labelCmd)
				} else {
					err = p.setLabel(r.Namespace, r.ObjectName, gvr)
				}
			} else {
				err = p.setLabel(r.Namespace, r.ObjectName, gvr)
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
