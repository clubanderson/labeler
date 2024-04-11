package main

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"regexp"
	"strings"

	"gopkg.in/yaml.v2"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/kubernetes"
)

func (p ParamsStruct) PluginLabeler(reflect bool) []string {
	return []string{}
}

func (p ParamsStruct) setLabelNamespace() error {
	gvr := schema.GroupVersionResource{
		Group:    "",
		Version:  "v1",
		Resource: "namespaces",
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

	// todo - this logic is not working right. should only label the namespace if installmode is true and dryrunmode is false - I am not doing something right in the following if statement
	// because it is always true - and I am not sure why
	// log.Printf("labeler.go: dryrunMode: %v, templateMode: %v, installMode: %v\n", p.dryrunMode, p.templateMode, p.installMode)
	namespace := ""
	if p.params["namespace"] != "" {
		namespace = p.params["namespace"]
	} else if p.params["n"] != "" {
		namespace = p.params["n"]
	}

	if p.flags["install"] && !p.flags["dry-run"] {
		// log.Printf("labeler.go: patching namespace %q with %v=%v %q %q %q %v\n", p.namespace, p.params["labelKey"], p.params["labelVal"], gvr.Resource, gvr.Version, gvr.Group, string(patch))
		_, err = p.DynamicClient.Resource(gvr).Patch(context.TODO(), namespace, types.MergePatchType, patch, metav1.PatchOptions{})
	}
	if err != nil {
		if p.flags["install"] && !p.flags["dry-run"] {
			labelCmd := fmt.Sprintf("kubectl label %v %v %v=%v\n", gvr.Resource, namespace, p.params["labelKey"], p.params["labelVal"])
			runResults.didNotLabel = append(runResults.didNotLabel, labelCmd)
		}
	} else {
		log.Printf("  🏷️ labeled object %v/%v/%v %q with %v=%v\n", gvr.Group, gvr.Version, gvr.Resource, namespace, p.params["labelKey"], p.params["labelVal"])
	}
	p.resources[gvr.Group+"/"+gvr.Version+"/"+gvr.Resource+"/"+namespace+"/"] = "apiVersion"

	return nil
}

func (p ParamsStruct) traverseInputAndLabel(r io.Reader, w io.Writer) error {
	mapper, _ := p.createCachedDiscoveryClient(*p.RestConfig)

	var linesOfOutput []string

	scanner := bufio.NewScanner(bufio.NewReader(r))
	for scanner.Scan() {
		linesOfOutput = append(linesOfOutput, scanner.Text())
	}
	allLines := strings.Join(linesOfOutput, "\n")

	if i := strings.Index(allLines, "---\n"); i != -1 {
		// slice the concatenated string from the index of "---\n"
		allLines = allLines[i:]
	}

	// Convert the sliced string back to a string slice
	linesOfOutput = strings.Split(allLines, "\n")

	decoder := yaml.NewDecoder(strings.NewReader(allLines))
	for {
		var obj map[string]interface{}
		err := decoder.Decode(&obj)
		if err != nil {
			if err.Error() != "EOF" && !strings.Contains(err.Error(), "did not find expected alphabetic or numeric character") {
				// log.Printf("labeler.go: decoding error: %v\n%v\n", err, obj)
			}
			break // reached end of file or error
		}

		// convert map to YAML byte representation
		yamlBytes, err := yaml.Marshal(obj)
		if err != nil {
			log.Printf("labeler.go: error marshaling YAML: %v\n", err)
			continue
		}
		runtimeObj, err := DecodeYAML(yamlBytes)
		if err != nil {
			// log.Printf("labeler.go: error decoding yaml: %v\n", err)
			continue
		}
		gvk := runtimeObj.GroupVersionKind()
		// log.Printf("labeler.go: G: %v, V: %v, K: %v, Name: %v", gvk.Group, gvk.Version, gvk.Kind, runtimeObj.GetName())

		gvr, err := p.getGVRFromGVK(mapper, gvk)
		if err != nil {
			if p.flags["l-debug"] {
				log.Printf("labeler.go: error getting gvr from gvk for %v/%v/%v. Retrying in 5 seconds: %v\n", gvk.Group, gvk.Version, gvk.Kind, err)
			}
		}

		err = p.setLabel(runtimeObj.GetNamespace(), runtimeObj.GetName(), gvr)
		if err != nil {
			// objName := strings.ReplaceAll(runtimeObj.GetName(), "release-name-", starHelmChartReleaseName+"-")
			// p.setLabel(runtimeObj.GetNamespace(), objName, gvr)
		}

	}
	return nil
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

	p.resources[gvr.Group+"/"+gvr.Version+"/"+gvr.Resource+"/"+objectName+"/"+namespace] = "apiVersion"

	if err != nil {
		if namespace != "" {
			labelCmd := fmt.Sprintf("kubectl label %v %v %v=%v -n %v\n", gvr.Resource, objectName, p.params["labelKey"], p.params["labelVal"], namespace)
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

func (p ParamsStruct) setLabelKubectl(input []string) {
	mapper, _ := p.createCachedDiscoveryClient(*p.RestConfig)
	allLines := strings.Join(input, "\n")

	re := regexp.MustCompile(`([a-zA-Z0-9.-]+\/[a-zA-Z0-9.-]+) ([a-zA-Z0-9.-]+)`)
	matches := re.FindAllStringSubmatch(allLines, -1)

	namespace := ""
	if p.params["namespace"] != "" {
		namespace = p.params["namespace"]
	} else if p.params["n"] != "" {
		namespace = p.params["n"]
	}
	if namespace == "" {
		namespace = "default" // this needs to be the value given to kubectl - if empty, then it is default
	}

	if flags.label == "" && p.params["labelKey"] == "" {
		if p.flags["l-debug"] {
			log.Println("labeler.go: no label provided")
		}
		return
	}
	if flags.label != "" {
		p.params["labelKey"], p.params["labelVal"] = strings.Split(flags.label, "=")[0], strings.Split(flags.label, "=")[1]
	}

	if len(matches) == 0 {
		if p.flags["l-debug"] {
			log.Println("labeler.go: no resources found to label")
		}
		return
	}

	// iterate over matches and extract group version kind and object name
	for _, match := range matches {
		var labelCmd []string
		// log.Printf("labeler.go: match: %v\n", match)
		// the first match group contains the group kind and object name
		groupKindObjectName := match[1]
		// split the string to get group version kind and object name
		parts := strings.Split(groupKindObjectName, "/")
		gvkParts := strings.Split(parts[0], ".")
		k := gvkParts[0]
		g := ""
		v := ""
		if len(gvkParts) >= 1 {
			g = strings.Join(gvkParts[1:], ".")
		}
		objectName := parts[1]
		// log.Printf("labeler.go: g: %s, k: %s, ObjectName: %s", g, k, objectName)
		labelCmd = []string{"-n", namespace, "label", k + "/" + objectName, p.params["labelKey"] + "=" + p.params["labelVal"]}
		if flags.context != "" {
			labelCmd = append(labelCmd, "--context="+flags.context)
			// labelCmd = []string{"--context=" + flags.context, "-n", namespace, "label", kind + "/" + objectName, p.params["labelKey"] + "=" + p.params["labelVal"], "--overwrite"}
		}
		if p.flags["overwrite"] || flags.overwrite {
			labelCmd = append(labelCmd, "--overwrite")
		}
		if p.params["context"] != "" {
			labelCmd = append(labelCmd, "--context="+p.params["context"])
		}
		if p.params["kube-context"] != "" {
			labelCmd = append(labelCmd, "--context="+p.params["kube-context"])
		}
		if p.params["kubeconfig"] != "" {
			labelCmd = append(labelCmd, "--kubeconfig="+p.params["kubeconfig"])
		}

		client, _ := kubernetes.NewForConfig(p.RestConfig)
		res, _ := discovery.ServerPreferredResources(client.Discovery())
		for _, resList := range res {
			for _, r := range resList.APIResources {
				// fmt.Printf("%q %q %q\n", r.Group, r.Version, r.Kind)
				if strings.ToLower(r.Group) == strings.ToLower(g) && strings.ToLower(r.Kind) == strings.ToLower(k) {
					if r.Version == "" {
						v = "v1"
					} else {
						v = r.Version
					}
					break
				}
			}
		}
		// log.Printf("labeler.go: labelCmd: %v\n", labelCmd)
		gvk := schema.GroupVersionKind{
			Group:   g,
			Version: v,
			Kind:    k,
		}
		gvr, err := p.getGVRFromGVK(mapper, gvk)
		if err != nil {
			if p.flags["l-debug"] {
				log.Printf("labeler.go: error getting gvr from gvk for %v/%v/%v: %v\n", gvk.Group, gvk.Version, gvk.Kind, err)
			}
		}

		p.resources[gvr.Group+"/"+gvr.Version+"/"+gvr.Resource+"/"+objectName+"/"+namespace] = "apiVersion"

		output, err := p.runCmd("kubectl", labelCmd)
		if err != nil {
			// log.Printf("labeler.go: label did not apply due to error: %v", err)
		} else {
			if strings.Contains(string(output), "not labeled") {
				log.Printf("  %v already has label %v=%v", strings.Split(string(output), " ")[0], p.params["labelKey"], p.params["labelVal"])
			} else {
				log.Printf("  🏷️ created and labeled object %q in namespace %q with %v=%v\n", objectName, namespace, p.params["labelKey"], p.params["labelVal"])
			}
		}
	}
}
