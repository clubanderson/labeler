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
	"regexp"
	"runtime"
	"strings"

	"gopkg.in/yaml.v3"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"

	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

type ParamsStruct struct {
	homeDir              string
	path                 string
	debug                bool
	labelerClientSet     *kubernetes.Clientset
	labelerRestConfig    *rest.Config
	labelerDynamicClient *dynamic.DynamicClient
}

type resultsStruct struct {
	didNotLabel []string
}

var runResults resultsStruct

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
	var buffer []string
	runResults.didNotLabel = []string{}

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
			buffer = append(buffer, line)
			if !isYAML(line) {
				continue // Skip non-YAML lines
			}
			break
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
			return p.helmOrKubectl(os.Stdin, os.Stdout, buffer)
		}
	} else {
		// ...otherwise get the file
		log.Println("data is from file")
		file, e := getFile()
		if e != nil {
			return e
		}
		defer file.Close()
		return p.helmOrKubectl(file, os.Stdout, buffer)
	}

	if len(runResults.didNotLabel) > 0 {
		log.Printf("\nThe following resources do not exist and can be labeled at a later time:\n\n")
		for _, cmd := range runResults.didNotLabel {
			log.Printf(cmd)
		}
	}
	log.Println()

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

func (p ParamsStruct) helmOrKubectl(r io.Reader, w io.Writer, input []string) error {
	originalCommand, cmdFound, err := p.getOriginalCommandFromHistory()
	if err != nil {
		log.Println("Error (get history):", err)
		// os.Exit(1)
	}

	log.Printf("original command: %q\n\n", originalCommand)

	if cmdFound == "helm" {
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
	} else if cmdFound == "kubectl" {
		// this is plain kubectl
		p.labelFromKubectlRunOutput(input)
	} else if cmdFound == "kustomize" {
		// this is kustomize
		p.labelFromKubectlRunOutput(input)
	}
	return nil
}

