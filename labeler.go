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

	"gopkg.in/yaml.v2"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/discovery/cached/disk"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/restmapper"
	"k8s.io/client-go/tools/clientcmd"
)

type ParamsStruct struct {
	homeDir string
	path    string
}

var flags struct {
	filepath   string
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
	label:           "label",
	labelShort:      "l",
	kubeconfig:      "kubeconfig",
	kubeconfigShort: "k",
	context:         "context",
	contextShort:    "c",
}

// need a new function to do labeling

func (p ParamsStruct) detectInput(labelerRestConfig *rest.Config) error {
	if isInputFromPipe() {
		// if input is from a pipe, traverseinput the
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
			input = append(input, scanner.Bytes()...)
			input = append(input, '\n') // Append newline to separate lines
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
		var yamlData interface{}
		if err := yaml.Unmarshal(input, &yamlData); err != nil {
			log.Printf("warning: no YAML input was detected %v", err)
		}

		// Check if YAML was provided
		if yamlData != nil {
			log.Println("YAML data detected in stdin")
			// Do something with the YAML data received - don't need to use history hack in this case - we got valid YAML input from template, --dry-run, or --debug
			err := traverseInput(strings.NewReader(string(input)), os.Stdout)
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

	mapper, _ := p.createCachedDiscoveryClient(*labelerRestConfig)

	// gvk (group, version, kind) should come out of traverseInput. you will use yaml decoding to find the '.kind', '.apiVersion', and '.metadata.name' from all yaml records. HINT: apiVersion is the group/version (gv) in gvk, and kind is the kind (k) in gvk
	log.Printf("\n\nlet's label some objects!\n\n")
	err := p.setLabel("output", "group", "version", "kind", "objectName", mapper)
	if err != nil {
		// HACKME!!!
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
		os.Exit(1)
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
		err = traverseInput(strings.NewReader(string(output)), os.Stdout)
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
		traverseInput(r, w)
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

func traverseInput(r io.Reader, w io.Writer) error {
	scanner := bufio.NewScanner(bufio.NewReader(r))
	for scanner.Scan() {
		line := scanner.Text()
		// HACKME!!! - this string comparison is wrong - this should use yaml decoding to find the '.kind', '.apiVersion', and '.metadata.name' from all yaml records
		if strings.HasPrefix(line, "kind:") || strings.HasPrefix(line, "apiVersion:") || strings.HasPrefix(line, "  name:") {
			_, err := fmt.Fprintf(w, line+"\n")
			if err != nil {
				return err
			}
		}
	}
	return nil
}

func (p ParamsStruct) setLabel(output, group, version, kind, objectName string, mapper *restmapper.DeferredDiscoveryRESTMapper) error {
	// decode output...
	//   decode 'output'  to find each object definition (separated by '---' but do not do string split here - there is a better way!)

	// yamlBytes := marshall(obj)...

	// runtimeObj := decodeYaml(yamlBytes)...

	// gvk := runtimeObj.GroupVersionKind()

	// var gvr schema.GroupVersionResource
	// gvr, err = getGVRFromGVK(mapper, gvk)
	// if err != nil {
	// 	if p.debug {
	// 		log.Printf("          🟡 error getting gvr from gvk for %v/%v/%v. Retrying in 5 seconds: %v\n", gvk.Group, gvk.Version, gvk.Kind, err)
	// 	}
	// }

	// now that you have gvr, run 'kubectl label %group%/%version%/%resource% %objectName% app.kubernetes.io/part-of=%value%'
	return nil
}

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

func (p ParamsStruct) sampleSetLabel(ocDynamicClientCoreOrWds dynamic.Interface, namespace, objectName string, gvr schema.GroupVersionResource) error {
	// don't label a bindingpolicy object - it is not going to be deployed to a remote
	if gvr.Resource == "bindingpolicies" {
		return nil
	}
	// don't label openshift namespaces as part-of anything - we do not want to manage or delete these namespaces
	if strings.HasPrefix(objectName, "openshift-") && gvr.Resource == "namespaces" {
		log.Println("          ℹ️ not labeling this namespace - it is part of openshift operations\n          NOTE: if you want to synchronize this namespace object, you must select it explicitly in a bindingpolicy")
		return nil
	}
	if strings.HasPrefix(objectName, "kube-") && gvr.Resource == "namespaces" {
		log.Println("          ℹ️ not labeling this namespace - it is part of kubernetes operations\n          NOTE: if you want to synchronize this namespace object, you must select it explicitly in a bindingpolicy")
		return nil
	}
	if gvr.Resource == "customresourcedefinitions" && (objectName == "operatorgroups.operators.coreos.com" || objectName == "subscriptions.operators.coreos.com") {
		log.Printf("          ℹ️ not labeling %v - it is part of openshift operations\n", objectName)
		return nil
	}
	if gvr.Resource == "sealedsecrets" {
		log.Printf("          ℹ️ not labeling %v - it is a sealed secret\n", objectName)
		return nil
	}

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
		_, err = ocDynamicClientCoreOrWds.Resource(gvr).Patch(context.TODO(), objectName, types.MergePatchType, patch, metav1.PatchOptions{})
	} else {
		_, err = ocDynamicClientCoreOrWds.Resource(gvr).Namespace(namespace).Patch(context.TODO(), objectName, types.MergePatchType, patch, metav1.PatchOptions{})
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
		return "", fmt.Errorf("helm not found: %v", historyText)
	}

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

func (p ParamsStruct) switchContext(contextName string) (*kubernetes.Clientset, *rest.Config, dynamic.Interface) {
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
		if _, exists := apiConfig.Contexts[contextName]; !exists {
			log.Printf("Context %q does not exist in the kubeconfig\n", contextName)
			os.Exit(1)
		}
		// switch the current context in the kubeconfig
		apiConfig.CurrentContext = contextName
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
			labelerClientSet, labelerRestConfig, labelerDynamicClient := p.switchContext(flags.context)
			_, _, _ = labelerClientSet, labelerRestConfig, labelerDynamicClient

			return p.detectInput(labelerRestConfig)
		},
	}

	rootCmd.Flags().StringVarP(&flags.filepath, flagsName.file, flagsName.fileShort, "", "path to the file")
	rootCmd.PersistentFlags().StringVarP(&flags.label, flagsName.label, flagsName.labelShort, "", "label to apply to all resources e.g. -l app.kubernetes.io/part-of=sample-value")
	rootCmd.PersistentFlags().StringVarP(&flags.kubeconfig, flagsName.kubeconfig, flagsName.kubeconfigShort, "", "kubeconfig to use")
	rootCmd.PersistentFlags().StringVarP(&flags.context, flagsName.context, flagsName.contextShort, "", "context to use")
	rootCmd.PersistentFlags().BoolVarP(&flags.verbose, flagsName.verbose, flagsName.verboseShort, false, "log verbose output")

	err = rootCmd.Execute()
	if err != nil {
		log.Println(err)
		os.Exit(1)
	}
}
