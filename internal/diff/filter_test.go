package diff

import (
	"strings"
	"testing"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func TestApplyFiltering(t *testing.T) {
	tests := []struct {
		name        string
		fileYaml    string
		clusterYaml string
		hasDiff     bool
	}{
		{
			name: "Deployment",
			fileYaml: `
apiVersion: apps/v1
kind: Deployment
metadata:
  name: test
spec:
  selector:
    matchLabels:
      app: test
  template:
    metadata:
      labels:
        app: test
    spec:
      containers:
        - name: container
          image: nginx
      initContainers:
        - name: init
          image: busybox:musl
      serviceAccount: sa`,
			clusterYaml: `
apiVersion: apps/v1
kind: Deployment
metadata:
  annotations:
    deployment.kubernetes.io/revision: "1"
  creationTimestamp: "2025-08-02T08:03:17Z"
  generation: 1
  name: test
  resourceVersion: "22560965"
  uid: 19bef6e0-e3db-4f55-824d-234fd5706db5
spec:
  progressDeadlineSeconds: 600
  replicas: 1
  revisionHistoryLimit: 10
  selector:
    matchLabels:
      app: test
  strategy:
    rollingUpdate:
      maxSurge: 25%
      maxUnavailable: 25%
    type: RollingUpdate
  template:
    metadata:
      creationTimestamp: null
      labels:
        app: test
    spec:
      containers:
      - image: nginx
        imagePullPolicy: Always
        name: container
        resources: {}
        terminationMessagePath: /dev/termination-log
        terminationMessagePolicy: File
      dnsPolicy: ClusterFirst
      initContainers:
      - image: busybox:musl
        imagePullPolicy: IfNotPresent
        name: init
        resources: {}
        terminationMessagePath: /dev/termination-log
        terminationMessagePolicy: File
      restartPolicy: Always
      schedulerName: default-scheduler
      securityContext: {}
      serviceAccount: sa
      serviceAccountName: sa
      terminationGracePeriodSeconds: 30
status:
  availableReplicas: 1
  conditions: []
  observedGeneration: 1
  readyReplicas: 1
  replicas: 1
  updatedReplicas: 1`,
			hasDiff: false,
		},
		{
			name: "Deployment, no SA",
			fileYaml: `
apiVersion: apps/v1
kind: Deployment
spec:
  template:
    spec:
      containers:
        - name: container
          image: nginx`,
			clusterYaml: `
apiVersion: apps/v1
kind: Deployment
spec:
  template:
    spec:
      containers:
      - image: nginx
        imagePullPolicy: Always
        name: container
        resources: {}
        terminationMessagePath: /dev/termination-log
        terminationMessagePolicy: File
      terminationGracePeriodSeconds: 30`,
			hasDiff: false,
		},
		{
			name: "Deployment, imagePullPolicy",
			fileYaml: `
apiVersion: apps/v1
kind: Deployment
spec:
  template:
    spec:
      containers:
        - name: container
          image: nginx:latest`,
			clusterYaml: `
apiVersion: apps/v1
kind: Deployment
spec:
  template:
    spec:
      containers:
      - name: container
        image: nginx
        imagePullPolicy: IfNotPresent`,
			hasDiff: true,
		},
		{
			name: "Service",
			fileYaml: `
apiVersion: v1
kind: Service
metadata:
  name: test
spec:
  selector:
    app: test
  ports:
    - port: 80`,
			clusterYaml: `
apiVersion: v1
kind: Service
metadata:
  creationTimestamp: "2025-08-02T08:10:42Z"
  name: test
  resourceVersion: "22563951"
  uid: 60ae6c15-6ea3-49f6-9477-55b5916e67cc
spec:
  clusterIP: 10.0.0.1
  clusterIPs:
  - 10.0.0.1
  internalTrafficPolicy: Cluster
  ipFamilies:
  - IPv4
  ipFamilyPolicy: SingleStack
  ports:
  - port: 80
    protocol: TCP
    targetPort: 80
  selector:
    app: test
  sessionAffinity: None
  type: ClusterIP
status:
  loadBalancer: {}`,
			hasDiff: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var fileObj, clusterObj *unstructured.Unstructured
			for obj := range objectsFromYaml(strings.NewReader(tt.fileYaml)) {
				if obj == nil {
					t.Fatal("failed to decode YAML")
				}
				fileObj = obj
			}
			for obj := range objectsFromYaml(strings.NewReader(tt.clusterYaml)) {
				if obj == nil {
					t.Fatal("failed to decode YAML")
				}
				clusterObj = obj
			}

			clusterObj = applyFiltering(fileObj, clusterObj)
			diffFound, err := executeDiff(normalizeObject(fileObj), normalizeObject(clusterObj))
			if err != nil {
				t.Fatal("failed to execute diff: ", err)
			}
			if diffFound != tt.hasDiff {
				t.Errorf("expected diff: %v, got: %v", tt.hasDiff, diffFound)
			}
		})
	}
}
