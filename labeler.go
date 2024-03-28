package main

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"os/user"
	"runtime"
	"strings"

	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/restmapper"
)

type inputParamsStruct struct {
	homeDir string
	path    string
}

var flags struct {
	filepath string
	verbose  bool
}

var flagsName = struct {
	file         string
	fileShort    string
	verbose      string
	verboseShort string
}{
	file:         "file",
	fileShort:    "f",
	verbose:      "verbose",
	verboseShort: "v",
}

func main() {
	var p inputParamsStruct

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
		Long: `Simple demo of the usage of linux pipes
	Transform the input (pipe or file) to uppercase letters`,
		RunE: func(cmd *cobra.Command, args []string) error {
			print = logNoop
			if flags.verbose {
				print = logOut
			}
			return p.detectInput()
		},
	}

	// flag for the filepath
	rootCmd.Flags().StringVarP(
		&flags.filepath,
		flagsName.file,
		flagsName.fileShort,
		"", "path to the file")

	// flag for the verbosity level
	rootCmd.PersistentFlags().BoolVarP(
		&flags.verbose,
		flagsName.verbose,
		flagsName.verboseShort,
		false, "log verbose output")

	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

func (p inputParamsStruct) detectInput() error {
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

func (p inputParamsStruct) helmOrKubectl(r io.Reader, w io.Writer) error {
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
		err = toUppercase(strings.NewReader(string(output)), os.Stdout)
		if err != nil {
			fmt.Println("Error (to uppercase):", err)
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
		toUppercase(r, w)
		if err != nil {
			fmt.Println("Error:", err)
			os.Exit(1)
		}
	}

	// gvk (group, version, kind) should come out of toUppercase. you will use yaml decoding to find the '.kind', '.apiVersion', and '.metadata.name' from all yaml records. HINT: apiVersion is the group/version (gv) in gvk, and kind is the kind (k) in gvk
	// p.setLabel(output, group, version, kind, objectName)
	// HACKME!!!

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

func toUppercase(r io.Reader, w io.Writer) error {
	scanner := bufio.NewScanner(bufio.NewReader(r))
	for scanner.Scan() {
		line := scanner.Text()
		// HACKME!!! - this string comparison is wrong - this should use yaml decoding to find the '.kind', '.apiVersion', and '.metadata.name' from all yaml records
		if strings.HasPrefix(line, "kind:") || strings.HasPrefix(line, "apiVersion:") || strings.HasPrefix(line, "  name:") {
			line = strings.ToUpper(line)
			_, err := fmt.Fprintf(w, line+"\n")
			if err != nil {
				return err
			}
		}
	}
	return nil
}

func (p inputParamsStruct) runCmd(cmdToRun string, cmdArgs []string) ([]byte, error) {
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

func (p inputParamsStruct) getOriginalCommandFromHistory() (string, error) {
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
		// ISSUE: unlike mac, ubuntu does not save the command piped into the labeler in history until after labeler is done executing - there has to be a way to get this to work HACKME!!!
		// you can still test with
		//     history -s "helm --kube-context=kind-kind install sealed-secrets sealed-secrets/sealed-secrets -n sealed-secrets --create-namespace"
		//     helm --kube-context=kind-kind install sealed-secrets sealed-secrets/sealed-secrets -n sealed-secrets --create-namespace | ./labeler app.kubernetes.io/part-of=sample-value
		// terrible workaround - but there has to be another way
		cmd = exec.Command("bash", "-c", "history -r ~/.bash_history; history 1")
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
		return "", fmt.Errorf("helm not found")
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

func (p inputParamsStruct) setLabel(output, group, version, kind, objectName string) error {
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