func (p ParamsStruct) functionToGetYAMLfromKustomizeFiles(originalCmd string) error {
	// the kind and object name is output from kustomize... how do we get the resource name... mapper to the rescue
	// this might work without mapping... lets try...

	mapper, _ := p.createCachedDiscoveryClient(*p.labelerRestConfig)
	_ = mapper
	gvk := schema.GroupVersionKind{
		Group:   "",
		Version: "",
		Kind:    "",
	}
	_ = gvk
	// resourceList, _ := mapper.ServerPreferredResources()
	// _ = resourceList

	// for _, resource := range resourceList {
	// 	// log.Println(resource)
	// 	for _, apiResource := range resource.APIResources {
	// 		// log.Printf("Group: %s, Version: %s, Kind: %s, Resource: %s\n", apiResource.Group, apiResource.Version, apiResource.Kind, apiResource.Name)
	// 		if apiResource.Name == rtype {
	// 			groupVersion := strings.Split(resource.GroupVersion, "/")
	// 			if len(groupVersion) == 2 {
	// 				gvr = schema.GroupVersionResource{
	// 					Group:    groupVersion[0],
	// 					Version:  groupVersion[1],
	// 					Resource: apiResource.Name,
	// 				}
	// 			} else {
	// 				gvr = schema.GroupVersionResource{
	// 					Group:    "",
	// 					Version:  groupVersion[0],
	// 					Resource: apiResource.Name,
	// 				}
	// 			}
	// 			return gvr, apiResource.Namespaced, nil
	// 		}
	// 	}
	// }
	return nil
	// return gvr, false, fmt.Errorf("resource kind not found for resource type %v", rtype)
}

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
				log.Printf("   🔴 decoding error: %v\n%v\n", err, obj)
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
			// log.Printf("   🔴 error decoding yaml: %v\n", err)
			continue
		}
		gvk := runtimeObj.GroupVersionKind()
		// log.Printf("G: %v, V: %v, K: %v, Name: %v", gvk.Group, gvk.Version, gvk.Kind, runtimeObj.GetName())

		gvr, err := p.getGVRFromGVK(mapper, gvk)
		if err != nil {
			if p.debug {
				log.Printf("   🟡 error getting gvr from gvk for %v/%v/%v. Retrying in 5 seconds: %v\n", gvk.Group, gvk.Version, gvk.Kind, err)
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

func (p ParamsStruct) labelFromKubectlRunOutput(input []string) {
	allLines := strings.Join(input, "\n")

	re := regexp.MustCompile(`([a-zA-Z0-9.-]+\/[a-zA-Z0-9.-]+) ([a-zA-Z0-9.-]+)`)
	matches := re.FindAllStringSubmatch(allLines, -1)

	namespace := "default" // this needs to be the value given to kubectl - if empty, then it is default

	if flags.label == "" {
		log.Println("No label provided")
		return
	}
	labelSlice := strings.Split(flags.label, "=")
	labelKey, labelVal := labelSlice[0], labelSlice[1]

	if len(matches) == 0 {
		log.Println("No resources found to label")
		return
	}

	// iterate over matches and extract group version kind and object name
	for _, match := range matches {
		var labelCmd []string
		// log.Printf("match: %v\n", match)
		// the first match group contains the group version kind and object name
		groupVersionKindObjectName := match[1]
		// split the string to get group version kind and object name
		parts := strings.Split(groupVersionKindObjectName, "/")
		gvkParts := strings.Split(parts[0], ".")
		kind := gvkParts[0]
		// group := gvkParts[1:]
		objectName := parts[1]
		// log.Printf("group: %s, kind: %s, ObjectName: %s", group, kind, objectName)
		if flags.context != "" {
			labelCmd = []string{"--context=" + flags.context, "-n", namespace, "label", kind + "/" + objectName, labelKey + "=" + labelVal, "--overwrite"}
		} else {
			labelCmd = []string{"-n", namespace, "label", kind + "/" + objectName, labelKey + "=" + labelVal, "--overwrite"}
		}
		output, err := p.runCmd("kubectl", labelCmd)
		if err != nil {
			log.Printf("label did not apply due to error: %v", err)
		} else {
			if strings.Contains(string(output), "not labeled") {
				log.Printf("  " + strings.Split(string(output), " ")[0] + " already has label")
			}
		}
	}
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
		if namespace != "" {
			labelCmd := fmt.Sprintf("kubectl label %v %v %v=%v -n %v\n", gvr.Resource, objectName, labelKey, labelVal, namespace)
			runResults.didNotLabel = append(runResults.didNotLabel, labelCmd)
		} else {
			labelCmd := fmt.Sprintf("kubectl label %v %v %v=%v\n", gvr.Resource, objectName, labelKey, labelVal)
			runResults.didNotLabel = append(runResults.didNotLabel, labelCmd)
		}
		return err
	}
	log.Printf("          🏷️ labeled object %v/%v/%v %q in namespace %q with %v=%v\n", gvr.Group, gvr.Version, gvr.Resource, objectName, namespace, labelKey, labelVal)
	return nil
}

func (p ParamsStruct) getOriginalCommandFromHistory() (string, string, error) {
	// TODO: this may not always be zsh, could be bash - should check if bash_history or zsh_history has "labeler" in it - that would tell us we have the right history file
	cmd := exec.Command("bash")

	switch os := runtime.GOOS; os {
	case "darwin":
		// if mac
		log.Println("mac")
		cmd = exec.Command("bash", "-c", "history -a; history -r ~/.zsh_history; history 1")
	case "linux":
		log.Println("linux")
		// if linux (tested on ubuntu)
		// remember to set:
		//     echo PROMPT_COMMAND="history -a; $PROMPT_COMMAND"  > ~/.bashrc
		//     source ~/.bashrc
		// test with:
		//     history -s "helm --kube-context=kind-kind install sealed-secrets sealed-secrets/sealed-secrets -n sealed-secrets --create-namespace" > exec | ./labeler app.kubernetes.io/part-of=sample-value
		cmd = exec.Command("bash", "-c", "history -a; history -r ~/.bash_history; history 3")
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
		return "", "", err
	}

	err = cmd.Wait()
	if err != nil {
		log.Println("   🔴 error waiting for command to complete:", err)
		return "", "", err
	}

	originalCmd, cmdFound, err := extractCmdFromHistory(string(outputBuf.Bytes()))
	log.Printf("command found: %q\n", cmdFound)
	return originalCmd, cmdFound, err
}

func extractCmdFromHistory(historyText string) (string, string, error) {
	// Find the index of the first semicolon
	cmdFound := "helm"
	trimmedCommand := strings.TrimSpace(historyText)

	// find the index of the first pipe character in the trimmed command
	pipeIdx := strings.Index(trimmedCommand, "|")
	if pipeIdx == -1 {
		// return string(trimmedCommand), "", nil
		// return "", log.Errorf("pipe character not found")
	} else {
		trimmedCommand = trimmedCommand[:pipeIdx]
	}

	helmTextIndex := strings.Index(historyText, "helm")
	if helmTextIndex == -1 {
		// log.Printf("helm not found: %v", historyText)
	} else {
		cmdFound = "helm"
		trimmedCommand = trimmedCommand[helmTextIndex:]
		return strings.TrimSpace(trimmedCommand), cmdFound, nil
	}

	// find the index of the first 'k' character in the trimmed command
	kubectlIdx := strings.Index(trimmedCommand, "k")
	if kubectlIdx == -1 {
		return string(trimmedCommand), cmdFound, nil
	} else {
		trimmedCommand = trimmedCommand[kubectlIdx:]
		cmdFound = "kubectl"
	}

	// find the index of the first 'k' character in the trimmed command
	kustomizeIdx := strings.Index(trimmedCommand, " -k ")
	if kustomizeIdx == -1 {
		return string(trimmedCommand), cmdFound, nil
	} else {
		cmdFound = "kustomize"
	}

	// trim everything after the pipe character and trim any leading or trailing whitespace
	return strings.TrimSpace(trimmedCommand), cmdFound, nil

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
			if flags.label == "" {
				log.Println("No label provided")
				os.Exit(1)
			}

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
