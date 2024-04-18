package helpers

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"plugin"
	"reflect"
	"regexp"
	"runtime"
	"strings"

	c "github.com/clubanderson/labeler/pkg/common"
	k "github.com/clubanderson/labeler/pkg/kube-helpers"

	pluginAnnotator "github.com/clubanderson/labeler/pkg/plugin-annotator"
	pluginBPcreator "github.com/clubanderson/labeler/pkg/plugin-bp-creator"
	pluginHelp "github.com/clubanderson/labeler/pkg/plugin-help"
	pluginLabeler "github.com/clubanderson/labeler/pkg/plugin-labeler"
	pluginOCMcreator "github.com/clubanderson/labeler/pkg/plugin-ocm-creator"
	pluginRemoteDeploy "github.com/clubanderson/labeler/pkg/plugin-remote-deploy"

	// add other compile-time plugins here

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

type PluginFunction struct {
	Name     string
	Function interface{}
}

var pluginFunctions = []interface{}{
	pluginHelp.PluginHelp,
	pluginBPcreator.PluginCreateBP,
	pluginOCMcreator.PluginCreateMW,
	pluginRemoteDeploy.PluginRemoteDeployTo,
	pluginLabeler.PluginLabeler,
	pluginAnnotator.PluginAnnotator,
	// add other plugin functions here as needed
}

