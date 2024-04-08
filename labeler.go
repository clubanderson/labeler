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

var version = "0.2.0"

type ParamsStruct struct {
	homeDir              string
	path                 string
	debug                bool
	labelerClientSet     *kubernetes.Clientset
	labelerRestConfig    *rest.Config
	labelerDynamicClient *dynamic.DynamicClient
	labelKey             string
	labelVal             string
	namespace            string
	kubeconfig           string
	context              string
	overwrite            bool
	createnamespace      bool
	dryrunMode           bool
	debugMode            bool
	upgradeMode          bool
	templateMode         bool
	installMode          bool
	createBindingPolicy  bool
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
	overwrite  bool
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
	overwrite       string
	overwriteShort  string
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
	overwrite:       "overwrite",
	overwriteShort:  "o",
}

func (p ParamsStruct) aliasRun(args []string) error {
	args = os.Args[1:]

	p.overwrite = false
	p.createnamespace = false
	p.debugMode = false
	p.dryrunMode = false
	p.templateMode = false
	p.upgradeMode = false
	p.namespace = ""
	p.createBindingPolicy = false
	if args[0] == "k" || args[0] == "kubectl" || args[0] == "helm" {
		for i := 0; i < len(args); i++ {
			// log.Printf("labeler.go: args: %v\n", args[i])
			if strings.Contains(args[i], "--context=") {
				p.context = strings.Split(args[i], "=")[1]
			} else if args[i] == "-n" && i < len(args)-1 {
				p.namespace = args[i+1]
			} else if strings.Contains(args[i], "--namespace=") {
				p.namespace = strings.Split(args[i], "=")[1]
			} else if strings.Contains(args[i], "--kubeconfig=") {
				p.kubeconfig = strings.Split(args[i], "=")[1]
			} else if args[i] == "--overwrite" {
				p.overwrite = true
			} else if args[i] == "--create-namespace" {
				p.createnamespace = true
			} else if args[i] == "--debug" {
				p.debugMode = true
			} else if args[i] == "--dry-run" {
				p.dryrunMode = true
			} else if args[i] == "template" {
				p.templateMode = true
			} else if args[i] == "install" {
				p.installMode = true
			} else if args[i] == "upgrade" {
				p.upgradeMode = true
			}
		}
		// log.Println("labeler.go: before args: ", args)

		for i := 0; i < len(args); i++ {
			if args[i] == "-l" && i < len(args)-1 {
				p.labelKey = strings.Split(args[i+1], "=")[0]
				p.labelVal = strings.Split(args[i+1], "=")[1]
				args = append(args[:i], args[i+2:]...)
			} else if strings.Contains(args[i], "--label=") {
				p.labelKey = strings.Split(args[i], "=")[1]
				p.labelVal = strings.Split(args[i], "=")[2]
				args = append(args[:i], args[i+1:]...)
			} else if args[i] == "--create-bp" {
				p.createBindingPolicy = true
				args = append(args[:i], args[i+1:]...)
			}
		}

		// log.Println("labeler.go: after args: ", args)
		// log.Println("labeler.go: params: ", p.debugMode, p.dryrunMode, p.templateMode, p.namespace, p.context, p.kubeconfig, p.labelKey, p.labelVal, p.overwrite, p.createnamespace)

		// Run the command with the parsed flags
		if args[0] == "k" || args[0] == "kubectl" {
			cmd := exec.Command(args[0], args[1:]...)
			out, err := cmd.CombinedOutput()
			if err != nil {
				fmt.Printf("labeler.go: error: %v, %v", err, string(out))
				os.Exit(1)
			}

			p.labelerClientSet, p.labelerRestConfig, p.labelerDynamicClient = p.switchContext()

			// Format the output
			output := strings.TrimSpace(string(out))
			lines := strings.Split(output, "\n")
			p.setLabelKubectl(lines)

			if p.namespace != "" && p.namespace != "default" {
				err = p.setLabelNamespace()
				if err != nil {
					log.Println("labeler.go: error (set label namespace):", err)
					return err
				}
			}

		} else if args[0] == "helm" {
			// run the original helm command without the extra labeler flags
			output, err := p.runCmd("helm", args[1:])
			if err != nil {
				log.Println("labeler.go: error (run helm):", err)
				os.Exit(1)
			}
			// log.Printf("labeler.go: helm output: %v\n", string(output))

			// now run helm as template and label the output
			originalCommand := strings.Join(args, " ")
			modifiedCommand := strings.Replace(originalCommand, " install ", " template ", 1)
			modifiedCommand = strings.Replace(originalCommand, " upgrade ", " template ", 1)
			modifiedCommandComponents := append(strings.Split(modifiedCommand, " ")[1:])
			output, err = p.runCmd("helm", modifiedCommandComponents)
			if err != nil {
				log.Println("labeler.go: error (run helm):", err)
				os.Exit(1)
			}

			p.labelerClientSet, p.labelerRestConfig, p.labelerDynamicClient = p.switchContext()

			err = p.traverseInputAndLabel(strings.NewReader(string(output)), os.Stdout)
			if err != nil {
				log.Println("labeler.go: error (to traverseInput):", err)
				return err
			}
			if p.namespace != "" && p.namespace != "default" {
				err = p.setLabelNamespace()
				if err != nil {
					log.Println("labeler.go: Error (set label namespace):", err)
					return err
				}
			}

		}

		if len(runResults.didNotLabel) > 0 {
			log.Printf("\nlabeler.go: The following resources do not exist and can be labeled at a later time:\n\n")
			for _, cmd := range runResults.didNotLabel {
				log.Printf(cmd)
			}
		}
		if p.createBindingPolicy {
			log.Println()
			p.createBP()
		}

	}
	return nil
}

