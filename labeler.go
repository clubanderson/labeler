package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"runtime"
	"strings"

	"gopkg.in/yaml.v3"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	k8sYAML "k8s.io/apimachinery/pkg/util/yaml"

	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"k8s.io/client-go/discovery/cached/disk"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/restmapper"
	"k8s.io/client-go/tools/clientcmd"
)

type ParamsStruct struct {
	homeDir              string
	path                 string
	debug                bool
	labelerClientSet     *kubernetes.Clientset
	labelerRestConfig    *rest.Config
	labelerDynamicClient *dynamic.DynamicClient
}

var flags struct {
	filepath   string
	debug      bool
	verbose    bool
	label      string
	kubeconfig string
	context    string
}

var flagsName = struct {
	file            string
	fileShort       string
	verbose         string
	verboseShort    string
	debug           string
	debugShort      string
	label           string
	labelShort      string
	kubeconfig      string
	kubeconfigShort string
	context         string
	contextShort    string
}{
	file:            "file",
	fileShort:       "f",
	verbose:         "verbose",
	verboseShort:    "v",
	debug:           "debug",
	debugShort:      "d",
	label:           "label",
	labelShort:      "l",
	kubeconfig:      "kubeconfig",
	kubeconfigShort: "k",
	context:         "context",
	contextShort:    "c",
}

// need a new function to do labeling

func (p ParamsStruct) detectInput() error {
	var yamlData interface{}

	if isInputFromPipe() {
		// if input is from a pipe, traverseinput and label the
		// content of stdin
		log.Println("data is from pipe")
		// try to use output - might be yaml from --debug
		// if not --debug, then it is 'helm install', or 'helm template', or 'helm install --dry-run'
		// 'helm install' produces no yaml output
		// 'helm template' and 'helm install --dry-run' produce yaml - but they do not apply resources - and this might be the intent,
		// but labeling will fail if resources are not created, or are not present from a previous run of helm
		// '--debug' allows for install and yaml output - good combination we should check for first
		// 'template' and 'install --dry-run' are good also - but be prepared for failing to label if resources are missing
		// so, lets see if we got some yaml first - then behave nicely if labeling fails and instruct on how to run helm again with --debug piped into labeler
		// and, if there is no yaml input at all - return with info on how to use with helm with --debug and labeler
		// Read the input
		scanner := bufio.NewScanner(os.Stdin)
		var input []byte
		for scanner.Scan() {
			line := scanner.Text()
			if !isYAML(line) {
				continue // Skip non-YAML lines
			}
			break
			// input = append(input, scanner.Bytes()...)
			// input = append(input, '\n') // Append newline to separate lines
		}

		for scanner.Scan() {
			input = append(input, scanner.Bytes()...)
			input = append(input, '\n') // Append newline to separate lines
		}

		// Check for scanner error
		if err := scanner.Err(); err != nil {
			log.Printf("error reading input: %v", err)
			return nil
		}

		// Try parsing the input as YAML
		if err := yaml.Unmarshal(input, &yamlData); err != nil {
			log.Printf("warning: no YAML input was detected %v", err)
		}

		// Check if YAML was provided
		if yamlData != nil {
			log.Println("YAML data detected in stdin")
			// Do something with the YAML data received - don't need to use history hack in this case - we got valid YAML input from template, --dry-run, or --debug
			err := p.traverseInputAndLabel(strings.NewReader(string(input)), os.Stdout)
			if err != nil {
				log.Println("Error (traverseinput):", err)
				return err
			}
		} else {
			log.Println("No YAML data detected in stdin, will try to run again with YAML output")
			// time to do it the hard way - many may not like this approach (history hack) - the other options above are more than sufficient for most people's use
			return p.helmOrKubectl(os.Stdin, os.Stdout)
		}
	} else {
		// ...otherwise get the file
		log.Println("data is from file")
		file, e := getFile()
		if e != nil {
			return e
		}
		defer file.Close()
		return p.helmOrKubectl(file, os.Stdout)
	}

	return nil

}

func isYAML(line string) bool {
	// Check if the line starts with "---" or starts with whitespace followed by "-"
	return strings.HasPrefix(strings.TrimSpace(line), "-") || strings.HasPrefix(line, "---")
}

func isInputFromPipe() bool {
	fileInfo, _ := os.Stdin.Stat()
	return fileInfo.Mode()&os.ModeCharDevice == 0
}

func getFile() (*os.File, error) {
	if flags.filepath == "" {
		return nil, errors.New("please input a file")
	}
	if !fileExists(flags.filepath) {
		return nil, errors.New("the file provided does not exist")
	}
	file, e := os.Open(flags.filepath)
	if e != nil {
		return nil, errors.Wrapf(e,
			"unable to read the file %s", flags.filepath)
	}
	return file, nil
}

