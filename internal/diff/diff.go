package diff

import (
	"context"
	_ "embed"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/dynamic"
	kyaml "sigs.k8s.io/yaml"
)

//go:embed filter.yml
var filterYAML []byte

var filterObjects map[string]*unstructured.Unstructured

func init() {
	filterObjects = make(map[string]*unstructured.Unstructured)
	for obj := range objectsFromYaml(strings.NewReader(string(filterYAML))) {
		filterObjects[obj.GetKind()] = obj
	}
}

type Options struct {
	Filename    []string
	Recursive   bool
	Cluster     string
	Context     string
	Kubeconfig  string
	Namespace   string
	Token       string
	SkipSecrets bool
}

func Run(opts *Options) (int, error) {
	files, err := expandFilenames(opts.Filename, opts.Recursive)
	if err != nil {
		return 2, fmt.Errorf("failed to expand filenames: %w", err)
	}

	dynamicClient, discoveryClient, err := getClients(opts)
	if err != nil {
		return 2, fmt.Errorf("failed to get clients: %w", err)
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

// diffObject compares a obj with the cluster state, and returns true if there are differences.
func diffObject(fileObj *unstructured.Unstructured, opts *Options, dynamicClient dynamic.Interface, discoveryClient discovery.DiscoveryInterface) (bool, error) {
	gvk := fileObj.GroupVersionKind()
	if gvk.Kind == "Secret" && gvk.Group == "" && opts.SkipSecrets {
		fmt.Fprintf(os.Stderr, "Skipping Secret: %s/%s\n", opts.Namespace, fileObj.GetName())
		return false, nil
	}

	gvr, isNamespaced, err := getGVRAndScope(gvk, discoveryClient)
	if err != nil {
		return false, fmt.Errorf("failed to get GVR for %s: %w", gvk, err)
	}

	namespace := fileObj.GetNamespace()
	if namespace == "" && opts.Namespace != "" && isNamespaced {
		namespace = opts.Namespace
	}

	var resourceInterface dynamic.ResourceInterface
	if isNamespaced && namespace != "" {
		resourceInterface = dynamicClient.Resource(*gvr).Namespace(namespace)
	} else {
		resourceInterface = dynamicClient.Resource(*gvr)
	}

	ctx := context.TODO()
	clusterObj, err := resourceInterface.Get(ctx, fileObj.GetName(), metav1.GetOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			clusterObj = &unstructured.Unstructured{}
		} else {
			return false, fmt.Errorf("failed to get object from cluster: %w", err)
		}
	}

	clusterObj = applyFiltering(fileObj, clusterObj)
	return executeDiff(normalizeObject(fileObj), normalizeObject(clusterObj))
}

// executeDiff runs the diff command on the provided file and cluster objects, and returns true if differences are found.
func executeDiff(fileObj, clusterObj *unstructured.Unstructured) (bool, error) {
	tmpDir, err := os.MkdirTemp("", "kubediff-")
	if err != nil {
		return false, fmt.Errorf("failed to create temp directory: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	fileYAML, err := kyaml.Marshal(fileObj.Object)
	if err != nil {
		return false, fmt.Errorf("failed to marshal file object: %w", err)
	}

	clusterYAML := []byte{}
	if clusterObj.Object != nil && len(clusterObj.Object) != 0 {
		var err error
		clusterYAML, err = kyaml.Marshal(clusterObj.Object)
		if err != nil {
			return false, fmt.Errorf("failed to marshal cluster object: %w", err)
		}
	}

	fn := strings.ReplaceAll(fileObj.GetKind()+"-"+fileObj.GetName(), ":", "-")
	fileTemp, err := os.Create(fmt.Sprintf("%s/f-%s.yaml", tmpDir, fn))
	if err != nil {
		return false, fmt.Errorf("failed to create temp file: %w", err)
	}
	defer os.Remove(fileTemp.Name())
	defer fileTemp.Close()

	clusterTemp, err := os.Create(fmt.Sprintf("%s/c-%s.yaml", tmpDir, fn))
	if err != nil {
		return false, fmt.Errorf("failed to create temp file: %w", err)
	}
	defer os.Remove(clusterTemp.Name())
	defer clusterTemp.Close()

	if _, err := fileTemp.Write(fileYAML); err != nil {
		return false, fmt.Errorf("failed to write file yaml: %w", err)
	}

	if _, err := clusterTemp.Write(clusterYAML); err != nil {
		return false, fmt.Errorf("failed to write cluster yaml: %w", err)
	}

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