func AliasRun(args []string, p c.ParamsStruct) error {
	// args = os.Args[1:]
	p.Flags = make(map[string]bool)
	p.Params = make(map[string]string)
	p.Resources = make(map[c.ResourceStruct][]byte)
	p.PluginArgs = make(map[string][]string)
	p.PluginPtrs = make(map[string]reflect.Value)

	getPluginNamesAndArgs(p)

	p.Flags[args[0]] = true
	for i, arg := range args {
		if strings.HasPrefix(arg, "--") || strings.HasPrefix(arg, "-") {
			if i < len(args)-1 && !strings.HasPrefix(args[i+1], "-") {
				if strings.Contains(arg, "=") {
					parts := strings.Split(arg, "=")
					p.Params[parts[0][2:]] = parts[1]
				} else {
					p.Params[arg[1:]] = args[i+1]
				}
			} else if strings.Contains(arg, "=") {
				parts := strings.Split(arg, "=")
				if len(parts) > 2 {
					p.Params[parts[0][2:]] = parts[1] + "=" + parts[2]
				} else {
					p.Params[parts[0][2:]] = parts[1]
				}
			} else {
				if strings.HasPrefix(arg, "--") {
					p.Flags[arg[2:]] = true
				} else {
					p.Flags[arg[1:]] = true
				}
			}
		} else if strings.HasPrefix(arg, "install") ||
			strings.HasPrefix(arg, "upgrade") ||
			strings.HasPrefix(arg, "template") ||
			strings.HasPrefix(arg, "apply") ||
			strings.HasPrefix(arg, "create") ||
			strings.HasPrefix(arg, "delete") ||
			strings.HasPrefix(arg, "get") ||
			strings.HasPrefix(arg, "describe") ||
			strings.HasPrefix(arg, "edit") ||
			strings.HasPrefix(arg, "exec") ||
			strings.HasPrefix(arg, "logs") ||
			strings.HasPrefix(arg, "port-forward") ||
			strings.HasPrefix(arg, "replace") ||
			strings.HasPrefix(arg, "rollout") ||
			strings.HasPrefix(arg, "scale") ||
			strings.HasPrefix(arg, "set") ||
			strings.HasPrefix(arg, "top") ||
			strings.HasPrefix(arg, "expose") ||
			strings.HasPrefix(arg, "autoscale") ||
			strings.HasPrefix(arg, "attach") ||
			strings.HasPrefix(arg, "exec") ||
			strings.HasPrefix(arg, "wait") ||
			strings.HasPrefix(arg, "cp") ||
			strings.HasPrefix(arg, "run") ||
			strings.HasPrefix(arg, "label") ||
			strings.HasPrefix(arg, "annotate") ||
			strings.HasPrefix(arg, "patch") ||
			strings.HasPrefix(arg, "delete") ||
			strings.HasPrefix(arg, "create") ||
			strings.HasPrefix(arg, "replace") ||
			strings.HasPrefix(arg, "edit") {
			p.Flags[arg] = true
		}
	}

	// Print flags and params
	if p.Flags["l-debug"] {
		log.Println("labeler.go: [debug] Flags:")
		for flag, value := range p.Flags {
			log.Printf("labeler.go: [debug] %s: %t\n", flag, value)
		}

		log.Println("\nlabeler.go: [debug] Params:")
		for param, value := range p.Params {
			log.Printf("labeler.go: [debug] %s: %s\n", param, value)
		}
		log.Println()
	}

	k.AddNamespaceToResources(p)

	if args[0] == "k" || args[0] == "kubectl" || args[0] == "helm" {

		if p.Flags["l-debug"] {
			log.Printf("labeler.go: [debug] namespaceArg: %v", p.Params["namespaceArg"])
		}
		// remove the following args for both helm and kubectl because they do not recognize them
		for i := 0; i < len(args); i++ {
			// remove all labeler flags
			if strings.HasPrefix(args[i], "--l-") {
				args = append(args[:i], args[i+1:]...)
				i--
			}
			if strings.HasPrefix(args[i], "--label") {
				if strings.Contains(args[i], "=") {
					p.Params["labelKey"] = strings.Split(args[i], "=")[1]
					p.Params["labelVal"] = strings.Split(args[i], "=")[2]
					args = append(args[:i], args[i+1:]...)
					i--
				}
			}
			if strings.HasPrefix(args[i], "--annotation") {
				if strings.Contains(args[i], "=") {
					p.Params["annotationKey"] = strings.Split(args[i], "=")[1]
					p.Params["annotationVal"] = strings.Split(args[i], "=")[2]
					// args = append(args[:i], args[i+1:]...)
					// i--
				}
			}
			if strings.HasPrefix(args[i], "-l") {
				if len(args) > i+1 && !strings.HasPrefix(args[i+1], "-") {
					p.Params["labelKey"] = strings.Split(args[i+1], "=")[0]
					p.Params["labelVal"] = strings.Split(args[i+1], "=")[1]
					args = append(args[:i], args[i+2:]...)
					i--
					i--
				}
			}
		}
		if p.Flags["l-debug"] {
			log.Println("labeler.go: [debug] before args: ", args)
		}

		// Run the command with the parsed flags
		if args[0] == "k" || args[0] == "kubectl" {
			if p.Flags["l-debug"] {
				log.Println("labeler.go: [debug] after args: ", args)
			}

			originalCommand := strings.Join(args, " ")
			p.OriginalCmd = originalCommand

			// cmd := exec.Command(args[0], args[1:]...)
			out, err := p.RunCmd(args[0], args[1:], false)
			// out, err := cmd.CombinedOutput()
			if err != nil {
				// fmt.Printf("%v", string(out))
				os.Exit(1)
			}

			p.ClientSet, p.RestConfig, p.DynamicClient = SwitchContext(p)
			output := strings.TrimSpace(string(out))
			lines := strings.Split(output, "\n")
			traverseKubectlOutput(lines, p)

		} else if args[0] == "helm" {
			// run the original helm command without the extra labeler flags
			_, err := p.RunCmd("helm", args[1:], false)
			if err != nil {
				os.Exit(1)
			}

			// now run helm as template and collect output
			templateOutput := runHelmInTemplateMode(args, p)

			// set the context and get the helm output into the resources map
			p.ClientSet, p.RestConfig, p.DynamicClient = SwitchContext(p)
			err = traverseHelmOutput(strings.NewReader(string(templateOutput)), p)
			if err != nil {
				log.Println("labeler.go: error (to traverseInput):", err)
				return err
			}

		}

		combinedFlagsAndParams := make(map[string]bool)
		for key, value := range p.Flags {
			combinedFlagsAndParams[key] = value
		}
		for key := range p.Params {
			combinedFlagsAndParams[key] = true
		}
		if p.Flags["l-debug"] {
			for key, value := range p.PluginPtrs {
				log.Printf("labeler.go: key: %v, value: %v\n", key, value)
			}
		}

		fnArgs := []reflect.Value{reflect.ValueOf(p), reflect.ValueOf(false)}

		for key := range combinedFlagsAndParams {
			for pkey, value := range p.PluginArgs {
				for _, vCSV := range value {
					v := strings.Split(vCSV, ",")
					if key == v[0] {
						if p.PluginPtrs[pkey].IsValid() {
							log.Printf("\nlabeler plugin: %q:\n\n", pkey)
							p.PluginPtrs[pkey].Call(fnArgs)
						}
					}
				}
			}
		}
		if p.Flags["l-debug"] {
			for key, value := range p.Resources {
				fmt.Printf("labeler.go: [debug] resources: Key: %s, Value: %s\n", key, value)
			}
		}
	}
	return nil
}

