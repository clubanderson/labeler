package main

import (
	"bytes"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	k8sYAML "k8s.io/apimachinery/pkg/util/yaml"
	"k8s.io/client-go/discovery/cached/disk"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/restmapper"
	"k8s.io/client-go/tools/clientcmd"
)

func fileExists(filename string) bool {
	_, err := os.Stat(filename)
	return err == nil
}

var print = func(v ...interface{}) {}

func logOut(v ...interface{}) {
	log.Println(v...)
}

func logNoop(v ...interface{}) {}

func (p ParamsStruct) switchContext() (*kubernetes.Clientset, *rest.Config, *dynamic.DynamicClient) {
	var err error
	var kubeConfigPath string

	if flags.kubeconfig == "" {
		kubeConfigPath = filepath.Join(p.homeDir, ".kube", "config")
	} else {
		kubeConfigPath = filepath.Join(flags.kubeconfig)
	}

	// load kubeconfig from file
	apiConfig, err := clientcmd.LoadFromFile(kubeConfigPath)
	if err != nil {
		log.Printf("🔴 error loading kubeconfig: %q\n", err)
		os.Exit(1)
	}

	if flags.context != "" {
		// check if the specified context exists in the kubeconfig
		if _, exists := apiConfig.Contexts[flags.context]; !exists {
			log.Printf("Context %q does not exist in the kubeconfig\n", flags.context)
			os.Exit(1)
		}
		// switch the current context in the kubeconfig
		apiConfig.CurrentContext = flags.context
	}

	// create a new clientset with the updated config
	clientConfig := clientcmd.NewDefaultClientConfig(*apiConfig, &clientcmd.ConfigOverrides{})
	restConfig, err := clientConfig.ClientConfig()
	if err != nil {
		log.Printf("🔴 error creating clientset config: %v\n", err)
		os.Exit(1)
	}
	ocClientset, err := kubernetes.NewForConfig(restConfig)
	if err != nil {
		log.Printf("🔴 error creating clientset: %v\n", err)
		os.Exit(1)
	}
	ocDynamicClient, err := dynamic.NewForConfig(restConfig)
	if err != nil {
		log.Printf("🔴 error create dynamic client: %v\n", err)
		os.Exit(1)
	}

	return ocClientset, restConfig, ocDynamicClient
}

func (p ParamsStruct) createCachedDiscoveryClient(restConfigCoreOrWds rest.Config) (*restmapper.DeferredDiscoveryRESTMapper, error) {
	// create a cached discovery client for the provided config
	cachedDiscoveryClient, err := disk.NewCachedDiscoveryClientForConfig(&restConfigCoreOrWds, p.homeDir, ".cache", 60)
	if err != nil {
		log.Printf("could not get cacheddiscoveryclient: %v", err)
		// handle error
	}
	mapper := restmapper.NewDeferredDiscoveryRESTMapper(cachedDiscoveryClient)
	return mapper, nil
}

func (p ParamsStruct) useContext(contextName string) {
	setContext := []string{"config", "use-context", contextName}
	_, err := p.runCmd("kubectl", setContext)
	if err != nil {
		log.Printf("   🔴 error setting kubeconfig's current context: %v\n", err)
	} else {
		log.Printf("   📍 kubeconfig's current context set to %v\n", contextName)
	}
}

func (p ParamsStruct) getGVRFromGVK(mapper *restmapper.DeferredDiscoveryRESTMapper, gvk schema.GroupVersionKind) (schema.GroupVersionResource, error) {
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

func DecodeYAML(yamlBytes []byte) (*unstructured.Unstructured, error) {
	obj := &unstructured.Unstructured{}
	dec := k8sYAML.NewYAMLOrJSONDecoder(bytes.NewReader(yamlBytes), 4096)
	err := dec.Decode(obj)
	if err != nil {
		return nil, err
	}
	return obj, nil
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

func (p ParamsStruct) runCmd(cmdToRun string, cmdArgs []string) ([]byte, error) {
	fmt.Printf("running command: %v ", cmdToRun)
	for _, arg := range cmdArgs {
		fmt.Printf("%v ", arg)
	}
	log.Println()
	// log.Println(cmdToRun, cmdArgs)
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