func (p ParamsStruct) helmOrKubectl(r io.Reader, w io.Writer) error {
	originalCommand, err := p.getOriginalCommandFromHistory()
	if err != nil {
		log.Println("Error (get history):", err)
		// os.Exit(1)
	}

	log.Printf("Original command: %q\n", originalCommand)
	isHelm, isKubectl, isKustomize := false, false, false

	if strings.HasPrefix(originalCommand, "helm") {
		log.Printf("your running helm\n")
		isHelm = true
	} else if strings.HasPrefix(originalCommand, "kubectl") {
		log.Printf("your running kubectl\n")
	}

	if isHelm {
		modifiedCommand := strings.Replace(originalCommand, "install", "template", 1)
		modifiedCommandComponents := append(strings.Split(modifiedCommand, " ")[1:])
		output, err := p.runCmd("helm", modifiedCommandComponents)
		if err != nil {
			log.Println("Error (run helm):", err)
			os.Exit(1)
		}
		err = p.traverseInputAndLabel(strings.NewReader(string(output)), os.Stdout)
		if err != nil {
			log.Println("Error (to traverseInput):", err)
			return err
		}
	} else {
		if strings.Contains(originalCommand, "-k") {
			isKustomize = true
			_ = isKustomize
			// this is kustomize - not sure if there is a reason to treat this differently then regular kubectl
			// HACKME!!!!
		} else {
			isKubectl = true
			_ = isKubectl
			// this is plain kubectl
			// HACKME!!!!
		}
		p.traverseInputAndLabel(r, w)
		if err != nil {
			log.Println("Error (traverseinput):", err)
			os.Exit(1)
		}
	}
	return nil
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

func (p ParamsStruct) traverseInputAndLabel(r io.Reader, w io.Writer) error {
	mapper, _ := p.createCachedDiscoveryClient(*p.labelerRestConfig)

	var linesOfOutput []string

	scanner := bufio.NewScanner(bufio.NewReader(r))
	for scanner.Scan() {
		linesOfOutput = append(linesOfOutput, scanner.Text())
	}
	allLines := strings.Join(linesOfOutput, "\n")

	if i := strings.Index(allLines, "---\n"); i != -1 {
		// Slice the concatenated string from the index of "---\n"
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
				log.Printf("      🔴 decoding error: %v\n%v\n", err, obj)
			}
			break // Reached end of file or error
		}

		// convert map to YAML byte representation
		yamlBytes, err := yaml.Marshal(obj)
		if err != nil {
			log.Printf("Error marshaling YAML: %v\n", err)
			continue
		}
		runtimeObj, err := DecodeYAML(yamlBytes)
		if err != nil {
			// log.Printf("      🔴 error decoding yaml: %v\n", err)
			continue
		}
		gvk := runtimeObj.GroupVersionKind()
		_ = gvk
		// log.Printf("G: %v, V: %v, K: %v, Name: %v", gvk.Group, gvk.Version, gvk.Kind, runtimeObj.GetName())

		gvr, err := p.getGVRFromGVK(mapper, gvk)
		if err != nil {
			if p.debug {
				log.Printf("          🟡 error getting gvr from gvk for %v/%v/%v. Retrying in 5 seconds: %v\n", gvk.Group, gvk.Version, gvk.Kind, err)
			}
		}

		err = p.setLabel(runtimeObj.GetNamespace(), runtimeObj.GetName(), gvr)
		if err != nil {
			// 	objName := strings.ReplaceAll(runtimeObj.GetName(), "release-name-", starHelmChartReleaseName+"-")
			// 	p.setLabel(namespace, objName, gvr)
		}

	}
	return nil
}

// // p.labelFromKustomizeRunOutput(listOfObjects, contextName, starNs, starLabel)
// func (p ParamsStruct) labelFromKustomizeRunOutput(listOfObjects []byte, contextName, starNs, starLabel string) {
// 	re := regexp.MustCompile(`([a-zA-Z0-9.-]+\/[a-zA-Z0-9.-]+) ([a-zA-Z0-9.-]+)`)
// 	matches := re.FindAllStringSubmatch(string(listOfObjects), -1)

