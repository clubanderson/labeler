package pluginRemoteDeploy

import (
	"log"
	"strings"

	c "github.com/clubanderson/labeler/pkg/common"
)

func PluginRemoteDeployTo(p c.ParamsStruct, reflect bool) []string {
	// function must be exportable (capitalize first letter of function name) to be discovered by labeler
	if reflect {
		return []string{"l-remote-contexts,string,comma-separated list of remote contexts to deploy to (usage: --l-remote-contexts=cluster1,cluster2,cluster3)"}
	}
	supportedArgs := []string{"l-remote-contexts"}
	_ = supportedArgs
	if p.Params["l-remote-contexts"] != "" {
		remoteContexts := strings.Split(p.Params["l-remote-contexts"], ",")

		if (p.Flags["kubectl"] || p.Flags["k"]) && (p.Flags["apply"] || p.Flags["create"]) && (p.Params["dry-run"] == "") {
			log.Printf(" attempting deployment to contexts: %v\n", remoteContexts)
			if p.Flags["l-debug"] {
				log.Printf("labeler.go: [debug] remoteDeployTo: original command: %q\n", p.OriginalCmd)
			}
			for _, context := range remoteContexts {
				modifiedCommand := []string{}
				isThereContext := false
				args := strings.Split(p.OriginalCmd, " ")
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
				if p.Flags["l-debug"] {
					log.Printf("labeler.go: [debug] modified command components: %v\n", modifiedCommand)
				}

				output, err := p.RunCmd("kubectl", modifiedCommand[1:], false)
				if err != nil {
					log.Println(err)
				} else {
					log.Println(output)
				}
			}
		} else if (p.Flags["helm"]) && (p.Flags["upgrade"] || p.Flags["install"]) && (!p.Flags["dry-run"]) {
			log.Printf(" attempting deployment to contexts: %v\n", remoteContexts)
			if p.Flags["l-debug"] {
				log.Printf("labeler.go: [debug] remoteDeployTo: original command: %q\n", p.OriginalCmd)
			}
			for _, context := range remoteContexts {
				modifiedCommand := []string{}
				isThereContext := false
				args := strings.Split(p.OriginalCmd, " ")
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
				if p.Flags["l-debug"] {
					log.Printf("labeler.go: [debug] modified command components: %v\n", modifiedCommand)
				}

				output, err := p.RunCmd("helm", modifiedCommand[1:], false)
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
	return []string{}
}
