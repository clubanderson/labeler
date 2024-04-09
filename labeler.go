package main

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"os/user"
	"regexp"
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

var version = "0.10.0"

type ParamsStruct struct {
	homeDir       string
	path          string
	originalCmd   string
	ClientSet     *kubernetes.Clientset
	RestConfig    *rest.Config
	DynamicClient *dynamic.DynamicClient
	flags         map[string]bool
	params        map[string]string
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
	p.flags = make(map[string]bool)
	p.params = make(map[string]string)

	p.flags[args[0]] = true
	for i, arg := range args {
		if strings.HasPrefix(arg, "--") || strings.HasPrefix(arg, "-") {
			if i < len(args)-1 && !strings.HasPrefix(args[i+1], "-") {
				if strings.Contains(arg, "=") {
					parts := strings.Split(arg, "=")
					p.params[parts[0][2:]] = parts[1]
				} else {
					p.params[arg[1:]] = args[i+1]
				}
			} else if strings.Contains(arg, "=") {
				parts := strings.Split(arg, "=")
				if len(parts) > 2 {
					p.params[parts[0][2:]] = parts[1] + "=" + parts[2]
				} else {
					p.params[parts[0][2:]] = parts[1]
				}
			} else {
				if strings.HasPrefix(arg, "--") {
					p.flags[arg[2:]] = true
				} else {
					p.flags[arg[1:]] = true
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
			p.flags[arg] = true
		}
	}

	// Print flags and params
	if p.flags["debug"] {
		log.Println("labeler.go: [debug] Flags:")
		for flag, value := range p.flags {
			log.Printf("labeler.go: [debug] %s: %t\n", flag, value)
		}

		log.Println("\nlabeler.go: [debug] Params:")
		for param, value := range p.params {
			log.Printf("labeler.go: [debug] %s: %s\n", param, value)
		}
		log.Println()
	}

	if args[0] == "k" || args[0] == "kubectl" || args[0] == "helm" {
		// if kubectl - remove debug from args

		// remove the following args for both helm and kubectl because they do not recognize them
		for i := 0; i < len(args); i++ {
			// log.Printf("args: %v", args[i])
			if strings.HasPrefix(args[i], "--bp-") {
				args = append(args[:i], args[i+1:]...)
				i--
			}
			if strings.HasPrefix(args[i], "--mw-") {
				args = append(args[:i], args[i+1:]...)
				i--
			}
			if strings.HasPrefix(args[i], "--remote-") {
				args = append(args[:i], args[i+1:]...)
				i--
			}
			if strings.HasPrefix(args[i], "--label") {
				p.params["labelKey"] = strings.Split(args[i], "=")[1]
				p.params["labelVal"] = strings.Split(args[i], "=")[2]
				args = append(args[:i], args[i+1:]...)
				i--
			}
			if strings.HasPrefix(args[i], "-l") {
				p.params["labelKey"] = strings.Split(args[i+1], "=")[0]
				p.params["labelVal"] = strings.Split(args[i+1], "=")[1]
				args = append(args[:i], args[i+2:]...)
				i--
				i--
			}
		}
		if p.flags["debug"] {
			log.Println("labeler.go: [debug] before args: ", args)
		}
		// Run the command with the parsed flags
		if args[0] == "k" || args[0] == "kubectl" {
			for i := 0; i < len(args); i++ {
				// log.Printf("args: %v", args[i])

				// remove these args specifically for kubectl because it does not recognize them
				if strings.HasPrefix(args[i], "--debug") {
					p.flags[args[i]] = true
					args = append(args[:i], args[i+1:]...)
					i--
				}
			}
			if p.flags["debug"] {
				log.Println("labeler.go: [debug] after args: ", args)
			}

			originalCommand := strings.Join(args, " ")
			p.originalCmd = originalCommand

			cmd := exec.Command(args[0], args[1:]...)
			out, err := cmd.CombinedOutput()
			fmt.Printf("%v", string(out))
			if err != nil {
				fmt.Printf("%v", string(out))
				os.Exit(1)
			}

			p.ClientSet, p.RestConfig, p.DynamicClient = p.switchContext()

			// Format the output
			output := strings.TrimSpace(string(out))
			lines := strings.Split(output, "\n")
			p.setLabelKubectl(lines)

			namespace := ""
			if p.params["namespace"] != "" {
				namespace = p.params["namespace"]
			} else if p.params["n"] != "" {
				namespace = p.params["n"]
			}
			if namespace != "" && namespace != "default" {
				err = p.setLabelNamespace()
				if err != nil {
					log.Println("labeler.go: error (set label namespace):", err)
					return err
				}
			}

		} else if args[0] == "helm" {
			// run the original helm command without the extra labeler flags
			output, err := p.runCmd("helm", args[1:])
			fmt.Printf("%v", string(output))
			if err != nil {
				log.Println(err)
				os.Exit(1)
			}
			// log.Printf("labeler.go: helm output: %v\n", string(output))

			// now run helm as template and label the output
			originalCommand := strings.Join(args, " ")
			p.originalCmd = originalCommand

			if p.flags["debug"] {
				log.Printf("labeler.go: [debug] original command: %v\n", originalCommand)
			}
			modifiedCommand := strings.Replace(originalCommand, " install ", " template ", 1)
			modifiedCommand = strings.Replace(modifiedCommand, " upgrade ", " template ", 1)
			modifiedCommandComponents := append(strings.Split(modifiedCommand, " ")[1:])
			if p.flags["debug"] {
				log.Printf("labeler.go: [debug] modified command components: %v\n", modifiedCommandComponents)
			}

			output, err = p.runCmd("helm", modifiedCommandComponents)
			if err != nil {
				// log.Println("labeler.go: error (run helm):", err)
				os.Exit(1)
			}

			p.ClientSet, p.RestConfig, p.DynamicClient = p.switchContext()

			err = p.traverseInputAndLabel(strings.NewReader(string(output)), os.Stdout)
			if err != nil {
				log.Println("labeler.go: error (to traverseInput):", err)
				return err
			}
			namespace := ""
			if p.params["namespace"] != "" {
				namespace = p.params["namespace"]
			} else if p.params["n"] != "" {
				namespace = p.params["n"]
			}
			if namespace != "" && namespace != "default" {
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
		if p.flags["bp-create"] {
			log.Println()
			p.createBP()
		}
		if p.flags["mw-create"] {
			log.Println()
			p.createMW()
		}
		if p.params["remote-contexts"] != "" {
			log.Println()
			p.remoteDeployTo()
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

func (p ParamsStruct) traverseInputAndLabel(r io.Reader, w io.Writer) error {
	mapper, _ := p.createCachedDiscoveryClient(*p.RestConfig)

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
			if p.flags["debug"] {
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
		log.Println("labeler.go: no label provided")
		return
	}
	if flags.label != "" {
		p.params["labelKey"], p.params["labelVal"] = strings.Split(flags.label, "=")[0], strings.Split(flags.label, "=")[1]
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
		labelCmd = []string{"-n", namespace, "label", kind + "/" + objectName, p.params["labelKey"] + "=" + p.params["labelVal"]}
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

		// log.Printf("labeler.go: labelCmd: %v\n", labelCmd)
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

func (p ParamsStruct) setLabel(namespace, objectName string, gvr schema.GroupVersionResource) error {
	if flags.label == "" && p.params["labelKey"] == "" {
		log.Println("labeler.go: no label provided")
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

	if namespace == "" {
		_, err = p.DynamicClient.Resource(gvr).Patch(context.TODO(), objectName, types.MergePatchType, patch, metav1.PatchOptions{})
		// if err != nil {
		// 	log.Printf("labeler.go: error patching object %v/%v/%v %q in namespace %q: %v\n", gvr.Group, gvr.Version, gvr.Resource, objectName, namespace, err)
		// }
	} else {
		_, err = p.DynamicClient.Resource(gvr).Namespace(namespace).Patch(context.TODO(), objectName, types.MergePatchType, patch, metav1.PatchOptions{})
		// if err != nil {
		// 	log.Printf("labeler.go: error patching object %v/%v/%v %q in namespace %q: %v\n", gvr.Group, gvr.Version, gvr.Resource, objectName, namespace, err)
		// }
	}

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
		if len(os.Args) <= 1 {
			log.Printf("no arguments given, need usage here (TODO)")
		} else {
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
		}
	} else {
		// requires labeler-piped.go - this 'else' can be removed if only using aliased commands
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
				p.ClientSet, p.RestConfig, p.DynamicClient = p.switchContext()

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
