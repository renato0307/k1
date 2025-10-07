package dummy

import (
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// Formatter provides fake YAML/describe output for development
type Formatter struct{}

func NewFormatter() *Formatter {
	return &Formatter{}
}

func (f *Formatter) GetResourceYAML(gvr schema.GroupVersionResource, namespace, name string) (string, error) {
	// Return dummy YAML for development
	return `apiVersion: v1
kind: Pod
metadata:
  name: ` + name + `
  namespace: ` + namespace + `
status:
  phase: Running`, nil
}

func (f *Formatter) DescribeResource(gvr schema.GroupVersionResource, namespace, name string) (string, error) {
	// Return dummy describe output for development
	return `Name:         ` + name + `
Namespace:    ` + namespace + `
Status:       Running
(Dummy data - connect to real cluster for actual describe output)`, nil
}