func traverseKubectlOutput(input []string, p c.ParamsStruct) {
	mapper, _ := createCachedDiscoveryClient(*p.RestConfig, p)
	allLines := strings.Join(input, "\n")

	re := regexp.MustCompile(`([a-zA-Z0-9.-]+\/[a-zA-Z0-9.-]+) ([a-zA-Z0-9.-]+)`)
	matches := re.FindAllStringSubmatch(allLines, -1)

	namespace := p.Params["namespaceArg"]

	// if c.Flags.Label == "" && p.Params["labelKey"] == "" {
	// 	if p.Flags["l-debug"] {
	// 		log.Println("labeler.go: no label provided")
	// 	}
	// 	return
	// }
	// if c.Flags.Label != "" {
	// 	p.Params["labelKey"], p.Params["labelVal"] = strings.Split(c.Flags.Label, "=")[0], strings.Split(c.Flags.Label, "=")[1]
	// }

	if len(matches) == 0 {
		if p.Flags["l-debug"] {
			log.Println("labeler.go: no resources found")
		}
		return
	}

	// iterate over matches and extract group version kind and object name
	for _, match := range matches {
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
		gvr, err := getGVRFromGVK(mapper, gvk)
		if err != nil {
			if p.Flags["l-debug"] {
				log.Printf("labeler.go: error getting gvr from gvk for %v/%v/%v: %v\n", gvk.Group, gvk.Version, gvk.Kind, err)
			}
		}

		resource := c.ResourceStruct{
			Group:      gvr.Group,
			Version:    gvr.Version,
			Resource:   gvr.Resource,
			Namespace:  namespace,
			ObjectName: objectName,
		}
		addObjectsToResourcesAfterKubectlApply(resource, p)
	}
}

func addObjectsToResourcesAfterKubectlApply(r c.ResourceStruct, p c.ParamsStruct) {
	gvr := schema.GroupVersionResource{
		Group:    r.Group,
		Version:  r.Version,
		Resource: r.Resource,
	}
	// log.Printf("labeler.go: getting object %v/%v/%v %q\n", gvr.Group, gvr.Version, gvr.Resource, r.ObjectName)
	yamlBytes, err := p.GetObject(p.DynamicClient, r.Namespace, gvr, r.ObjectName)
	if err != nil {
		log.Printf("labeler.go: error getting object: %v\n", err)
		// os.Exit(1)
	}
	// Define the fields to remove from metadata
	fieldsToRemove := []string{"creationTimestamp", "generation", "managedFields", "resourceVersion", "selfLink", "uid"}
	annotationsToRemove := []string{"kubectl.kubernetes.io/last-applied-configuration"}

	// Unmarshal YAML into a map
	var objMap map[string]interface{}
	err = yaml.Unmarshal(yamlBytes, &objMap)
	if err != nil {
		log.Printf("labeler.go: error unmarshaling YAML: %v\n", err)
		return
	}

	// Check if metadata field exists
	metadata, ok := objMap["metadata"].(map[string]interface{})
	if !ok {
		log.Println("labeler.go: metadata field not found or not a map[string]interface{}")
		return
	}

	// Remove specified fields from metadata
	for _, field := range fieldsToRemove {
		delete(metadata, field)
	}

	// Check if annotations field exists within metadata
	annotations, ok := metadata["annotations"].(map[string]interface{})
	if !ok {
		log.Println("labeler.go: annotations field not found or not a map[string]interface{}")
		return
	}

	// Remove the specified annotation
	for _, field := range annotationsToRemove {
		delete(annotations, field)
	}

	// Marshal the modified object back to YAML
	modifiedYAMLBytes, err := yaml.Marshal(objMap)
	if err != nil {
		log.Printf("labeler.go: error marshaling YAML: %v\n", err)
		return
	}

	p.Resources[r] = modifiedYAMLBytes
}

func runHelmInTemplateMode(args []string, p c.ParamsStruct) []byte {
	originalCommand := strings.Join(args, " ")
	p.OriginalCmd = originalCommand

	if p.Flags["l-debug"] {
		log.Printf("labeler.go: [debug] original command: %v\n", originalCommand)
	}
	modifiedCommand := strings.Replace(originalCommand, " install ", " template ", 1)
	modifiedCommand = strings.Replace(modifiedCommand, " upgrade ", " template ", 1)
	modifiedCommandComponents := strings.Split(modifiedCommand, " ")[1:]
	if p.Flags["l-debug"] {
		log.Printf("labeler.go: [debug] modified command components: %v\n", modifiedCommandComponents)
	}

	output, err := p.RunCmd("helm", modifiedCommandComponents, true)
	if err != nil {
		// log.Println("labeler.go: error (run helm):", err)
		os.Exit(1)
	}
	return output
}

