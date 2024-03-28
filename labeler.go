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
	install    bool
	label      string
	kubeconfig string
	context    string
}

var flagsName = struct {
	file            string
	fileShort       string
	verbose         string
	verboseShort    string
	install         string
	installShort    string
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
	install:         "install",
	installShort:    "i",
	label:           "label",
	labelShort:      "l",
	kubeconfig:      "kubeconfig",
	kubeconfigShort: "k",
	context:         "context",
	contextShort:    "c",
}

func main() {
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

	// flag for the filepath
	rootCmd.Flags().StringVarP(
		&flags.filepath,
		flagsName.file,
		flagsName.fileShort,
		"", "path to the file")

	// flag for the filepath
	rootCmd.PersistentFlags().StringVarP(
		&flags.label,
		flagsName.label,
		flagsName.labelShort,
		"", "label to apply to all resources e.g. -l app.kubernetes.io/part-of=sample-value")

	// flag for the kubeconfig
	rootCmd.PersistentFlags().StringVarP(
		&flags.kubeconfig,
		flagsName.kubeconfig,
		flagsName.kubeconfigShort,
		"", "kubeconfig to use")

	// flag for the kubeconfig
	rootCmd.PersistentFlags().StringVarP(
		&flags.context,
		flagsName.context,
		flagsName.contextShort,
		"", "context to use")

	// flag for the verbosity level
	rootCmd.PersistentFlags().BoolVarP(
		&flags.verbose,
		flagsName.verbose,
		flagsName.verboseShort,
		false, "log verbose output")

	// flag for to indicate whether to use with 'helm install' with dry-run, or not.
	// Use without dry-run require shell history, use with dry-run does not.
	// Use with dry-run does not give us access to the vars used in 'helm install' however.
	rootCmd.PersistentFlags().BoolVarP(
		&flags.install,
		flagsName.install,
		flagsName.installShort,
		false, "use directly on 'helm install' instead of dry-run input")

	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

func (p ParamsStruct) detectInput(labelerRestConfig *rest.Config) error {
	if !flags.install {
		if isInputFromPipe() {
			// check to see if there was no input from the piped command - there is likely an error in the piped command - so detect no input and stop
			scanner := bufio.NewScanner(os.Stdin)
			if !scanner.Scan() {
				return nil
			}
			err := traverseInput(os.Stdin, os.Stdout)
			if err != nil {
				fmt.Println("Error (to uppercase):", err)
				return err
			}
		}
		log.Printf("labeling all resources with: %q", flags.label)
		return nil
	}
	if isInputFromPipe() {
		// if input is from a pipe, upper case the
		// content of stdin
		print("data is from pipe")
		return p.helmOrKubectl(os.Stdin, os.Stdout)
	} else {
		// ...otherwise get the file
		file, e := getFile()
		if e != nil {
			return e
		}
		defer file.Close()
		return p.helmOrKubectl(file, os.Stdout)
	}

	mapper, _ := p.createCachedDiscoveryClient(*labelerRestConfig)

	// gvk (group, version, kind) should come out of traverseInput. you will use yaml decoding to find the '.kind', '.apiVersion', and '.metadata.name' from all yaml records. HINT: apiVersion is the group/version (gv) in gvk, and kind is the kind (k) in gvk
	err := p.setLabel("output", "group", "version", "kind", "objectName", mapper)
	if err != nil {
		// HACKME!!!
	}

	return nil

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
		fmt.Println("Error (get history):", err)
		os.Exit(1)
	}

	fmt.Printf("Original command: %q\n", originalCommand)
	isHelm, isKubectl, isKustomize := false, false, false

	if strings.HasPrefix(originalCommand, "helm") {
		fmt.Printf("your running helm\n")
		isHelm = true
	} else if strings.HasPrefix(originalCommand, "kubectl") {
		fmt.Printf("your running kubectl\n")
	}

	if isHelm {
		modifiedCommand := strings.Replace(originalCommand, "install", "template", 1)
		modifiedCommandComponents := append(strings.Split(modifiedCommand, " ")[1:])
		output, err := p.runCmd("helm", modifiedCommandComponents)
		if err != nil {
			fmt.Println("Error (run helm):", err)
			os.Exit(1)
		}
		err = traverseInput(strings.NewReader(string(output)), os.Stdout)
		if err != nil {
			fmt.Println("Error (to traverseInput):", err)
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
			fmt.Println("Error (to uppercase):", err)
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
	fmt.Println(v...)
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

func (p ParamsStruct) runCmd(cmdToRun string, cmdArgs []string) ([]byte, error) {
	fmt.Println(cmdArgs)
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
		// return "", fmt.Errorf("pipe character not found")
	}

	// trim everything after the pipe character and trim any leading or trailing whitespace
	return strings.TrimSpace(trimmedCommand[:pipeIndex]), nil

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
	// don't label a placement object - it is not going to be deployed to a remote
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
