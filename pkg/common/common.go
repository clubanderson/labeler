package common

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"os/user"
	"reflect"
	"strings"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

var Version = "0.18.2"

// Plugin interface
type Plugin interface {
	PluginRun() []string
}

type ResourceStruct struct {
	Group      string
	Version    string
	Resource   string
	Namespace  string
	ObjectName string
}

type ParamsStruct struct {
	HomeDir       string
	Path          string
	OriginalCmd   string
	Kubeconfig    string
	ClientSet     *kubernetes.Clientset
	RestConfig    *rest.Config
	DynamicClient *dynamic.DynamicClient
	Flags         map[string]bool
	Params        map[string]string
	Resources     map[ResourceStruct][]byte
	PluginArgs    map[string][]string
	PluginPtrs    map[string]reflect.Value
}

type ResultsStruct struct {
	DidNotLabel    []string
	DidNotAnnotate []string
}

var RunResults ResultsStruct

var Flags struct {
	Filepath   string
	Debug      bool
	Verbose    bool
	Label      string
	Annotation string
	Kubeconfig string
	Context    string
	Overwrite  bool
}

var FlagsName = struct {
	File            string
	FileShort       string
	Verbose         string
	VerboseShort    string
	Debug           string
	DebugShort      string
	Annotation      string
	AnnotationShort string
	Label           string
	LabelShort      string
	Kubeconfig      string
	KubeconfigShort string
	Context         string
	ContextShort    string
	Overwrite       string
	OverwriteShort  string
}{
	File:            "file",
	FileShort:       "f",
	Verbose:         "verbose",
	VerboseShort:    "v",
	Debug:           "debug",
	DebugShort:      "d",
	Annotation:      "annotation",
	AnnotationShort: "a",
	Label:           "label",
	LabelShort:      "l",
	Kubeconfig:      "kubeconfig",
	KubeconfigShort: "k",
	Context:         "context",
	ContextShort:    "c",
	Overwrite:       "overwrite",
	OverwriteShort:  "o",
}

type Metadata struct {
	Name string `yaml:"name"`
}

type Namespace struct {
	APIVersion string   `yaml:"apiVersion"`
	Kind       string   `yaml:"kind"`
	Metadata   Metadata `yaml:"metadata"`
}

func (p ParamsStruct) RunCmd(cmdToRun string, cmdArgs []string, suppressOutput bool) ([]byte, error) {
	cmdArgs = expandTilde(cmdArgs)

	cmd := exec.Command(cmdToRun, cmdArgs...)
	cmd.Env = append(cmd.Env, "PATH="+p.Path)
	cmd.Env = append(cmd.Env, "HOME="+p.HomeDir)
	cmd.Env = append(cmd.Env, "KUBECONFIG="+os.Getenv("KUBECONFIG"))

	var outputBuf bytes.Buffer
	if suppressOutput {
		cmd.Stdout = io.MultiWriter(&outputBuf)
	} else {
		cmd.Stdout = io.MultiWriter(&outputBuf, os.Stdout)
	}
	cmd.Stderr = io.MultiWriter(&outputBuf, os.Stderr)
	cmd.Stdin = os.Stdin

	err := cmd.Start()
	if err != nil {
		log.Println("labeler.go: error starting command:", err)
		return nil, err
	}

	err = cmd.Wait()
	if err != nil {
		// log.Println("labeler.go: error waiting for command to complete:", err)
		// log.Printf(string(outputBuf.Bytes()))
		return nil, err
	}
	return outputBuf.Bytes(), nil
}

func expandTilde(args []string) []string {
	for i, arg := range args {
		if strings.Contains(arg, "~") {
			usr, err := user.Current()
			if err != nil {
				log.Printf("Error getting current user: %v\n", err)
				return args
			}
			args[i] = strings.ReplaceAll(args[i], "~", usr.HomeDir)
		}
	}
	return args
}

func (p ParamsStruct) CreateObjForPlugin(gvk schema.GroupVersionKind, yamlData []byte, objName, objResource, namespace string, objectJSON []byte) {
	gvr := schema.GroupVersionResource{
		Group:    gvk.Group,
		Version:  gvk.Version,
		Resource: objResource,
	}

	nsgvr := schema.GroupVersionResource{
		Group:    "",
		Version:  "v1",
		Resource: "namespaces",
	}

	if p.Flags["l-debug"] {
		log.Printf("  ℹ️  object info %v/%v/%v %v\n", nsgvr.Group, nsgvr.Version, nsgvr.Resource, namespace)
	}

	_, err := p.createObject(p.DynamicClient, namespace, gvr, objectJSON)
	if err != nil {
		log.Printf("  🔴 failed to create %v object %q in namespace %q: %v. Check if %q CRD is missing from cluster.\n", objResource, objName, namespace, err, objResource)
	}
}

