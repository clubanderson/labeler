package main

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"regexp"
	"strings"

	"github.com/pkg/errors"
	"gopkg.in/yaml.v3"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	k8sYAML "k8s.io/apimachinery/pkg/util/yaml"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/discovery/cached/disk"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/restmapper"
	"k8s.io/client-go/tools/clientcmd"
)

func (p ParamsStruct) traverseKubectlOutput(input []string) {
	mapper, _ := p.createCachedDiscoveryClient(*p.RestConfig)
	allLines := strings.Join(input, "\n")

	re := regexp.MustCompile(`([a-zA-Z0-9.-]+\/[a-zA-Z0-9.-]+) ([a-zA-Z0-9.-]+)`)
	matches := re.FindAllStringSubmatch(allLines, -1)

	namespace := p.params["namespaceArg"]

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

		resource := ResourceStruct{
			Group:      gvr.Group,
			Version:    gvr.Version,
			Resource:   gvr.Resource,
			Namespace:  namespace,
			ObjectName: objectName,
		}
		p.resources[resource] = []byte("apiVersion")
	}
}

func (p ParamsStruct) traverseHelmOutput(r io.Reader, w io.Writer) error {
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

		resource := ResourceStruct{
			Group:      gvr.Group,
			Version:    gvr.Version,
			Resource:   gvr.Resource,
			Namespace:  runtimeObj.GetNamespace(),
			ObjectName: runtimeObj.GetName(),
		}
		p.resources[resource] = yamlBytes
		// log.Printf("labeler.go: resource: %v %v\n", resource, string(yamlBytes))

		// if err != nil {
		// 	// objName := strings.ReplaceAll(runtimeObj.GetName(), "release-name-", starHelmChartReleaseName+"-")
		// 	// p.setLabel(runtimeObj.GetNamespace(), objName, gvr)
		// }

	}
	return nil
}

func (p ParamsStruct) getPluginNamesAndArgs() {
	t := reflect.TypeOf(p)
	// Iterate through the methods of the struct
	for i := 0; i < t.NumMethod(); i++ {
		// Get the method
		method := t.Method(i)
		fnValue := reflect.ValueOf(method.Func.Interface())

		if strings.HasPrefix(method.Name, "Plugin") {
			// log.Println("labeler.go: method.Name:", method.Name)
			args := []reflect.Value{reflect.ValueOf(p), reflect.ValueOf(true)}
			result := fnValue.Call(args)
			for _, rv := range result {
				v := rv.Interface()
				p.pluginArgs[method.Name] = v.([]string)
				p.pluginPtrs[method.Name] = fnValue
			}
		}
	}
}

func getFile() (*os.File, error) {
	if flags.filepath == "" {
		return nil, errors.New("labeler.go: please input a file")
	}
	if !fileExists(flags.filepath) {
		return nil, errors.New("labeler.go: the file provided does not exist")
	}
	file, e := os.Open(flags.filepath)
	if e != nil {
		return nil, errors.Wrapf(e,
			"labeler.go: unable to read the file %s", flags.filepath)
	}
	return file, nil
}

func isYAML(line string) bool {
	// Check if the line starts with "---" or starts with whitespace followed by "-"
	return strings.HasPrefix(strings.TrimSpace(line), "-") || strings.HasPrefix(line, "---")
}

func isInputFromPipe() bool {
	fileInfo, _ := os.Stdin.Stat()
	return fileInfo.Mode()&os.ModeCharDevice == 0
}

func fileExists(filename string) bool {
	_, err := os.Stat(filename)
	return err == nil
}

var print = func(v ...interface{}) {}

func logOut(v ...interface{}) {
	log.Println(v...)
}

func logNoop(v ...interface{}) {}