// 	// iterate over matches and extract group version kind and object name
// 	for _, match := range matches {
// 		// the first capture group contains the group version kind and object name
// 		groupVersionKindObjectName := match[1]
// 		// split the string to get group version kind and object name
// 		parts := strings.Split(groupVersionKindObjectName, "/")
// 		gvkParts := strings.Split(parts[0], ".")
// 		kind := gvkParts[0]
// 		group := gvkParts[1:]
// 		objectName := parts[1]
// 		log.Printf("group: %s, kind: %s, ObjectName: %s", group, kind, objectName)
// 		if starLabel != "{{values.no-label}}" {
// 			labelCmd := []string{"kubectl", "--context=" + contextName, "-n", starNs, "label", kind + "/" + objectName, "app.kubernetes.io/part-of=" + starLabel}
// 			_, err := p.runCmd(labelCmd, true)
// 			if err != nil {
// 				log.Printf("label did not apply: %v", err)
// 			}
// 		}
// 	}
// }

func getGVRFromGVK(mapper *restmapper.DeferredDiscoveryRESTMapper, gvk schema.GroupVersionKind) (schema.GroupVersionResource, error) {
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

func (p ParamsStruct) setLabel(namespace, objectName string, gvr schema.GroupVersionResource) error {

	labelSlice := strings.Split(flags.label, "=")
	labelKey, labelVal := labelSlice[0], labelSlice[1]
	labels := map[string]string{
		labelKey: labelVal,
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

	if namespace == "" {
		_, err = p.labelerDynamicClient.Resource(gvr).Patch(context.TODO(), objectName, types.MergePatchType, patch, metav1.PatchOptions{})
	} else {

		_, err = p.labelerDynamicClient.Resource(gvr).Namespace(namespace).Patch(context.TODO(), objectName, types.MergePatchType, patch, metav1.PatchOptions{})
	}

	if err != nil {
		log.Printf("          🟡 failed to set labels on object %v/%v/%v %q in namespace %q: %v\n", gvr.Group, gvr.Version, gvr.Resource, objectName, namespace, err)
		return err
	}

	log.Printf("          🏷️ labeled object %v/%v/%v %q in namespace %q with %v=%v\n", gvr.Group, gvr.Version, gvr.Resource, objectName, namespace, labelKey, labelVal)
	return nil
}

func (p ParamsStruct) runCmd(cmdToRun string, cmdArgs []string) ([]byte, error) {
	log.Println(cmdArgs)
	cmd := exec.Command(cmdToRun, cmdArgs...)
	cmd.Env = append(cmd.Env, "PATH="+p.path)
	cmd.Env = append(cmd.Env, "HOME="+p.homeDir)

	var outputBuf bytes.Buffer
	cmd.Stdout = io.MultiWriter(&outputBuf)
	cmd.Stderr = io.MultiWriter(&outputBuf)

	err := cmd.Start()
	if err != nil {
		log.Println("   🔴 error starting command:", err)
		return nil, err
	}

	err = cmd.Wait()
	if err != nil {
		log.Println("   🔴 error waiting for command to complete:", err)
		log.Println(string(outputBuf.Bytes()))
		return nil, err
	}
	return outputBuf.Bytes(), nil
}

func (p ParamsStruct) getOriginalCommandFromHistory() (string, error) {
	// TODO: this may not always be zsh, could be bash - should check if bash_history or zsh_history has "labeler" in it - that would tell us we have the right history file

	//placeholder
	cmd := exec.Command("bash")

	switch os := runtime.GOOS; os {
	case "darwin":
		// if mac
		log.Println("mac")
		cmd = exec.Command("bash", "-c", "history -r ~/.zsh_history; history 1")
	case "linux":
		log.Println("linux")
		// if linux (tested on ubuntu)
		// remember to set:
		//     echo PROMPT_COMMAND="history -a; $PROMPT_COMMAND"  > ~/.bashrc
		//     source ~/.bashrc
		// test with:
		//     history -s "helm --kube-context=kind-kind install sealed-secrets sealed-secrets/sealed-secrets -n sealed-secrets --create-namespace" > exec | ./labeler app.kubernetes.io/part-of=sample-value
		cmd = exec.Command("bash", "-c", "history -r ~/.bash_history; history 3")
	default:
	}

	cmd.Env = append(cmd.Env, "PATH="+p.path)
	cmd.Env = append(cmd.Env, "HOME="+p.homeDir)

	var outputBuf bytes.Buffer
	cmd.Stdout = io.MultiWriter(&outputBuf)
	cmd.Stderr = io.MultiWriter(&outputBuf)

	err := cmd.Start()
	if err != nil {
		log.Println("   🔴 error starting command:", err)
		return "", err
	}

	err = cmd.Wait()
	if err != nil {
		log.Println("   🔴 error waiting for command to complete:", err)
		return "", err
	}

	originalCmd, err := extractCmdFromHistory(string(outputBuf.Bytes()))
	return originalCmd, err
}

func extractCmdFromHistory(historyText string) (string, error) {
	// Find the index of the first semicolon
	helmTextIndex := strings.Index(historyText, "helm")
	if helmTextIndex == -1 {
		log.Printf("helm not found: %v", historyText)
	}

	log.Printf("trimmedCommand: %v", historyText)

	// trim everything before the semicolon and trim any leading or trailing whitespace
	trimmedCommand := strings.TrimSpace(historyText[helmTextIndex:])

	// find the index of the first pipe character in the trimmed command
	pipeIndex := strings.Index(trimmedCommand, "|")
	if pipeIndex == -1 {
		return string(trimmedCommand), nil
		// return "", log.Errorf("pipe character not found")
	}

	// trim everything after the pipe character and trim any leading or trailing whitespace
	return strings.TrimSpace(trimmedCommand[:pipeIndex]), nil

}

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
		log.Printf("🔴 error loading kubeconfig: %q\n", err)
		os.Exit(1)
	}

	if flags.context != "" {
		// check if the specified context exists in the kubeconfig
		if _, exists := apiConfig.Contexts[flags.context]; !exists {
			log.Printf("Context %q does not exist in the kubeconfig\n", flags.context)
			os.Exit(1)
		}
		// switch the current context in the kubeconfig
		apiConfig.CurrentContext = flags.context
	}

	// create a new clientset with the updated config
	clientConfig := clientcmd.NewDefaultClientConfig(*apiConfig, &clientcmd.ConfigOverrides{})
	restConfig, err := clientConfig.ClientConfig()
	if err != nil {
		log.Printf("🔴 error creating clientset config: %v\n", err)
		os.Exit(1)
	}
	ocClientset, err := kubernetes.NewForConfig(restConfig)
	if err != nil {
		log.Printf("🔴 error creating clientset: %v\n", err)
		os.Exit(1)
	}
	ocDynamicClient, err := dynamic.NewForConfig(restConfig)
	if err != nil {
		log.Printf("🔴 error create dynamic client: %v\n", err)
		os.Exit(1)
	}

	return ocClientset, restConfig, ocDynamicClient
}