func (p ParamsStruct) createObject(ocDynamicClientCoreOrWds dynamic.Interface, namespace string, gvr schema.GroupVersionResource, objectJSON []byte) (string, error) {
	var objMap map[string]interface{}
	err := json.Unmarshal(objectJSON, &objMap)
	if err != nil {
		fmt.Println("Error unmarshaling JSON:", err)
		return namespace, err
	}

	// Create an unstructured.Unstructured object from the map
	objToCreate := &unstructured.Unstructured{Object: objMap}

	// Now objToCreate is an unstructured.Unstructured object representing the JSON data
	// log.Printf("objToCreate: %v\n", objToCreate)
	metadata, ok, _ := unstructured.NestedMap(objToCreate.Object, "Metadata")
	if !ok {
		fmt.Println("Metadata section not found")
		return namespace, err
	}
	name, ok, _ := unstructured.NestedString(metadata, "Name")
	if !ok {
		fmt.Println("Name not found")
		return namespace, err
	}

	// log.Printf("name: %v\n", name)

	_, err = p.GetObject(ocDynamicClientCoreOrWds, namespace, gvr, name)
	if err == nil {
		// object still exists, can't create
		if p.Flags["l-debug"] {
			log.Printf("          ℹ️  object exists %v/%v/%v %v\n", gvr.Group, gvr.Version, gvr.Resource, name)
		}
		return namespace, err
	}

	// log.Printf("          ℹ️  object info %v/%v/%v %v\n", gvr.Group, gvr.Version, gvr.Resource, objToCreate.GetName())
	if errors.IsNotFound(err) {
		retryCount := 3
		for attempt := 1; attempt <= retryCount; attempt++ {
			if namespace == "" {
				_, err = ocDynamicClientCoreOrWds.Resource(gvr).Create(context.TODO(), objToCreate, metav1.CreateOptions{})

			} else {
				_, err = ocDynamicClientCoreOrWds.Resource(gvr).Namespace(namespace).Create(context.TODO(), objToCreate, metav1.CreateOptions{})
			}
			if err == nil {
				break
			}
			if p.Flags["l-debug"] {
				log.Printf("          ℹ️  object %s is being created (if error, namespace might be missing from resource definition). Retrying in 5 seconds: %v/%v/%v: %v\n", objToCreate.GetName(), gvr.Group, gvr.Version, gvr.Resource, err)
			}
			time.Sleep(5 * time.Second)
			continue
		}

		if p.Flags["l-debug"] {
			if err != nil {
				if namespace == "" {
					log.Printf("       🟡 error creating object %v/%v/%v %v: %v\n", gvr.Group, gvr.Version, gvr.Resource, objToCreate.GetName(), err)
					return namespace, err
				} else {
					log.Printf("       🟡 error creating object %v/%v/%v %v in %v: %v\n", gvr.Group, gvr.Version, gvr.Resource, objToCreate.GetName(), namespace, err)
					return namespace, err
				}
			} else {
				if namespace == "" {
					log.Printf("          ✨ created object %v/%v/%v %q\n", gvr.Group, gvr.Version, gvr.Resource, objToCreate.GetName())
					return namespace, nil
				} else {
					log.Printf("          ✨ created object %v/%v/%v %q in %v\n", gvr.Group, gvr.Version, gvr.Resource, objToCreate.GetName(), namespace)
					return namespace, nil
				}
			}
		}
	}
	if err != nil {
		return namespace, err
	}
	return namespace, nil
}

func (p ParamsStruct) GetObject(ocDynamicClientCoreOrWds dynamic.Interface, namespace string, gvr schema.GroupVersionResource, objectName string) ([]byte, error) {

	var tempObj *unstructured.Unstructured
	var err error
	if namespace == "" {
		tempObj, err = ocDynamicClientCoreOrWds.Resource(gvr).Get(context.TODO(), objectName, metav1.GetOptions{})
		if err != nil {
			_ = err
			// log.Println("1 did not find object")
		} else {
			_ = err
			// log.Println("2 found object")
		}
	} else {
		tempObj, err = ocDynamicClientCoreOrWds.Resource(gvr).Namespace(namespace).Get(context.TODO(), objectName, metav1.GetOptions{})
		if err != nil {
			_ = err
			// log.Println("3 did not find object")
		} else {
			_ = err
			// log.Println("4 found object")
		}
	}

	// var objectJSON []byte
	objectJSON, errMarshal := json.Marshal(tempObj)
	if errMarshal != nil {
		return nil, errMarshal
	}

	if err != nil {
		if p.Flags["l-debug"] {
			log.Printf("          > object not found %v/%v/%v %q in %q: %v\n", gvr.Group, gvr.Version, gvr.Resource, objectName, namespace, err)
		}
		return nil, err

	} else {
		if p.Flags["l-debug"] {
			log.Printf("          > found object %v/%v/%v %q in %q\n", gvr.Group, gvr.Version, gvr.Resource, objectName, namespace)
		}
		return objectJSON, nil
	}

	if err != nil {
		return nil, err
	}
	return objectJSON, nil
}
