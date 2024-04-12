package main

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"os/user"
	"reflect"
	"strings"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

var version = "0.15.0"

type ResourceStruct struct {
	Group      string
	Version    string
	Resource   string
	Namespace  string
	ObjectName string
}

type ParamsStruct struct {
	homeDir       string
	path          string
	originalCmd   string
	ClientSet     *kubernetes.Clientset
	RestConfig    *rest.Config
	DynamicClient *dynamic.DynamicClient
	flags         map[string]bool
	params        map[string]string
	resources     map[ResourceStruct][]byte
	pluginArgs    map[string][]string
	pluginPtrs    map[string]reflect.Value
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

type Namespace struct {
	APIVersion string   `yaml:"apiVersion"`
	Kind       string   `yaml:"kind"`
	Metadata   Metadata `yaml:"metadata"`
}

func (p ParamsStruct) aliasRun(args []string) error {
	args = os.Args[1:]
	p.flags = make(map[string]bool)
	p.params = make(map[string]string)
	p.resources = make(map[ResourceStruct][]byte)
	p.pluginArgs = make(map[string][]string)
	p.pluginPtrs = make(map[string]reflect.Value)

	p.getPluginNamesAndArgs()

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
	if p.flags["l-debug"] {
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

	p.addNamespaceToResources()

	if args[0] == "k" || args[0] == "kubectl" || args[0] == "helm" {

		if p.flags["l-debug"] {
			log.Printf("labeler.go: [debug] namespaceArg: %v", p.params["namespaceArg"])
		}
		// remove the following args for both helm and kubectl because they do not recognize them
		for i := 0; i < len(args); i++ {
			// log.Printf("args: %v", args[i])
			// remove all labeler flags
			if strings.HasPrefix(args[i], "--l-") {
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
		if p.flags["l-debug"] {
			log.Println("labeler.go: [debug] before args: ", args)
		}
		// Run the command with the parsed flags

		if args[0] == "k" || args[0] == "kubectl" {
			if p.flags["l-debug"] {
				log.Println("labeler.go: [debug] after args: ", args)
			}

			originalCommand := strings.Join(args, " ")
			p.originalCmd = originalCommand

			cmd := exec.Command(args[0], args[1:]...)
			out, err := cmd.CombinedOutput()
			if err != nil {
				fmt.Printf("%v", string(out))
				os.Exit(1)
			} else {
				fmt.Printf("%v", string(out))
			}

			p.ClientSet, p.RestConfig, p.DynamicClient = p.switchContext()
			output := strings.TrimSpace(string(out))
			lines := strings.Split(output, "\n")
			p.traverseKubectlOutput(lines)

		} else if args[0] == "helm" {
			// run the original helm command without the extra labeler flags
			output, err := p.runCmd("helm", args[1:])
			if err != nil {
				log.Println(err)
				os.Exit(1)
			} else {
				fmt.Printf("%v", string(output))
			}

			// now run helm as template and label the output
			templateOutput := p.runHelmInTemplateMode(args)

			// set the context and get the helm output into the resources map
			p.ClientSet, p.RestConfig, p.DynamicClient = p.switchContext()
			err = p.traverseHelmOutput(strings.NewReader(string(templateOutput)), os.Stdout)
			if err != nil {
				log.Println("labeler.go: error (to traverseInput):", err)
				return err
			}

		}

		combined := make(map[string]bool)
		for key, value := range p.flags {
			combined[key] = value
		}
		for key := range p.params {
			combined[key] = true
		}
		if p.flags["l-debug"] {
			for key, value := range p.pluginPtrs {
				log.Printf("labeler.go: key: %v, value: %v\n", key, value)
			}
		}

		fnArgs := []reflect.Value{reflect.ValueOf(p), reflect.ValueOf(false)}

		for key := range combined {
			for pkey, value := range p.pluginArgs {
				for _, vCSV := range value {
					v := strings.Split(vCSV, ",")
					if key == v[0] {
						if p.pluginPtrs[pkey].IsValid() {
							p.pluginPtrs[pkey].Call(fnArgs)
						}
					}
				}
			}
		}
		if p.flags["l-debug"] {
			for key, value := range p.resources {
				fmt.Printf("labeler.go: [debug] resources: Key: %s, Value: %s\n", key, value)
			}
		}

	}
	return nil
}

func (p ParamsStruct) addNamespaceToResources() error {
	p.params["namespaceArg"] = ""
	if p.params["namespace"] != "" {
		p.params["namespaceArg"] = p.params["namespace"]
	} else if p.params["n"] != "" {
		p.params["namespaceArg"] = p.params["n"]
	}
	if p.params["namespaceArg"] == "" {
		p.params["namespaceArg"] = "default"
	}

	resource := ResourceStruct{
		Group:      "",
		Version:    "v1",
		Resource:   "namespaces",
		Namespace:  "",
		ObjectName: p.params["namespaceArg"],
	}
	namespaceYAML := Namespace{
		APIVersion: "v1",
		Kind:       "namespace",
		Metadata: Metadata{
			Name: p.params["namespaceArg"],
		},
	}
	yamlData, err := yaml.Marshal(namespaceYAML)
	if err != nil {
		fmt.Println("Error marshaling YAML:", err)
		return err
	}

	p.resources[resource] = yamlData
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
