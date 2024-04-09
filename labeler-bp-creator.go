package main

import (
	"fmt"
	"strings"

	"gopkg.in/yaml.v2"
)

type BindingPolicy struct {
	APIVersion                 string            `yaml:"apiVersion"`
	Kind                       string            `yaml:"kind"`
	Metadata                   Metadata          `yaml:"metadata"`
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
		APIVersion: "control.kubestellar.io/v1alpha1",
		Kind:       "BindingPolicy",
		Metadata: Metadata{
			Name: bpName,
		},
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
							p.labelKey: p.labelVal,
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

	fmt.Println(string(yamlData))
}
