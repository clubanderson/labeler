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

func (p ParamsStruct) createBP() {
	bpName := "change-me"
	clusterSelectorLabelKey := "location-group"
	clusterSelectorLabelVal := "edge"
	wantSingletonReportedState := false
	bpGroup := "control.kubestellar.io"
	bpVersion := "v1alpha1"
	bpKind := "BindingPolicy"

	gvk := schema.GroupVersionKind{
		Group:   bpGroup,
		Version: bpVersion,
		Kind:    bpKind,
	}

	if p.params["bp-name"] != "" {
		bpName = p.params["bp-name"]
	}
	if p.params["bp-clusterselector"] != "" {
		clusterSelectorLabelKey = strings.Split(p.params["bp-clusterselector"], "=")[0]
		clusterSelectorLabelVal = strings.Split(p.params["bp-clusterselector"], "=")[1]
	}
	if p.flags["bp-wantsingletonreportedstate"] {
		wantSingletonReportedState = true
	}

	bindingPolicy := BindingPolicy{
		APIVersion: gvk.Group + "/" + gvk.Version,
		Kind:       gvk.Kind,
		Metadata: Metadata{
			Name: bpName,
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
		return
	}

	if p.flags["debug"] {
		log.Println("bp-wds flag:", p.params["bp-wds"])
	}

	if p.params["bp-wds"] != "" {
		log.Printf("  🚀 Attempting to create BindingPolicy object %q in WDS namespace %q", bpName, p.params["bp-wds"])
		objResource := "bindingpolicies"
		p.createObjForPlugin(gvk, yamlData, bpName, objResource)
	} else {
		fmt.Println(string(yamlData))
	}
}
