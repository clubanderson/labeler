package main

import (
	"fmt"
	"log"
	"strings"

	"gopkg.in/yaml.v3"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

type BindingPolicy struct {
	APIVersion string   `yaml:"apiVersion"`
	Kind       string   `yaml:"kind"`
	Metadata   Metadata `yaml:"metadata"`
	Spec       Spec     `yaml:"spec"`
}

type Spec struct {
	WantSingletonReportedState bool              `yaml:"wantSingletonReportedState"`
	ClusterSelectors           []ClusterSelector `yaml:"clusterSelectors"`
	Downsync                   []Downsync        `yaml:"downsync"`
}

type Metadata struct {
	Name string `yaml:"name"`
}

type ClusterSelector struct {
	MatchLabels map[string]string `yaml:"matchLabels"`
}

type Downsync struct {
	ObjectSelectors []ObjectSelector `yaml:"objectSelectors"`
}

type ObjectSelector struct {
	MatchLabels map[string]string `yaml:"matchLabels"`
}

func (p ParamsStruct) PluginCreateBP(reflect bool) []string {
	// function must be exportable (capitalize first letter of function name) to be discovered by labeler
	if reflect {
		return []string{"l-bp-name,string,name for the bindingpolicy (usage: --l-bp-name=hello-world)", "l-bp-ns,string,namespace for the bindingpolicies (usage: --l-bp-ns=default)", "l-bp-clusterselector,string,value of clusterSelector (usage: --l-bp-clusterselector=app.kubernetes.io/part-of=sample-app)", "l-bp-wantsingletonreportedstate,flag,do you prefer singleton status for an object, if not, then grouped status will be recorded", "l-bp-wds,string,where should the object be created (usage: --l-bp-wds=namespace)"}
	}
	n := "change-me"
	nArg := "l-bp-name"
	nsArg := "l-bp-ns"
	clusterSelectorLabelKey := "location-group"
	clusterSelectorLabelVal := "edge"
	wantSingletonReportedState := false
	g := "control.kubestellar.io"
	v := "v1alpha1"
	k := "BindingPolicy"
	r := "bindingpolicies"

	gvk := schema.GroupVersionKind{
		Group:   g,
		Version: v,
		Kind:    k,
	}

	if p.params[nArg] != "" {
		n = p.params[nArg]
	}
	if p.params["l-bp-clusterselector"] != "" {
		clusterSelectorLabelKey = strings.Split(p.params["l-bp-clusterselector"], "=")[0]
		clusterSelectorLabelVal = strings.Split(p.params["l-bp-clusterselector"], "=")[1]
	}
	if p.flags["l-bp-wantsingletonreportedstate"] {
		wantSingletonReportedState = true
	}

	bindingPolicy := BindingPolicy{
		APIVersion: gvk.Group + "/" + gvk.Version,
		Kind:       gvk.Kind,
		Metadata: Metadata{
			Name: n,
		},
		Spec: Spec{
			WantSingletonReportedState: wantSingletonReportedState,
			ClusterSelectors: []ClusterSelector{
				{
					MatchLabels: map[string]string{
						clusterSelectorLabelKey: clusterSelectorLabelVal,
					},
				},
			},
			Downsync: []Downsync{
				{
					ObjectSelectors: []ObjectSelector{
						{
							MatchLabels: map[string]string{
								p.params["labelKey"]: p.params["labelVal"],
							},
						},
					},
				},
			},
		},
	}

	yamlData, err := yaml.Marshal(bindingPolicy)
	if err != nil {
		fmt.Println("Error marshaling YAML:", err)
		return []string{}
	}

	if p.flags["l-debug"] {
		log.Printf("%v parameter: %v", nsArg, p.params[nsArg])
	}

	if p.params["l-bp-wds"] != "" {
		log.Printf("  🚀 Attempting to create %v object %q in WDS namespace %q", k, n, p.params[nsArg])
		p.createObjForPlugin(gvk, yamlData, n, r, p.params["namespaceArg"])
	} else {
		fmt.Println(string(yamlData))
	}
	return []string{}
}