func (p ParamsStruct) createCachedDiscoveryClient(restConfigCoreOrWds rest.Config) (*restmapper.DeferredDiscoveryRESTMapper, error) {
	// create a cached discovery client for the provided config
	cachedDiscoveryClient, err := disk.NewCachedDiscoveryClientForConfig(&restConfigCoreOrWds, p.homeDir, ".cache", 60)
	if err != nil {
		log.Printf("could not get cacheddiscoveryclient: %v", err)
		// handle error
	}
	mapper := restmapper.NewDeferredDiscoveryRESTMapper(cachedDiscoveryClient)
	return mapper, nil
}

func (p ParamsStruct) useContext(contextName string) {
	setContext := []string{"config", "use-context", contextName}
	_, err := p.runCmd("kubectl", setContext)
	if err != nil {
		log.Printf("   🔴 error setting kubeconfig's current context: %v\n", err)
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

func main() {
	log.SetFlags(0) // remove the date and time stamp from log.print output
	var p ParamsStruct

	currentUser, err := user.Current()
	if err != nil {
		log.Println("Error (current user):", err)
		return
	}
	p.homeDir = currentUser.HomeDir
	p.path = os.Getenv("PATH")

	var rootCmd = &cobra.Command{
		Use:   "labeler",
		Short: "label all kubernetes resources with provided key/value pair",
		Long:  `Utility that automates the labeling of resources output from kubectl, kustomize, and helm`,
		RunE: func(cmd *cobra.Command, args []string) error {
			print = logNoop
			if flags.verbose {
				print = logOut
			}
			p.labelerClientSet, p.labelerRestConfig, p.labelerDynamicClient = p.switchContext()

			return p.detectInput()
		},
	}

	rootCmd.Flags().StringVarP(&flags.filepath, flagsName.file, flagsName.fileShort, "", "path to the file")
	rootCmd.PersistentFlags().StringVarP(&flags.label, flagsName.label, flagsName.labelShort, "", "label to apply to all resources e.g. -l app.kubernetes.io/part-of=sample-value")
	rootCmd.PersistentFlags().StringVarP(&flags.kubeconfig, flagsName.kubeconfig, flagsName.kubeconfigShort, "", "kubeconfig to use")
	rootCmd.PersistentFlags().StringVarP(&flags.context, flagsName.context, flagsName.contextShort, "", "context to use")
	rootCmd.PersistentFlags().BoolVarP(&flags.verbose, flagsName.verbose, flagsName.verboseShort, false, "log verbose output")
	rootCmd.PersistentFlags().BoolVarP(&flags.debug, flagsName.debug, flagsName.debugShort, false, "debug mode")

	err = rootCmd.Execute()
	if err != nil {
		log.Println(err)
		os.Exit(1)
	}
}
