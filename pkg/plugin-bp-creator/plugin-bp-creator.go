package pluginBPcreator

import (
	"fmt"
	"log"
	"strings"

	c "github.com/clubanderson/labeler/pkg/common"

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

func PluginCreateBP(p c.ParamsStruct, reflect bool) []string {
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

	if p.Params[nArg] != "" {
		n = p.Params[nArg]
	}
	if p.Params["l-bp-clusterselector"] != "" {
		clusterSelectorLabelKey = strings.Split(p.Params["l-bp-clusterselector"], "=")[0]
		clusterSelectorLabelVal = strings.Split(p.Params["l-bp-clusterselector"], "=")[1]
	}
	if p.Flags["l-bp-wantsingletonreportedstate"] {
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
								p.Params["labelKey"]: p.Params["labelVal"],
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

	if p.Flags["l-debug"] {
		log.Printf("%v parameter: %v", nsArg, p.Params[nsArg])
	}

	if p.Params["l-bp-wds"] != "" {
		log.Printf("  🚀 Attempting to create %v object %q in WDS namespace %q", k, n, p.Params[nsArg])
		p.CreateObjForPlugin(gvk, yamlData, n, r, p.Params["namespaceArg"])
	} else {
		fmt.Printf("%v", string(yamlData))
	}
	return []string{}
}
