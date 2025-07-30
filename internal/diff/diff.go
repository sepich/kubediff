package diff

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/yaml"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
	kyaml "sigs.k8s.io/yaml"
)

type Options struct {
	Filename   []string
	Recursive  bool
	Cluster    string
	Context    string
	Kubeconfig string
	Namespace  string
	Token      string
}

func Run(opts *Options) (int, error) {
	config, err := buildConfig(opts)
	if err != nil {
		return 2, fmt.Errorf("failed to build config: %w", err)
	}

	dynamicClient, err := dynamic.NewForConfig(config)
	if err != nil {
		return 2, fmt.Errorf("failed to create dynamic client: %w", err)
	}

	discoveryClient, err := discovery.NewDiscoveryClientForConfig(config)
	if err != nil {
		return 2, fmt.Errorf("failed to create discovery client: %w", err)
	}

	files, err := expandFilenames(opts.Filename, opts.Recursive)
	if err != nil {
		return 2, fmt.Errorf("failed to expand filenames: %w", err)
	}

	hasDiff := false
	for _, file := range files {
		diffFound, err := processFile(file, opts, dynamicClient, discoveryClient)
		if err != nil {
			return 2, fmt.Errorf("failed to process file %s: %w", file, err)
		}
		if diffFound {
			hasDiff = true
		}
	}

	if hasDiff {
		return 1, nil
	}
	return 0, nil
}

func buildConfig(opts *Options) (*rest.Config, error) {
	loadingRules := clientcmd.NewDefaultClientConfigLoadingRules()
	if opts.Kubeconfig != "" {
		loadingRules.ExplicitPath = opts.Kubeconfig
	}

	configOverrides := &clientcmd.ConfigOverrides{
		ClusterInfo: clientcmdapi.Cluster{},
		Context: clientcmdapi.Context{
			Namespace: opts.Namespace,
		},
	}

	if opts.Context != "" {
		configOverrides.CurrentContext = opts.Context
	}

	if opts.Cluster != "" {
		configOverrides.Context.Cluster = opts.Cluster
	}

	if opts.Token != "" {
		configOverrides.AuthInfo.Token = opts.Token
	}

	clientConfig := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(loadingRules, configOverrides)
	return clientConfig.ClientConfig()
}

func expandFilenames(filenames []string, recursive bool) ([]string, error) {
	var files []string

	for _, filename := range filenames {
		if stat, err := os.Stat(filename); err != nil {
			return nil, fmt.Errorf("failed to stat %s: %w", filename, err)
		} else if stat.IsDir() {
			if recursive {
				err := filepath.Walk(filename, func(path string, info os.FileInfo, err error) error {
					if err != nil {
						return err
					}
					if !info.IsDir() && (strings.HasSuffix(path, ".yaml") || strings.HasSuffix(path, ".yml")) {
						files = append(files, path)
					}
					return nil
				})
				if err != nil {
					return nil, err
				}
			} else {
				entries, err := ioutil.ReadDir(filename)
				if err != nil {
					return nil, err
				}
				for _, entry := range entries {
					if !entry.IsDir() && (strings.HasSuffix(entry.Name(), ".yaml") || strings.HasSuffix(entry.Name(), ".yml")) {
						files = append(files, filepath.Join(filename, entry.Name()))
					}
				}
			}
		} else {
			files = append(files, filename)
		}
	}

	return files, nil
}

func processFile(filename string, opts *Options, dynamicClient dynamic.Interface, discoveryClient discovery.DiscoveryInterface) (bool, error) {
	data, err := ioutil.ReadFile(filename)
	if err != nil {
		return false, fmt.Errorf("failed to read file: %w", err)
	}

	hasDiff := false
	decoder := yaml.NewYAMLOrJSONDecoder(strings.NewReader(string(data)), 4096)
	for {
		var obj unstructured.Unstructured
		err := decoder.Decode(&obj)
		if err != nil {
			if err.Error() == "EOF" {
				break
			}
			return false, fmt.Errorf("failed to decode YAML: %w", err)
		}

		if obj.GetKind() == "" {
			continue
		}

		diffFound, err := diffObject(&obj, opts, dynamicClient, discoveryClient)
		if err != nil {
			return false, fmt.Errorf("failed to diff object %s/%s: %w", obj.GetKind(), obj.GetName(), err)
		}
		if diffFound {
			hasDiff = true
		}
	}

	return hasDiff, nil
}

