package main

import (
	"log"
	"strings"
)

func (p ParamsStruct) remoteDeployTo() {
	if p.params["remote-contexts"] != "" {
		remoteContexts := strings.Split(p.params["remote-contexts"], ",")

		if (p.flags["kubectl"] || p.flags["k"]) && (p.flags["apply"] || p.flags["create"]) && (p.params["dry-run"] == "") {
			log.Printf(" attempting deployment to contexts: %v\n", remoteContexts)
			if p.flags["debug"] {
				log.Printf("labeler.go: [debug] remoteDeployTo: original command: %q\n", p.originalCmd)
			}
			for _, context := range remoteContexts {
				modifiedCommand := []string{}
				isThereContext := false
				args := strings.Split(p.originalCmd, " ")
				for i := 0; i < len(args); i++ {
					if strings.HasPrefix(args[i], "--context=") {
						modifiedCommand = append(modifiedCommand, "--context="+context)
						isThereContext = true
					} else {
						modifiedCommand = append(modifiedCommand, args[i])
					}
				}
				if isThereContext == false {
					modifiedCommand = append(modifiedCommand, "--context="+context)
				}
				if p.flags["debug"] {
					log.Printf("labeler.go: [debug] modified command components: %v\n", modifiedCommand)
				}

				output, err := p.runCmd("kubectl", modifiedCommand[1:])
				if err != nil {
					log.Println(err)
				} else {
					log.Println(output)
				}
			}
		} else if (p.flags["helm"]) && (p.flags["upgrade"] || p.flags["install"]) && (!p.flags["dry-run"]) {
			log.Printf(" attempting deployment to contexts: %v\n", remoteContexts)
			if p.flags["debug"] {
				log.Printf("labeler.go: [debug] remoteDeployTo: original command: %q\n", p.originalCmd)
			}
			for _, context := range remoteContexts {
				modifiedCommand := []string{}
				isThereContext := false
				args := strings.Split(p.originalCmd, " ")
				for i := 0; i < len(args); i++ {
					if strings.HasPrefix(args[i], "--kube-context=") {
						modifiedCommand = append(modifiedCommand, "--kube-context="+context)
						isThereContext = true
					} else {
						modifiedCommand = append(modifiedCommand, args[i])
					}
				}
				if isThereContext == false {
					modifiedCommand = append(modifiedCommand, "--kube-context="+context)
				}
				if p.flags["debug"] {
					log.Printf("labeler.go: [debug] modified command components: %v\n", modifiedCommand)
				}

				output, err := p.runCmd("helm", modifiedCommand[1:])
				if err != nil {
					log.Println(err)
				} else {
					log.Println(output)
				}
			}
		} else {
			log.Println("logger: deploy-to requested but flags do not include 'apply' or 'create' or 'dry-run'")
		}
	}
}
