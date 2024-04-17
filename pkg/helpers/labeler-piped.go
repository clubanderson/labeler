package helpers

import (
	"bufio"
	"bytes"
	"io"
	"log"
	"os"
	"os/exec"
	"runtime"
	"strings"

	c "github.com/clubanderson/labeler/pkg/common"
	"gopkg.in/yaml.v3"
)

func DetectInput(p c.ParamsStruct) error {
	var yamlData interface{}
	var buffer []string
	c.RunResults.DidNotLabel = []string{}

	if IsInputFromPipe() {
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
			_ = err
			// log.Printf("labeler.go: warning: no YAML input was detected %v", err)
		}

		// Check if YAML was provided
		if yamlData != nil {
			// log.Println("labeler.go: YAML data detected in stdin")
			// Do something with the YAML data received - don't need to use history hack in this case - we got valid YAML input from template, --dry-run, or --debug
			err := traverseHelmOutput(strings.NewReader(string(input)), p)
			if err != nil {
				log.Println("labeler.go: error (traverseinput):", err)
				return err
			}
			labelResources(p)
		} else {
			// log.Println("labeler.go: no YAML data detected in stdin, will try to run again with YAML output")
			// time to do it the hard way - many may not like this approach (history hack) - the other options above are more than sufficient for most people's use
			return helmOrKubectl(buffer, p)
		}
	} else {
		// ...otherwise get the file
		log.Println("labeler.go: data is from file")
		file, e := getFile()
		if e != nil {
			return e
		}
		defer file.Close()
		return helmOrKubectl(buffer, p)
	}

	if len(c.RunResults.DidNotLabel) > 0 {
		log.Printf("labeler.go: The following resources do not exist and can be labeled at a later time:\n\n")
		for _, cmd := range c.RunResults.DidNotLabel {
			log.Printf("%v", cmd)
		}
	}
	return nil
}

func helmOrKubectl(input []string, p c.ParamsStruct) error {
	originalCommand, cmdFound, err := getOriginalCommandFromHistory(p)
	if err != nil {
		log.Println("labeler.go: error (get history):", err)
		// os.Exit(1)
	}

	// log.Printf("labeler.go: original command: %q\n\n", originalCommand)

	if cmdFound == "helm" {
		modifiedCommand := strings.Replace(originalCommand, "install", "template", 1)
		modifiedCommandComponents := strings.Split(modifiedCommand, " ")[1:]
		// log.Printf("labeler.go: modified command: %q\n", modifiedCommand)
		// log.Printf("labeler.go: modified command components: %q\n", modifiedCommandComponents)
		output, err := p.RunCmd("helm", modifiedCommandComponents, true)
		if err != nil {
			// log.Println("labeler.go: error (running helm):", err)
			os.Exit(1)
		}

		err = traverseHelmOutput(strings.NewReader(string(output)), p)
		if err != nil {
			log.Println("labeler.go: error (to traverseInput):", err)
			return err
		}

		labelResources(p)

	} else if cmdFound == "kubectl" || cmdFound == "kustomize" {
		traverseKubectlOutput(input, p)
		labelResources(p)
	}
	return nil
}

func getOriginalCommandFromHistory(p c.ParamsStruct) (string, string, error) {
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

	cmd.Env = append(cmd.Env, "PATH="+p.Path)
	cmd.Env = append(cmd.Env, "HOME="+p.HomeDir)

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
