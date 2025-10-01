package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/metadata"
	metadatainformer "k8s.io/client-go/metadata/metadatainformer"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/clientcmd"
)

func main() {
	fmt.Println("Starting Kubernetes Informer Example...")
	var kubeconfig *string
	if home := os.Getenv("HOME"); home != "" {
		kubeconfig = flag.String("kubeconfig", filepath.Join(home, ".kube", "config"), "(optional) absolute path to the kubeconfig file")
	} else {
		kubeconfig = flag.String("kubeconfig", "", "absolute path to the kubeconfig file")
	}
	contextFlag := flag.String("context", "", "(optional) kubeconfig context to use")
	flag.Parse()

	// Build config from kubeconfig with optional context
	loadingRules := &clientcmd.ClientConfigLoadingRules{ExplicitPath: *kubeconfig}
	configOverrides := &clientcmd.ConfigOverrides{}
	if *contextFlag != "" {
		configOverrides.CurrentContext = *contextFlag
	}

	config, err := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(
		loadingRules,
		configOverrides,
	).ClientConfig()
	if err != nil {
		fmt.Printf("Error building kubeconfig: %v\n", err)
		os.Exit(1)
	}

	// Create metadata-only client (much lighter weight!)
	metadataClient, err := metadata.NewForConfig(config)
	if err != nil {
		fmt.Printf("Error creating metadata client: %v\n", err)
		os.Exit(1)
	}

	// Create metadata informer factory
	factory := metadatainformer.NewSharedInformerFactory(metadataClient, 30*time.Second)

	// Define pod resource
	podGVR := schema.GroupVersionResource{
		Group:    "",
		Version:  "v1",
		Resource: "pods",
	}

	// Create metadata informer for pods
	informer := factory.ForResource(podGVR).Informer()
	lister := factory.ForResource(podGVR).Lister()

	// Add event handler for real-time updates
	informer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			meta := obj.(*metav1.PartialObjectMetadata)
			fmt.Printf("Pod ADDED: %s/%s\n", meta.Namespace, meta.Name)
		},
		UpdateFunc: func(oldObj, newObj interface{}) {
			meta := newObj.(*metav1.PartialObjectMetadata)
			fmt.Printf("Pod UPDATED: %s/%s\n", meta.Namespace, meta.Name)
		},
		DeleteFunc: func(obj interface{}) {
			meta := obj.(*metav1.PartialObjectMetadata)
			fmt.Printf("Pod DELETED: %s/%s\n", meta.Namespace, meta.Name)
		},
	})

	// Start informer immediately
	ctx := context.Background()
	factory.Start(ctx.Done())

	// Wait for cache to sync
	fmt.Println("Loading cache...")
	startTime := time.Now()
	if !cache.WaitForCacheSync(ctx.Done(), informer.HasSynced) {
		fmt.Println("Failed to sync cache")
		os.Exit(1)
	}
	syncDuration := time.Since(startTime)
	fmt.Printf("✓ Cache synced in %v\n", syncDuration)

	// Query from cache
	start := time.Now()
	items, err := lister.List(labels.Everything())
	elapsed := time.Since(start)

	if err != nil {
		fmt.Printf("Error listing pods: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("\nListed %d pods from cache in %v\n", len(items), elapsed)
	fmt.Println("\nSample pods (metadata only):")
	for i, item := range items {
		if i >= 5 {
			break
		}
		meta := item.(*metav1.PartialObjectMetadata)
		fmt.Printf("  - %s/%s\n", meta.Namespace, meta.Name)
		fmt.Printf("    Created: %v\n", meta.CreationTimestamp)
	}

	// Keep running to observe updates
	fmt.Println("\nWatching for pod changes... (Ctrl+C to exit)")
	<-ctx.Done()
}
