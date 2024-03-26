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
	"strings"

	"github.com/pkg/errors"
	"github.com/spf13/cobra"
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
		log.Println("Error:", err)
		return
	}
	p.homeDir = currentUser.HomeDir
	p.path = os.Getenv("PATH")

	var rootCmd = &cobra.Command{
		Use:   "labeler",
		Short: "Transform the input to uppercase letters",
		Long: `Simple demo of the usage of linux pipes
	Transform the input (pipe or file) to uppercase letters`,
		RunE: func(cmd *cobra.Command, args []string) error {
			print = logNoop
			if flags.verbose {
				print = logOut
			}
			return p.runCommand()
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

func (p inputParamsStruct) runCommand() error {
	if isInputFromPipe() {
		// if input is from a pipe, upper case the
		// content of stdin
		print("data is from pipe")
		return p.toLabel(os.Stdin, os.Stdout)
	} else {
		// ...otherwise get the file
		file, e := getFile()
		if e != nil {
			return e
		}
		defer file.Close()
		return p.toLabel(file, os.Stdout)
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

func (p inputParamsStruct) toLabel(r io.Reader, w io.Writer) error {
	originalCommand, err := p.detectOriginalCommand()
	if err != nil {
		fmt.Println("Error:", err)
		os.Exit(1)
	}

	fmt.Printf("Original command: %q\n", originalCommand)
	if strings.HasPrefix(originalCommand, "helm") {
		fmt.Printf("your running helm\n")
	} else if strings.HasPrefix(originalCommand, "kubectl") {
		fmt.Printf("your running kubectl\n")
	}

	originalCommandComponents := []string{"install", "sealed-secrets"}
	originalCommandComponents = append(originalCommandComponents, strings.Split(originalCommand, " ")[2:]...)
	err = p.runCmd("helm", originalCommandComponents)
	if err != nil {
		fmt.Println("Error:", err)
		os.Exit(1)
	}

	toUppercase(r, w)
	if err != nil {
		fmt.Println("Error:", err)
		os.Exit(1)
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

func toUppercase(r io.Reader, w io.Writer) error {
	scanner := bufio.NewScanner(bufio.NewReader(r))
	for scanner.Scan() {
		line := scanner.Text()
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

func (p inputParamsStruct) runCmd(cmdToRun string, cmdArgs []string) error {
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
		return err
	}

	err = cmd.Wait()
	if err != nil {
		log.Println("   🔴 error waiting for command to complete:", err)
		log.Println(string(outputBuf.Bytes()))
		return err
	}

	fmt.Println(string(outputBuf.Bytes()))

	return err
}

func (p inputParamsStruct) detectOriginalCommand() (string, error) {
	// TODO: this may not always be zsh, could be bash - should check if bash_history or zsh_history has "labeler" in it - that would tell us we have the right history file
	cmd := exec.Command("bash", "-c", "history -r ~/.zsh_history; history 1")
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

	originalCmd, err := extractCmd(string(outputBuf.Bytes()))
	return originalCmd, err
}

func extractCmd(historyText string) (string, error) {
	// Find the index of the first semicolon
	semicolonIndex := strings.Index(historyText, ";")
	if semicolonIndex == -1 {
		return "", fmt.Errorf("semicolon not found")
	}

	// trim everything before the semicolon and trim any leading or trailing whitespace
	trimmedCommand := strings.TrimSpace(historyText[semicolonIndex+1:])

	// find the index of the first pipe character in the trimmed command
	pipeIndex := strings.Index(trimmedCommand, "|")
	if pipeIndex == -1 {
		return "", fmt.Errorf("pipe character not found")
	}

	// trim everything after the pipe character and trim any leading or trailing whitespace
	return strings.TrimSpace(trimmedCommand[:pipeIndex]), nil

}