func (p ParamsStruct) switchContext() (*kubernetes.Clientset, *rest.Config, *dynamic.DynamicClient) {
	var err error
	var kubeConfigPath string

	if flags.kubeconfig == "" {
		kubeConfigPath = filepath.Join(p.homeDir, ".kube", "config")
	} else {
		kubeConfigPath = filepath.Join(flags.kubeconfig)
	}

	// load kubeconfig from file
	apiConfig, err := clientcmd.LoadFromFile(kubeConfigPath)
	if err != nil {
		log.Printf("labeler.go: error loading kubeconfig: %q\n", err)
		os.Exit(1)
	}

	if flags.context != "" {
		// check if the specified context exists in the kubeconfig
		if _, exists := apiConfig.Contexts[flags.context]; !exists {
			log.Printf("labeler.go: context %q does not exist in the kubeconfig\n", flags.context)
			os.Exit(1)
		}
		// switch the current context in the kubeconfig
		apiConfig.CurrentContext = flags.context
	}

	// create a new clientset with the updated config
	clientConfig := clientcmd.NewDefaultClientConfig(*apiConfig, &clientcmd.ConfigOverrides{})
	restConfig, err := clientConfig.ClientConfig()
	if err != nil {
		log.Printf("labeler.go: error creating clientset config: %v\n", err)
		os.Exit(1)
	}
	ocClientset, err := kubernetes.NewForConfig(restConfig)
	if err != nil {
		log.Printf("labeler.go: error creating clientset: %v\n", err)
		os.Exit(1)
	}
	ocDynamicClient, err := dynamic.NewForConfig(restConfig)
	if err != nil {
		log.Printf("labeler.go: error create dynamic client: %v\n", err)
		os.Exit(1)
	}

	return ocClientset, restConfig, ocDynamicClient
}

func (p ParamsStruct) createCachedDiscoveryClient(restConfigCoreOrWds rest.Config) (*restmapper.DeferredDiscoveryRESTMapper, error) {
	// create a cached discovery client for the provided config
	cachedDiscoveryClient, err := disk.NewCachedDiscoveryClientForConfig(&restConfigCoreOrWds, p.homeDir, ".cache", 60)
	if err != nil {
		log.Printf("labeler.go: could not get cacheddiscoveryclient: %v", err)
		// handle error
	}
	mapper := restmapper.NewDeferredDiscoveryRESTMapper(cachedDiscoveryClient)
	return mapper, nil
}

func (p ParamsStruct) useContext(contextName string) {
	setContext := []string{"config", "use-context", contextName}
	_, err := p.runCmd("kubectl", setContext)
	if err != nil {
		// log.Printf("   🔴 error setting kubeconfig's current context: %v\n", err)
	} else {
		log.Printf("   📍 kubeconfig's current context set to %v\n", contextName)
	}
}

func (p ParamsStruct) getGVRFromGVK(mapper *restmapper.DeferredDiscoveryRESTMapper, gvk schema.GroupVersionKind) (schema.GroupVersionResource, error) {
	mapping, err := mapper.RESTMapping(gvk.GroupKind(), gvk.Version)
	if err != nil {
		return schema.GroupVersionResource{}, fmt.Errorf("failed to get REST mapping: %v", err)
	}

	gvr := mapping.Resource

	// Check if the resource is found
	if gvr.Resource == "" {
		return schema.GroupVersionResource{}, fmt.Errorf("resource name not found for kind %s/%s %s", gvk.Group, gvk.Version, gvk.Kind)
	}

	return gvr, nil
}

func DecodeYAML(yamlBytes []byte) (*unstructured.Unstructured, error) {
	obj := &unstructured.Unstructured{}
	dec := k8sYAML.NewYAMLOrJSONDecoder(bytes.NewReader(yamlBytes), 4096)
	err := dec.Decode(obj)
	if err != nil {
		return nil, err
	}
	return obj, nil
}

func (p ParamsStruct) runCmd(cmdToRun string, cmdArgs []string) ([]byte, error) {
	cmd := exec.Command(cmdToRun, cmdArgs...)
	cmd.Env = append(cmd.Env, "PATH="+p.path)
	cmd.Env = append(cmd.Env, "HOME="+p.homeDir)

	var outputBuf bytes.Buffer
	cmd.Stdout = io.MultiWriter(&outputBuf)
	cmd.Stderr = io.MultiWriter(&outputBuf)

	err := cmd.Start()
	if err != nil {
		log.Println("labeler.go: error starting command:", err)
		return nil, err
	}

	err = cmd.Wait()
	if err != nil {
		// log.Println("labeler.go: error waiting for command to complete:", err)
		log.Printf(string(outputBuf.Bytes()))
		return nil, err
	}
	return outputBuf.Bytes(), nil
}