func traverseHelmOutput(r io.Reader, p c.ParamsStruct) error {
	mapper, _ := createCachedDiscoveryClient(*p.RestConfig, p)

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

	decoder := yaml.NewDecoder(strings.NewReader(allLines))
	for {
		var obj map[string]interface{}
		err := decoder.Decode(&obj)
		if err != nil {
			if err.Error() != "EOF" && !strings.Contains(err.Error(), "did not find expected alphabetic or numeric character") {
				_ = err
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
		runtimeObj, err := decodeYAML(yamlBytes)
		if err != nil {
			// log.Printf("labeler.go: error decoding yaml: %v\n", err)
			continue
		}
		gvk := runtimeObj.GroupVersionKind()
		// log.Printf("labeler.go: G: %v, V: %v, K: %v, Name: %v", gvk.Group, gvk.Version, gvk.Kind, runtimeObj.GetName())

		gvr, err := getGVRFromGVK(mapper, gvk)
		if err != nil {
			if p.Flags["l-debug"] {
				log.Printf("labeler.go: error getting gvr from gvk for %v/%v/%v. Retrying in 5 seconds: %v\n", gvk.Group, gvk.Version, gvk.Kind, err)
			}
		}

		resource := c.ResourceStruct{
			Group:      gvr.Group,
			Version:    gvr.Version,
			Resource:   gvr.Resource,
			Namespace:  runtimeObj.GetNamespace(),
			ObjectName: runtimeObj.GetName(),
		}
		p.Resources[resource] = yamlBytes
		// log.Printf("labeler.go: resource: %v %v\n", resource, string(yamlBytes))

		// if err != nil {
		// 	// objName := strings.ReplaceAll(runtimeObj.GetName(), "release-name-", starHelmChartReleaseName+"-")
		// 	// p.setLabel(runtimeObj.GetNamespace(), objName, gvr)
		// }

	}
	return nil
}

func getPluginNamesAndArgs(p c.ParamsStruct) {
	//
	// COMPILE-TIME PLUGIN DISCOVERY SECTION
	//
	for _, pluginFunc := range pluginFunctions {
		ptr := reflect.ValueOf(pluginFunc).Pointer()
		methodName := runtime.FuncForPC(ptr).Name()
		parts := strings.Split(methodName, ".")
		methodName = parts[len(parts)-1]
		fnValue := reflect.ValueOf(pluginFunc)

		if strings.HasPrefix(methodName, "Plugin") {
			args := []reflect.Value{reflect.ValueOf(p), reflect.ValueOf(true)}
			result := fnValue.Call(args)
			for _, rv := range result {
				v := rv.Interface()
				p.PluginArgs[methodName] = v.([]string)
				p.PluginPtrs[methodName] = fnValue
			}
		}
	}

	//
	// RUNTIME PLUGIN DISCOVERY SECTION
	//
	exePath, err := os.Executable()
	if err != nil {
		fmt.Println("Error getting executable path:", err)
		os.Exit(1)
	}

	// Get the directory containing the executable
	exeDir := filepath.Dir(exePath)

	// Read the files in the directory
	files, err := os.ReadDir(exeDir)
	if err != nil {
		fmt.Println("Error reading directory:", err)
		os.Exit(1)
	}

	// Load and run plugins
	for _, file := range files {
		if file.IsDir() {
			continue
		}

		if match, _ := filepath.Match("labeler-*", file.Name()); match {
			log.Println("*****labeler.go: Found plugin:", file.Name())
			// load plugin
			pi, err := plugin.Open(filepath.Join(exeDir, file.Name()))
			if err != nil {
				fmt.Println("Error opening plugin:", err)
				continue
			}

			// lookup symbol
			sym, err := pi.Lookup("PluginRun")
			if err != nil {
				fmt.Println("Error looking up symbol:", err)
				continue
			}

			// assert and call plugin method
			pluginImpl, ok := sym.(func() []string)
			if !ok {
				fmt.Println("Error: unexpected type from module symbol")
				continue
			}
			pluginFNnames := pluginImpl()
			log.Println("Plugin function names:", pluginFNnames)

			for _, methodName := range pluginFNnames {
				sym, err = pi.Lookup(methodName)
				if err != nil {
					fmt.Println("Error looking up symbol:", err)
					continue
				}
				pluginReflect, ok := sym.(func(c.ParamsStruct, bool) []string)
				if !ok {
					fmt.Println("Error: unexpected type from module symbol")
					continue
				}
				pluginArgs := pluginReflect(p, true)
				p.PluginArgs[methodName] = pluginArgs
				p.PluginPtrs[methodName] = reflect.ValueOf(pluginReflect)
				log.Println("Plugin args:", pluginFNnames)
			}
		}
	}
}

func getFile() (*os.File, error) {
	if c.Flags.Filepath == "" {
		return nil, errors.New("labeler.go: please input a file")
	}
	if !fileExists(c.Flags.Filepath) {
		return nil, errors.New("labeler.go: the file provided does not exist")
	}
	file, e := os.Open(c.Flags.Filepath)
	if e != nil {
		return nil, errors.Wrapf(e,
			"labeler.go: unable to read the file %s", c.Flags.Filepath)
	}
	return file, nil
}

func isYAML(line string) bool {
	// Check if the line starts with "---" or starts with whitespace followed by "-"
	return strings.HasPrefix(strings.TrimSpace(line), "-") || strings.HasPrefix(line, "---")
}

func IsInputFromPipe() bool {
	fileInfo, _ := os.Stdin.Stat()
	return fileInfo.Mode()&os.ModeCharDevice == 0
}

func fileExists(filename string) bool {
	_, err := os.Stat(filename)
	return err == nil
}

func SwitchContext(p c.ParamsStruct) (*kubernetes.Clientset, *rest.Config, *dynamic.DynamicClient) {
	var err error
	var kubeConfigPath string

	if c.Flags.Kubeconfig == "" {
		kubeConfigPath = filepath.Join(p.HomeDir, ".kube", "config")
	} else {
		kubeConfigPath = filepath.Join(c.Flags.Kubeconfig)
	}

	// load kubeconfig from file
	apiConfig, err := clientcmd.LoadFromFile(kubeConfigPath)
	if err != nil {
		log.Printf("labeler.go: error loading kubeconfig: %q\n", err)
		os.Exit(1)
	}

	// log.Printf("labeler.go: current context: %q %q %q\n", apiConfig.CurrentContext, c.Flags.Context, p.Params["context"])
	if p.Params["context"] != "" {
		// check if the specified context exists in the kubeconfig
		if _, exists := apiConfig.Contexts[p.Params["context"]]; !exists {
			log.Printf("labeler.go: context %q does not exist in the kubeconfig\n", p.Params["context"])
			os.Exit(1)
		}
		// switch the current context in the kubeconfig
		apiConfig.CurrentContext = p.Params["context"]
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

func createCachedDiscoveryClient(restConfigCoreOrWds rest.Config, p c.ParamsStruct) (*restmapper.DeferredDiscoveryRESTMapper, error) {
	// create a cached discovery client for the provided config
	cachedDiscoveryClient, err := disk.NewCachedDiscoveryClientForConfig(&restConfigCoreOrWds, p.HomeDir, ".cache", 60)
	if err != nil {
		log.Printf("labeler.go: could not get cacheddiscoveryclient: %v", err)
		// handle error
	}
	mapper := restmapper.NewDeferredDiscoveryRESTMapper(cachedDiscoveryClient)
	return mapper, nil
}

func getGVRFromGVK(mapper *restmapper.DeferredDiscoveryRESTMapper, gvk schema.GroupVersionKind) (schema.GroupVersionResource, error) {
	mapping, err := mapper.RESTMapping(gvk.GroupKind(), gvk.Version)
	if err != nil {
		return schema.GroupVersionResource{}, fmt.Errorf("failed to get REST mapping: %v", err)
	}
	gvr := mapping.Resource
	if gvr.Resource == "" {
		return schema.GroupVersionResource{}, fmt.Errorf("resource name not found for kind %s/%s %s", gvk.Group, gvk.Version, gvk.Kind)
	}

	return gvr, nil
}

func decodeYAML(yamlBytes []byte) (*unstructured.Unstructured, error) {
	obj := &unstructured.Unstructured{}
	dec := k8sYAML.NewYAMLOrJSONDecoder(bytes.NewReader(yamlBytes), 4096)
	err := dec.Decode(obj)
	if err != nil {
		return nil, err
	}
	return obj, nil
}