func (p ParamsStruct) setLabelNamespace() error {
	gvr := schema.GroupVersionResource{
		Group:    "",
		Version:  "v1",
		Resource: "namespaces",
	}
	labels := map[string]string{
		p.labelKey: p.labelVal,
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
	if p.installMode && !p.dryrunMode {
		// log.Printf("labeler.go: patching namespace %q with %v=%v %q %q %q %v\n", p.namespace, p.labelKey, p.labelVal, gvr.Resource, gvr.Version, gvr.Group, string(patch))
		_, err = p.labelerDynamicClient.Resource(gvr).Patch(context.TODO(), p.namespace, types.MergePatchType, patch, metav1.PatchOptions{})
	}
	if err != nil {
		if p.installMode && !p.dryrunMode {
			labelCmd := fmt.Sprintf("kubectl label %v %v %v=%v\n", gvr.Resource, p.namespace, p.labelKey, p.labelVal)
			runResults.didNotLabel = append(runResults.didNotLabel, labelCmd)
		}
	} else {
		log.Printf("  🏷️ labeled object %v/%v/%v %q with %v=%v\n", gvr.Group, gvr.Version, gvr.Resource, p.namespace, p.labelKey, p.labelVal)
	}
	return nil
}

func traverseLine(line, namespace, context, kubeconfig string) {
	// Implement your traversal logic here
	// Example: Apply label if the line contains "pattern"
	pattern := "pattern" // Change this to your desired regex pattern
	matched, err := regexp.MatchString(pattern, line)
	if err != nil {
		fmt.Println("labeler.go: error:", err)
		return
	}
	if matched {
		// Prepare kubectl command with namespace, context, and kubeconfig if provided
		cmdArgs := []string{"label", "apply", line, "some-label=value"}
		if namespace != "" {
			cmdArgs = append(cmdArgs, "--namespace="+namespace)
		}
		if context != "" {
			cmdArgs = append(cmdArgs, "--context="+context)
		}
		if kubeconfig != "" {
			cmdArgs = append(cmdArgs, "--kubeconfig="+kubeconfig)
		}

		// Run kubectl label command
		cmd := exec.Command("kubectl", cmdArgs...)
		if err := cmd.Run(); err != nil {
			fmt.Println("labeler.go: error:", err)
			return
		}
	}
}

func (p ParamsStruct) detectInput() error {
	var yamlData interface{}
	var buffer []string
	runResults.didNotLabel = []string{}

	if isInputFromPipe() {
		// if input is from a pipe, traverseinput and label the content of stdin
		// log.Println("labeler.go: data is from pipe")
		// // Read the input
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
			log.Printf("labeler.go: error reading input: %v", err)
			return nil
		}

		// Try parsing the input as YAML
		if err := yaml.Unmarshal(input, &yamlData); err != nil {
			// log.Printf("labeler.go: warning: no YAML input was detected %v", err)
		}

		// Check if YAML was provided
		if yamlData != nil {
			// log.Println("labeler.go: YAML data detected in stdin")
			// Do something with the YAML data received - don't need to use history hack in this case - we got valid YAML input from template, --dry-run, or --debug
			err := p.traverseInputAndLabel(strings.NewReader(string(input)), os.Stdout)
			if err != nil {
				log.Println("labeler.go: error (traverseinput):", err)
				return err
			}
		} else {
			// log.Println("labeler.go: no YAML data detected in stdin, will try to run again with YAML output")
			// time to do it the hard way - many may not like this approach (history hack) - the other options above are more than sufficient for most people's use
			return p.helmOrKubectl(os.Stdin, os.Stdout, buffer)
		}
	} else {
		// ...otherwise get the file
		log.Println("labeler.go: data is from file")
		file, e := getFile()
		if e != nil {
			return e
		}
		defer file.Close()
		return p.helmOrKubectl(file, os.Stdout, buffer)
	}

	if len(runResults.didNotLabel) > 0 {
		log.Printf("labeler.go: The following resources do not exist and can be labeled at a later time:\n\n")
		for _, cmd := range runResults.didNotLabel {
			log.Printf(cmd)
		}
	}
	return nil
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

func (p ParamsStruct) helmOrKubectl(r io.Reader, w io.Writer, input []string) error {
	originalCommand, cmdFound, err := p.getOriginalCommandFromHistory()
	if err != nil {
		log.Println("labeler.go: error (get history):", err)
		// os.Exit(1)
	}

	// log.Printf("labeler.go: original command: %q\n\n", originalCommand)

	if cmdFound == "helm" {
		modifiedCommand := strings.Replace(originalCommand, "install", "template", 1)
		modifiedCommandComponents := append(strings.Split(modifiedCommand, " ")[1:])
		// log.Printf("labeler.go: modified command: %q\n", modifiedCommand)
		// log.Printf("labeler.go: modified command components: %q\n", modifiedCommandComponents)
		output, err := p.runCmd("helm", modifiedCommandComponents)
		if err != nil {
			// log.Println("labeler.go: error (running helm):", err)
			os.Exit(1)
		}

		err = p.traverseInputAndLabel(strings.NewReader(string(output)), os.Stdout)
		if err != nil {
			log.Println("labeler.go: error (to traverseInput):", err)
			return err
		}
	} else if cmdFound == "kubectl" || cmdFound == "kustomize" {
		p.setLabelKubectl(input)
	}
	return nil
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
				log.Printf("labeler.go: decoding error: %v\n%v\n", err, obj)
			}
			break // Reached end of file or error
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
			if p.debug {
				log.Printf("labeler.go: error getting gvr from gvk for %v/%v/%v. Retrying in 5 seconds: %v\n", gvk.Group, gvk.Version, gvk.Kind, err)
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

func (p ParamsStruct) setLabelKubectl(input []string) {
	allLines := strings.Join(input, "\n")

	re := regexp.MustCompile(`([a-zA-Z0-9.-]+\/[a-zA-Z0-9.-]+) ([a-zA-Z0-9.-]+)`)
	matches := re.FindAllStringSubmatch(allLines, -1)

	namespace := p.namespace
	if namespace == "" {
		namespace = "default" // this needs to be the value given to kubectl - if empty, then it is default
	}

	if flags.label == "" && p.labelKey == "" {
		log.Println("labeler.go: no label provided")
		return
	}
	if flags.label != "" {
		p.labelKey, p.labelVal = strings.Split(flags.label, "=")[0], strings.Split(flags.label, "=")[1]
	}

	if len(matches) == 0 {
		log.Println("labeler.go: no resources found to label")
		return
	}

	// iterate over matches and extract group version kind and object name
	for _, match := range matches {
		var labelCmd []string
		// log.Printf("labeler.go: match: %v\n", match)
		// the first match group contains the group version kind and object name
		groupVersionKindObjectName := match[1]
		// split the string to get group version kind and object name
		parts := strings.Split(groupVersionKindObjectName, "/")
		gvkParts := strings.Split(parts[0], ".")
		kind := gvkParts[0]
		// group := gvkParts[1:]
		objectName := parts[1]
		// log.Printf("labeler.go: group: %s, kind: %s, ObjectName: %s", group, kind, objectName)
		labelCmd = []string{"-n", namespace, "label", kind + "/" + objectName, p.labelKey + "=" + p.labelVal}
		if flags.context != "" {
			labelCmd = append(labelCmd, "--context="+flags.context)
			// labelCmd = []string{"--context=" + flags.context, "-n", namespace, "label", kind + "/" + objectName, p.labelKey + "=" + p.labelVal, "--overwrite"}
		}
		if p.overwrite || flags.overwrite {
			labelCmd = append(labelCmd, "--overwrite")
		}
		if p.context != "" {
			labelCmd = append(labelCmd, "--context="+p.context)
		}
		if p.kubeconfig != "" {
			labelCmd = append(labelCmd, "--kubeconfig="+p.kubeconfig)
		}

		// log.Printf("labeler.go: labelCmd: %v\n", labelCmd)
		output, err := p.runCmd("kubectl", labelCmd)
		if err != nil {
			// log.Printf("labeler.go: label did not apply due to error: %v", err)
		} else {
			if strings.Contains(string(output), "not labeled") {
				log.Printf("  %v already has label %v=%v", strings.Split(string(output), " ")[0], p.labelKey, p.labelVal)
			} else {
				log.Printf("  🏷️ created and labeled object %q in namespace %q with %v=%v\n", objectName, namespace, p.labelKey, p.labelVal)
			}
		}
	}
}

func (p ParamsStruct) setLabel(namespace, objectName string, gvr schema.GroupVersionResource) error {
	if flags.label == "" && p.labelKey == "" {
		log.Println("labeler.go: no label provided")
		return nil
	}
	if flags.label != "" {
		p.labelKey, p.labelVal = strings.Split(flags.label, "=")[0], strings.Split(flags.label, "=")[1]
	}

	labels := map[string]string{
		p.labelKey: p.labelVal,
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
		// if err != nil {
		// 	log.Printf("labeler.go: error patching object %v/%v/%v %q in namespace %q: %v\n", gvr.Group, gvr.Version, gvr.Resource, objectName, namespace, err)
		// }
	} else {
		_, err = p.labelerDynamicClient.Resource(gvr).Namespace(namespace).Patch(context.TODO(), objectName, types.MergePatchType, patch, metav1.PatchOptions{})
		// if err != nil {
		// 	log.Printf("labeler.go: error patching object %v/%v/%v %q in namespace %q: %v\n", gvr.Group, gvr.Version, gvr.Resource, objectName, namespace, err)
		// }
	}

	if err != nil {
		if namespace != "" {
			labelCmd := fmt.Sprintf("kubectl label %v %v %v=%v -n %v\n", gvr.Resource, objectName, p.labelKey, p.labelVal, namespace)
			runResults.didNotLabel = append(runResults.didNotLabel, labelCmd)
		} else {
			labelCmd := fmt.Sprintf("kubectl label %v %v %v=%v\n", gvr.Resource, objectName, p.labelKey, p.labelVal)
			runResults.didNotLabel = append(runResults.didNotLabel, labelCmd)
		}
		return err
	}

	log.Printf("  🏷️ labeled object %v/%v/%v %q in namespace %q with %v=%v\n", gvr.Group, gvr.Version, gvr.Resource, objectName, namespace, p.labelKey, p.labelVal)
	return nil
}

func (p ParamsStruct) getOriginalCommandFromHistory() (string, string, error) {
	// TODO: this may not always be zsh, could be bash - should check if bash_history or zsh_history has "labeler" in it - that would tell us we have the right history file
	cmd := exec.Command("bash")

	switch os := runtime.GOOS; os {
	case "darwin":
		// if mac
		// log.Println("mac")
		cmd = exec.Command("bash", "-c", "history -a; history -r ~/.zsh_history; history 1")
	case "linux":
		// log.Println("linux")
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
		// log.Println("labeler.go: error starting command:", err)
		return "", "", err
	}

	err = cmd.Wait()
	if err != nil {
		// log.Println("labeler.go: error waiting for command to complete:", err)
		return "", "", err
	}

	originalCmd, cmdFound, err := extractCmdFromHistory(string(outputBuf.Bytes()))
	// log.Printf("labeler.go: command found: %q\n", cmdFound)
	return originalCmd, cmdFound, err
}

func extractCmdFromHistory(historyText string) (string, string, error) {
	// Find the index of the first semicolon
	cmdFound := ""
	trimmedCommand := strings.TrimSpace(historyText)

	// find the index of the first pipe character in the trimmed command
	pipeIdx := strings.Index(trimmedCommand, "|")
	if pipeIdx == -1 {
		// return string(trimmedCommand), "", nil
		// return "", log.Errorf("labeler.go: pipe character not found")
	} else {
		trimmedCommand = trimmedCommand[:pipeIdx]
	}

	helmTextIndex := strings.Index(historyText, "helm")
	if helmTextIndex == -1 {
		// log.Printf("labeler.go: helm not found: %v", historyText)
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
		log.Println("labeler.go: error (current user):", err)
		return
	}
	p.homeDir = currentUser.HomeDir
	p.path = os.Getenv("PATH")
	if !isInputFromPipe() {
		args := os.Args[1:]
		if args[0] == "--version" || args[0] == "-v" {
			log.Printf("labeler version %v\n", version)
		}
		if len(args) > 0 {
			if args[0] == "k" || args[0] == "h" || args[0] == "kubectl" || args[0] == "helm" {
				// log.Println("labeler.go: invoked as alias: ")
				p.aliasRun(args)
			}
		}
	} else {
		var versionFlag bool

		var rootCmd = &cobra.Command{
			SilenceErrors: true,
			SilenceUsage:  true,
			Use:           "labeler",
			Short:         "label all kubernetes resources with provided key/value pair",
			Long:          `Utility that automates the labeling of resources output from kubectl, kustomize, and helm`,
			Run: func(cmd *cobra.Command, args []string) {
				if versionFlag {
					log.Printf("labeler version %v\n", version)
					return
				}
				if flags.label == "" {
					log.Println("labeler.go: no label provided")
					os.Exit(1)
				}

				print = logNoop
				if flags.verbose {
					print = logOut
				}
				p.labelerClientSet, p.labelerRestConfig, p.labelerDynamicClient = p.switchContext()

				p.detectInput()
			},
		}

		rootCmd.SetFlagErrorFunc(func(cmd *cobra.Command, err error) error {
			cmd.Println(err)
			cmd.Println(cmd.UsageString())
			return SilentErr(err)
		})
		rootCmd.Flags().BoolVar(&versionFlag, "version", false, "print the version")
		// rootCmd.Flags().StringVarP(&flags.filepath, flagsName.file, flagsName.fileShort, "", "path to the file")
		rootCmd.PersistentFlags().StringVarP(&flags.label, flagsName.label, flagsName.labelShort, "", "label to apply to all resources e.g. -l app.kubernetes.io/part-of=sample-value")
		rootCmd.PersistentFlags().StringVarP(&flags.kubeconfig, flagsName.kubeconfig, flagsName.kubeconfigShort, "", "kubeconfig to use")
		rootCmd.PersistentFlags().StringVarP(&flags.context, flagsName.context, flagsName.contextShort, "", "context to use")
		rootCmd.PersistentFlags().BoolVarP(&flags.verbose, flagsName.verbose, flagsName.verboseShort, false, "log verbose output")
		rootCmd.PersistentFlags().BoolVarP(&flags.debug, flagsName.debug, flagsName.debugShort, false, "debug mode")
		rootCmd.PersistentFlags().BoolVarP(&flags.overwrite, flagsName.overwrite, flagsName.overwriteShort, false, "overwrite mode")

		err = rootCmd.Execute()
		if err != nil {
			log.Println(err)
			os.Exit(1)
		}
	}
}

func SilentErr(error) error {
	return nil
}