func diffObject(fileObj *unstructured.Unstructured, opts *Options, dynamicClient dynamic.Interface, discoveryClient discovery.DiscoveryInterface) (bool, error) {
	gvk := fileObj.GroupVersionKind()

	gvr, isNamespaced, err := getGVRAndScope(gvk, discoveryClient)
	if err != nil {
		return false, fmt.Errorf("failed to get GVR for %s: %w", gvk, err)
	}

	namespace := fileObj.GetNamespace()
	if namespace == "" && opts.Namespace != "" && isNamespaced {
		namespace = opts.Namespace
	}

	var clusterObj *unstructured.Unstructured
	var resourceInterface dynamic.ResourceInterface

	if isNamespaced && namespace != "" {
		resourceInterface = dynamicClient.Resource(*gvr).Namespace(namespace)
	} else {
		resourceInterface = dynamicClient.Resource(*gvr)
	}

	ctx := context.TODO()
	fetchedObj, err := resourceInterface.Get(ctx, fileObj.GetName(), metav1.GetOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			clusterObj = &unstructured.Unstructured{}
		} else {
			return false, fmt.Errorf("failed to get object from cluster: %w", err)
		}
	} else {
		clusterObj = fetchedObj.DeepCopy()
		clusterObj = normalizeObject(clusterObj)
	}

	normalizedFileObj := normalizeObject(fileObj.DeepCopy())

	return executeDiff(normalizedFileObj, clusterObj, fileObj.GetKind(), fileObj.GetName())
}

func normalizeObject(obj *unstructured.Unstructured) *unstructured.Unstructured {
	delete(obj.Object, "status")

	if metadata, ok := obj.Object["metadata"].(map[string]interface{}); ok {
		delete(metadata, "resourceVersion")
		delete(metadata, "uid")
		delete(metadata, "selfLink")
		delete(metadata, "creationTimestamp")
		delete(metadata, "generation")
		delete(metadata, "managedFields")

		// Remove ignored annotations
		if annotations, ok := metadata["annotations"].(map[string]interface{}); ok {
			delete(annotations, "kubectl.kubernetes.io/last-applied-configuration")
			delete(annotations, "deployment.kubernetes.io/revision")
			delete(annotations, "meta.helm.sh/release-name")
			delete(annotations, "meta.helm.sh/release-namespace")

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

	return obj
}

func getGVRAndScope(gvk schema.GroupVersionKind, discoveryClient discovery.DiscoveryInterface) (*schema.GroupVersionResource, bool, error) {
	apiResourceList, err := discoveryClient.ServerResourcesForGroupVersion(gvk.GroupVersion().String())
	if err != nil {
		return nil, false, err
	}

	for _, resource := range apiResourceList.APIResources {
		if resource.Kind == gvk.Kind {
			return &schema.GroupVersionResource{
				Group:    gvk.Group,
				Version:  gvk.Version,
				Resource: resource.Name,
			}, resource.Namespaced, nil
		}
	}

	return nil, false, fmt.Errorf("resource not found for kind %s", gvk.Kind)
}

func executeDiff(fileObj, clusterObj *unstructured.Unstructured, kind, name string) (bool, error) {
	cacheDir := filepath.Join(os.Getenv("HOME"), ".kube", "cache")
	if err := os.MkdirAll(cacheDir, 0755); err != nil {
		return false, fmt.Errorf("failed to create cache directory: %w", err)
	}

	fileYAML, err := kyaml.Marshal(fileObj.Object)
	if err != nil {
		return false, fmt.Errorf("failed to marshal file object: %w", err)
	}

	var clusterYAML []byte
	if clusterObj.Object == nil || len(clusterObj.Object) == 0 {
		clusterYAML = []byte{}
	} else {
		var err error
		clusterYAML, err = kyaml.Marshal(clusterObj.Object)
		if err != nil {
			return false, fmt.Errorf("failed to marshal cluster object: %w", err)
		}
	}

	fileTemp, err := ioutil.TempFile(cacheDir, fmt.Sprintf("%s-%s-file-*.yaml", kind, name))
	if err != nil {
		return false, fmt.Errorf("failed to create temp file: %w", err)
	}
	defer os.Remove(fileTemp.Name())
	defer fileTemp.Close()

	clusterTemp, err := ioutil.TempFile(cacheDir, fmt.Sprintf("%s-%s-cluster-*.yaml", kind, name))
	if err != nil {
		return false, fmt.Errorf("failed to create temp file: %w", err)
	}
	defer os.Remove(clusterTemp.Name())
	defer clusterTemp.Close()

	if _, err := fileTemp.Write(fileYAML); err != nil {
		return false, fmt.Errorf("failed to write file temp: %w", err)
	}

	if _, err := clusterTemp.Write(clusterYAML); err != nil {
		return false, fmt.Errorf("failed to write cluster temp: %w", err)
	}

	fileTemp.Close()
	clusterTemp.Close()

	diffCmd := os.Getenv("KUBECTL_EXTERNAL_DIFF")
	if diffCmd == "" {
		diffCmd = "diff -u -N"
	}

	parts := strings.Fields(diffCmd)
	cmd := exec.Command(parts[0], append(parts[1:], clusterTemp.Name(), fileTemp.Name())...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		if exitError, ok := err.(*exec.ExitError); ok {
			if exitError.ExitCode() == 1 {
				// Exit code 1 means differences found
				return true, nil
			}
		}
		return false, fmt.Errorf("diff command failed: %w", err)
	}

	// Exit code 0 means no differences
	return false, nil
}
