package main

import (
	"fmt"

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
	bindingPolicy := BindingPolicy{
		APIVersion: "control.kubestellar.io/v1alpha1",
		Kind:       "BindingPolicy",
		Metadata: Metadata{
			Name: "wec-kwasm-bindingpolicy",
		},
		WantSingletonReportedState: true,
		ClusterSelectors: []ClusterSelector{
			{
				MatchLabels: map[string]string{
					"location-group": "edge",
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
