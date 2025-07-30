Kubediff
========

This is a `kubectl diff` alternative that does not [need patch permission](https://github.com/kubernetes/kubectl/issues/981) to operate. Ideal for CI environments with **read-only** access where you want to view diffs in a merge request.

### How it works
You can use the same `-f` and `-R` to specify k8s yaml file of dir with files
- it reads yaml to in-memory k8s object
- then tries to read the same object from k8s
- dumps both objects to yaml files in a temp directory, stripping some unnecessary fields like `resourceVersion` or `managedFields`
- objects have the same structure for comparing, to reduce false diff due to order of keys
- executes command from `KUBECTL_EXTERNAL_DIFF` env, same as `kubectl` (default is `diff -u -N`)
- exit code is: 0=no diff, 1=diff found, >1=error

### Usage
You can download precompiled binary from [Releases](https://github.com/sepich/kubediff/releases) section or compile via:
```bash
go install github.com/sepich/kubediff/cmd/kubediff
```
Usual cli args from `kubectl diff` are available:
```bash
$ kubediff -h
Usage of ./kubediff:
      --cluster string      The name of the kubeconfig cluster to use
      --context string      The name of the kubeconfig context to use
  -f, --filename strings    Filename, directory, or URL to files to compare
      --kubeconfig string   Path to the kubeconfig file to use for CLI requests
  -n, --namespace string    If present, the namespace scope for this CLI request
  -R, --recursive           Process the directory used in -f, --filename recursively
      --token string        Bearer token for authentication to the API server
  -v, --version             Show version and exit
```
