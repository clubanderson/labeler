package pluginHelp

import (
	"fmt"
	"log"
	"strings"

	c "github.com/clubanderson/labeler/pkg/common"
)

func PluginHelp(p c.ParamsStruct, reflect bool) []string {
	// function must be exportable (capitalize first letter of function name) to be discovered by labeler
	if reflect {
		return []string{"l-help,flag,displays this help message"}
	}
	log.Println()
	log.Println("Labeler supported parameters and flags")
	for k, v := range p.PluginArgs {
		log.Printf("\n  plugin: %q", k)
		for _, vCSV := range v {
			v := strings.Split(vCSV, ",")
			flagWidth := 35
			value1Width := 10
			formatString := fmt.Sprintf("    %%-%ds  %%-%ds  %%s\n", flagWidth, value1Width)
			log.Printf(formatString, "--"+v[0], "("+v[1]+")", strings.Join(v[2:], ","))
		}
	}
	log.Println()

	return []string{}
}
