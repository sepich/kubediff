package filter

import (
	_ "embed"
	"errors"
	"os"
	"strings"

	"github.com/sepich/kubediff/internal/store"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

//go:embed filter.yml
var builtinYAML []byte

type Filter struct {
	filterObjects map[string]*unstructured.Unstructured
}

func NewFilter(fn string) (*Filter, error) {
	var err error
	f := &Filter{
		filterObjects: make(map[string]*unstructured.Unstructured),
	}
	data := builtinYAML
	if fn != "" {
		data, err = os.ReadFile(fn)
		if err != nil {
			return nil, err
		}
	}
	for obj := range store.YamlToObj(strings.NewReader(string(data))) {
		if obj == nil {
			return nil, errors.New("failed to decode YAML")
		}
		f.filterObjects[obj.GetKind()] = obj
	}
	return f, nil
}

// Apply applies filtering rules to drop fields from clusterObj, if not set in fileObj
func (f Filter) Apply(fileObj, clusterObj *unstructured.Unstructured) {
	normalizeObject(clusterObj)
	normalizeObject(fileObj)

	kind := fileObj.GetKind()
	filterObj, exists := f.filterObjects[kind]
	if !exists {
		return
	}
	applyFilteringRecursive(fileObj.Object, clusterObj.Object, filterObj.Object)
}

// applyFilteringRecursive recursively applies filtering rules
func applyFilteringRecursive(fileData, clusterData, filterData map[string]any) {
	for filterKey, filterValue := range filterData {
		clusterValue, clusterHasKey := clusterData[filterKey]
		if !clusterHasKey {
			continue
		}
		fileValue, fileHasKey := fileData[filterKey]

		switch fv := filterValue.(type) {
		case bool:
			if !fileHasKey && (clusterValue == fv) {
				delete(clusterData, filterKey)
			}
		case string:
			if !fileHasKey && (clusterValue == fv || fv == "*") {
				delete(clusterData, filterKey)
			}
			if fv == "$" { //workarounds
				handleSpecialCases(fileData, clusterData, filterKey, fileHasKey)
			}
		case int64:
			if !fileHasKey && clusterValue == fv {
				delete(clusterData, filterKey)
			}
		case map[string]any: // it is a map, recurse
			fileMap := map[string]any{}
			if fileHasKey {
				if fm, ok := fileValue.(map[string]any); ok {
					fileMap = fm
				}
			}
			if clusterMap, ok := clusterValue.(map[string]any); ok {
				applyFilteringRecursive(fileMap, clusterMap, fv)
				if !fileHasKey && len(clusterMap) == 0 {
					delete(clusterData, filterKey)
				}
			}
		case []any: // handle arrays by template (like containers, initContainers)
			if clusterArray, ok := clusterValue.([]any); ok {
				var fileArray []any
				if fileHasKey {
					if fa, ok := fileValue.([]any); ok {
						fileArray = fa
					}
				}
				applyFilteringToArray(fileArray, clusterArray, fv)
			}
		}
	}
}

// applyFilteringToArray applies filtering to array elements
func applyFilteringToArray(fileArray, clusterArray, filterArray []any) {
	if len(filterArray) == 0 {
		return
	}

	// Get the filter template from the first element
	filterTemplate, ok := filterArray[0].(map[string]any)
	if !ok {
		return
	}

	// Apply the filter template to each element in the cluster array
	for i, clusterItem := range clusterArray {
		if clusterMap, ok := clusterItem.(map[string]any); ok {
			fileMap := map[string]any{}
			if i < len(fileArray) {
				if fm, ok := fileArray[i].(map[string]any); ok {
					fileMap = fm
				}
			}
			applyFilteringRecursive(fileMap, clusterMap, filterTemplate)
		}
	}
}

// handleSpecialCases try to duplicate k8s behaviour for some values
func handleSpecialCases(fileData, clusterData map[string]any, filterKey string, fileHasKey bool) {
	if filterKey == "serviceAccount" && fileHasKey {
		if _, ok := fileData["serviceAccountName"]; !ok {
			delete(clusterData, "serviceAccountName")
		}
	}
	if filterKey == "serviceAccountName" && fileHasKey {
		if _, ok := fileData["serviceAccount"]; !ok {
			delete(clusterData, "serviceAccount")
		}
	}
	if filterKey == "imagePullPolicy" && !fileHasKey {
		val := "IfNotPresent"
		if image, ok := fileData["image"]; ok {
			tag := ""
			j := strings.LastIndex(image.(string), ":")
			tag = image.(string)[j+1:]
			if tag == "latest" || j == -1 {
				val = "Always"
			}
		}
		fileData["imagePullPolicy"] = val
	}
	if filterKey == "targetPort" && !fileHasKey {
		if port, ok := fileData["port"]; ok {
			fileData["targetPort"] = port
		}
	}
	if filterKey == "listKind" && !fileHasKey {
		if kind, ok := fileData["kind"]; ok {
			fileData["listKind"] = kind.(string) + "List"
		}
	}
}

func normalizeObject(obj *unstructured.Unstructured) {
	delete(obj.Object, "status")

	if metadata, ok := obj.Object["metadata"].(map[string]interface{}); ok {
		delete(metadata, "resourceVersion")
		delete(metadata, "uid")
		delete(metadata, "selfLink")
		delete(metadata, "creationTimestamp")
		delete(metadata, "generation")
		delete(metadata, "managedFields")
		delete(metadata, "namespace")

		// Remove ignored annotations
		if annotations, ok := metadata["annotations"].(map[string]interface{}); ok {
			delete(annotations, "kubectl.kubernetes.io/last-applied-configuration")
			delete(annotations, "deployment.kubernetes.io/revision")
			delete(annotations, "meta.helm.sh/release-name")
			delete(annotations, "meta.helm.sh/release-namespace")
			delete(annotations, "policies.kyverno.io/last-applied-patches")

			// Remove empty annotations map
			if len(annotations) == 0 {
				delete(metadata, "annotations")
			}
		}

		// Remove ignored labels
		if labels, ok := metadata["labels"].(map[string]interface{}); ok {
			delete(labels, "helm.sh/chart")
			delete(labels, "app.kubernetes.io/managed-by")

			// Remove empty labels map
			if len(labels) == 0 {
				delete(metadata, "labels")
			}
		}
	}
}
