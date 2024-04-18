package main

import (
	"log"
	"os"
	"os/user"

	c "github.com/clubanderson/labeler/pkg/common"
	h "github.com/clubanderson/labeler/pkg/helpers"
	"github.com/spf13/cobra"
)

func main() {
	log.SetFlags(0) // remove the date and time stamp from log.print output
	var p c.ParamsStruct

	currentUser, err := user.Current()
	if err != nil {
		log.Println("labeler.go: error (current user):", err)
		return
	}
	p.HomeDir = currentUser.HomeDir
	p.Path = os.Getenv("PATH")

	if !h.IsInputFromPipe() {
		if len(os.Args) <= 1 {
			log.Printf("no arguments given, need usage here (TODO)")
		} else {
			args := os.Args[1:]
			if args[0] == "--version" || args[0] == "-v" {
				log.Printf("labeler version %v\n", c.Version)
			}
			if len(args) > 0 {
				if args[0] == "k" || args[0] == "h" || args[0] == "kubectl" || args[0] == "helm" {
					// log.Println("labeler.go: invoked as alias: ")
					h.AliasRun(args, p)
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
					log.Printf("labeler version %v\n", c.Version)
					return
				}
				if c.Flags.Label == "" {
					log.Println("labeler.go: no label provided")
					os.Exit(1)
				}

				print = logNoop
				if c.Flags.Verbose {
					print = logOut
				}
				p.ClientSet, p.RestConfig, p.DynamicClient = h.SwitchContext(p)

				h.DetectInput(p)
			},
		}

		rootCmd.SetFlagErrorFunc(func(cmd *cobra.Command, err error) error {
			cmd.Println(err)
			cmd.Println(cmd.UsageString())
			return SilentErr(err)
		})
		rootCmd.Flags().BoolVar(&versionFlag, "version", false, "print the version")
		// rootCmd.Flags().StringVarP(&flags.filepath, flagsName.file, flagsName.fileShort, "", "path to the file")
		rootCmd.PersistentFlags().StringVarP(&c.Flags.Label, c.FlagsName.Label, c.FlagsName.LabelShort, "", "label to apply to all resources e.g. -l app.kubernetes.io/part-of=sample-value")
		rootCmd.PersistentFlags().StringVarP(&c.Flags.Annotation, c.FlagsName.Annotation, c.FlagsName.AnnotationShort, "", "annotation to apply to all resources e.g. --annotation=creator='John Doe'")
		rootCmd.PersistentFlags().StringVarP(&c.Flags.Kubeconfig, c.FlagsName.Kubeconfig, c.FlagsName.KubeconfigShort, "", "kubeconfig to use")
		rootCmd.PersistentFlags().StringVarP(&c.Flags.Context, c.FlagsName.Context, c.FlagsName.ContextShort, "", "context to use")
		rootCmd.PersistentFlags().BoolVarP(&c.Flags.Verbose, c.FlagsName.Verbose, c.FlagsName.VerboseShort, false, "log verbose output")
		rootCmd.PersistentFlags().BoolVarP(&c.Flags.Debug, c.FlagsName.Debug, c.FlagsName.DebugShort, false, "debug mode")
		rootCmd.PersistentFlags().BoolVarP(&c.Flags.Overwrite, c.FlagsName.Overwrite, c.FlagsName.OverwriteShort, false, "overwrite mode")

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

var print = func(v ...interface{}) {}

func logOut(v ...interface{}) {
	log.Println(v...)
}

func logNoop(v ...interface{}) {}
