package labeler

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"gopkg.in/yaml.v3"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
)

func (p ParamsStruct) createObjForPlugin(gvk schema.GroupVersionKind, yamlData []byte, objName, objResource, namespace string) {
	// Unmarshal YAML data into a map
	var objMap map[string]interface{}
	err := yaml.Unmarshal([]byte(yamlData), &objMap)
	if err != nil {
		fmt.Println("Error unmarshaling YAML:", err)
		return
	}

	// Marshal the map into JSON
	objectJSON, err := json.Marshal(objMap)
	if err != nil {
		fmt.Println("Error marshaling JSON:", err)
		return
	}

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

	if p.flags["debug"] {
		log.Printf("  ℹ️  object info %v/%v/%v %v\n", nsgvr.Group, nsgvr.Version, nsgvr.Resource, namespace)
	}
	_, err = p.getObject(p.DynamicClient, "", nsgvr, namespace)
	if err != nil {
		log.Printf("  🔴 failed to create %v %q, namespace %q does not exist. Is KubeStellar installed?\n", objResource, objName, namespace)
	} else {
		_, err = p.createObject(p.DynamicClient, namespace, gvr, objectJSON)
		if err != nil {
			log.Printf("  🔴 failed to create %v object %q in namespace %v. Is KubeStellar installed?\n", objResource, objName, namespace)
		}
	}
}

func (p ParamsStruct) createObject(ocDynamicClientCoreOrWds dynamic.Interface, namespace string, gvr schema.GroupVersionResource, objectJSON []byte) (string, error) {
	objToCreate := &unstructured.Unstructured{}

	// printUnstructured(objToCreate)

	// unmarshal the JSON data into the Unstructured object
	err := objToCreate.UnmarshalJSON(objectJSON)
	if err != nil {
		log.Printf("%v\n", err)
		return namespace, nil
	}

	_, err = p.getObject(ocDynamicClientCoreOrWds, namespace, gvr, objToCreate.GetName())
	if err == nil {
		// object still exists, can't create
		if p.flags["debug"] {
			log.Printf("          ℹ️  object exists %v/%v/%v %v\n", gvr.Group, gvr.Version, gvr.Resource, objToCreate.GetName())
		}
		return namespace, err
	}

	// log.Printf("          ℹ️  object info %v/%v/%v %v\n", gvr.Group, gvr.Version, gvr.Resource, objToCreate.GetName())
	if errors.IsNotFound(err) {
		retryCount := 10
		for attempt := 1; attempt <= retryCount; attempt++ {
			if namespace == "" {
				_, err = ocDynamicClientCoreOrWds.Resource(gvr).Create(context.TODO(), objToCreate, metav1.CreateOptions{})

			} else {
				_, err = ocDynamicClientCoreOrWds.Resource(gvr).Namespace(namespace).Create(context.TODO(), objToCreate, metav1.CreateOptions{})
			}
			if err == nil {
				break
			}
			if p.flags["debug"] {
				log.Printf("          ℹ️  object %s is being created (if error, namespace might be missing from resource definition). Retrying in 5 seconds: %v/%v/%v: %v\n", objToCreate.GetName(), gvr.Group, gvr.Version, gvr.Resource, err)
			}
			time.Sleep(5 * time.Second)
			continue
		}

		if p.flags["debug"] {
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

func (p ParamsStruct) getObject(ocDynamicClientCoreOrWds dynamic.Interface, namespace string, gvr schema.GroupVersionResource, objectName string) ([]byte, error) {

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

	if p.flags["debug"] {
		if err != nil {
			// log.Printf("          > object not found %v/%v/%v %v in %q: %v\n", gvr.Group, gvr.Version, gvr.Resource, objectName, namespace, err)
			return nil, err

		} else {
			// log.Printf("          > found object %v/%v/%v %v in %q\n", gvr.Group, gvr.Version, gvr.Resource, objectName, namespace)
			return objectJSON, nil
		}
	}
	if err != nil {
		return nil, err
	}
	return objectJSON, nil
}
