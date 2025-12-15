/*
Copyright 2025.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package main

import (
	"context"
	"crypto/tls"
	"flag"
	"os"
	"path/filepath"
	"strings"
	"time"

	// Import all Kubernetes client auth plugins (e.g. Azure, GCP, OIDC, etc.)
	// to ensure that exec-entrypoint and run can make use of them.
	_ "k8s.io/client-go/plugin/pkg/client/auth"

	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/certwatcher"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	manager "sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/metrics/filters"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"
	"sigs.k8s.io/controller-runtime/pkg/webhook"

	"sync"

	wikiv1alpha1 "github.com/dasmlab/glooscap-operator/api/v1alpha1"
	"github.com/dasmlab/glooscap-operator/internal/controller"
	"github.com/dasmlab/glooscap-operator/internal/server"
	"github.com/dasmlab/glooscap-operator/pkg/catalog"
	"github.com/dasmlab/glooscap-operator/pkg/nanabush"
	"github.com/dasmlab/glooscap-operator/pkg/vllm"
	// +kubebuilder:scaffold:imports
)

var (
	scheme   = runtime.NewScheme()
	setupLog = ctrl.Log.WithName("setup")
)

func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))

	utilruntime.Must(wikiv1alpha1.AddToScheme(scheme))
	// +kubebuilder:scaffold:scheme
}

// nolint:gocyclo
func main() {
	var metricsAddr string
	var metricsCertPath, metricsCertName, metricsCertKey string
	var webhookCertPath, webhookCertName, webhookCertKey string
	var enableLeaderElection bool
	var probeAddr string
	var secureMetrics bool
	var enableHTTP2 bool
	var tlsOpts []func(*tls.Config)
	flag.StringVar(&metricsAddr, "metrics-bind-address", "0", "The address the metrics endpoint binds to. "+
		"Use :8443 for HTTPS or :8080 for HTTP, or leave as 0 to disable the metrics service.")
	flag.StringVar(&probeAddr, "health-probe-bind-address", ":8081", "The address the probe endpoint binds to.")
	flag.BoolVar(&enableLeaderElection, "leader-elect", false,
		"Enable leader election for controller manager. "+
			"Enabling this will ensure there is only one active controller manager.")
	flag.BoolVar(&secureMetrics, "metrics-secure", true,
		"If set, the metrics endpoint is served securely via HTTPS. Use --metrics-secure=false to use HTTP instead.")
	flag.StringVar(&webhookCertPath, "webhook-cert-path", "", "The directory that contains the webhook certificate.")
	flag.StringVar(&webhookCertName, "webhook-cert-name", "tls.crt", "The name of the webhook certificate file.")
	flag.StringVar(&webhookCertKey, "webhook-cert-key", "tls.key", "The name of the webhook key file.")
	flag.StringVar(&metricsCertPath, "metrics-cert-path", "",
		"The directory that contains the metrics server certificate.")
	flag.StringVar(&metricsCertName, "metrics-cert-name", "tls.crt", "The name of the metrics server certificate file.")
	flag.StringVar(&metricsCertKey, "metrics-cert-key", "tls.key", "The name of the metrics server key file.")
	flag.BoolVar(&enableHTTP2, "enable-http2", false,
		"If set, HTTP/2 will be enabled for the metrics and webhook servers")
	opts := zap.Options{
		Development: true,
	}
	opts.BindFlags(flag.CommandLine)
	flag.Parse()

	ctrl.SetLogger(zap.New(zap.UseFlagOptions(&opts)))

	// if the enable-http2 flag is false (the default), http/2 should be disabled
	// due to its vulnerabilities. More specifically, disabling http/2 will
	// prevent from being vulnerable to the HTTP/2 Stream Cancellation and
	// Rapid Reset CVEs. For more information see:
	// - https://github.com/advisories/GHSA-qppj-fm5r-hxr3
	// - https://github.com/advisories/GHSA-4374-p667-p6c8
	disableHTTP2 := func(c *tls.Config) {
		setupLog.Info("disabling http/2")
		c.NextProtos = []string{"http/1.1"}
	}

	if !enableHTTP2 {
		tlsOpts = append(tlsOpts, disableHTTP2)
	}

	// Create watchers for metrics and webhooks certificates
	var metricsCertWatcher, webhookCertWatcher *certwatcher.CertWatcher

	// Initial webhook TLS options
	webhookTLSOpts := tlsOpts

	if len(webhookCertPath) > 0 {
		setupLog.Info("Initializing webhook certificate watcher using provided certificates",
			"webhook-cert-path", webhookCertPath, "webhook-cert-name", webhookCertName, "webhook-cert-key", webhookCertKey)

		var err error
		webhookCertWatcher, err = certwatcher.New(
			filepath.Join(webhookCertPath, webhookCertName),
			filepath.Join(webhookCertPath, webhookCertKey),
		)
		if err != nil {
			setupLog.Error(err, "Failed to initialize webhook certificate watcher")
			os.Exit(1)
		}

		webhookTLSOpts = append(webhookTLSOpts, func(config *tls.Config) {
			config.GetCertificate = webhookCertWatcher.GetCertificate
		})
	}

	webhookServer := webhook.NewServer(webhook.Options{
		TLSOpts: webhookTLSOpts,
	})

	// Metrics endpoint is enabled in 'config/default/kustomization.yaml'. The Metrics options configure the server.
	// More info:
	// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.21.0/pkg/metrics/server
	// - https://book.kubebuilder.io/reference/metrics.html
	metricsServerOptions := metricsserver.Options{
		BindAddress:   metricsAddr,
		SecureServing: secureMetrics,
		TLSOpts:       tlsOpts,
	}

	if secureMetrics {
		// FilterProvider is used to protect the metrics endpoint with authn/authz.
		// These configurations ensure that only authorized users and service accounts
		// can access the metrics endpoint. The RBAC are configured in 'config/rbac/kustomization.yaml'. More info:
		// https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.21.0/pkg/metrics/filters#WithAuthenticationAndAuthorization
		metricsServerOptions.FilterProvider = filters.WithAuthenticationAndAuthorization
	}

	// If the certificate is not specified, controller-runtime will automatically
	// generate self-signed certificates for the metrics server. While convenient for development and testing,
	// this setup is not recommended for production.
	//
	// TODO(user): If you enable certManager, uncomment the following lines:
	// - [METRICS-WITH-CERTS] at config/default/kustomization.yaml to generate and use certificates
	// managed by cert-manager for the metrics server.
	// - [PROMETHEUS-WITH-CERTS] at config/prometheus/kustomization.yaml for TLS certification.
	if len(metricsCertPath) > 0 {
		setupLog.Info("Initializing metrics certificate watcher using provided certificates",
			"metrics-cert-path", metricsCertPath, "metrics-cert-name", metricsCertName, "metrics-cert-key", metricsCertKey)

		var err error
		metricsCertWatcher, err = certwatcher.New(
			filepath.Join(metricsCertPath, metricsCertName),
			filepath.Join(metricsCertPath, metricsCertKey),
		)
		if err != nil {
			setupLog.Error(err, "to initialize metrics certificate watcher", "error", err)
			os.Exit(1)
		}

		metricsServerOptions.TLSOpts = append(metricsServerOptions.TLSOpts, func(config *tls.Config) {
			config.GetCertificate = metricsCertWatcher.GetCertificate
		})
	}

	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Scheme:                 scheme,
		Metrics:                metricsServerOptions,
		WebhookServer:          webhookServer,
		HealthProbeBindAddress: probeAddr,
		LeaderElection:         enableLeaderElection,
		LeaderElectionID:       "26d4bd72.glooscap.dasmlab.org",
		// LeaderElectionReleaseOnCancel defines if the leader should step down voluntarily
		// when the Manager ends. This requires the binary to immediately end when the
		// Manager is stopped, otherwise, this setting is unsafe. Setting this significantly
		// speeds up voluntary leader transitions as the new leader don't have to wait
		// LeaseDuration time first.
		//
		// In the default scaffold provided, the program ends immediately after
		// the manager stops, so would be fine to enable this option. However,
		// if you are doing or is intended to do any operation such as perform cleanups
		// after the manager stops then its usage might be unsafe.
		// LeaderElectionReleaseOnCancel: true,
	})
	if err != nil {
		setupLog.Error(err, "unable to start manager")
		os.Exit(1)
	}

	eventRecorder := mgr.GetEventRecorderFor("glooscap-operator")

	catalogStore := catalog.NewStore()
	jobStore := catalog.NewJobStore()
	outlineFactory := controller.DefaultOutlineClientFactory{}

	tektonNamespace := os.Getenv("VLLM_JOB_NAMESPACE")
	if tektonNamespace == "" {
		tektonNamespace = "nanabush"
	}
	vllmImage := os.Getenv("VLLM_JOB_IMAGE")
	if vllmImage == "" {
		vllmImage = "quay.io/dasmlab/vllm-runner:latest"
	}
	vllmAPI := os.Getenv("VLLM_API_URL")
	if vllmAPI == "" {
		vllmAPI = "http://vllm.nanabush.svc:8000"
	}

	var dispatcher vllm.Dispatcher
	if os.Getenv("VLLM_MODE") == string(vllm.ModeInline) {
		dispatcher = &vllm.InlineDispatcher{}
	} else {
		dispatcher = &vllm.TektonJobDispatcher{
			Client:       mgr.GetClient(),
			Namespace:    tektonNamespace,
			Image:        vllmImage,
			APIServerURL: vllmAPI,
		}
	}

	if err := (&controller.WikiTargetReconciler{
		Client:        mgr.GetClient(),
		Scheme:        mgr.GetScheme(),
		Recorder:      eventRecorder,
		Catalogue:     catalogStore,
		OutlineClient: outlineFactory,
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "WikiTarget")
		os.Exit(1)
	}
	// Initialize translation service gRPC client if configured
	// Supports both Nanabush and Iskoces (they use the same gRPC proto interface)
	var nanabushClient *nanabush.Client
	var nanabushStatusCh chan struct{}
	var nanabushClientMu sync.Mutex // Protects nanabushClient during reconfiguration

	// Create config store for runtime configuration
	configStore := server.NewConfigStore()

	// Initialize from environment variables if set
	translationServiceAddr := os.Getenv("TRANSLATION_SERVICE_ADDR")
	if translationServiceAddr == "" {
		translationServiceAddr = os.Getenv("NANABUSH_GRPC_ADDR")
	}

	// Determine service type for logging (nanabush or iskoces)
	serviceType := os.Getenv("TRANSLATION_SERVICE_TYPE")
	if serviceType == "" {
		// Try to infer from address (contains "iskoces" or "nanabush")
		if translationServiceAddr != "" {
			if strings.Contains(strings.ToLower(translationServiceAddr), "iskoces") {
				serviceType = "iskoces"
			} else if strings.Contains(strings.ToLower(translationServiceAddr), "nanabush") {
				serviceType = "nanabush"
			} else {
				serviceType = "iskoces" // default to iskoces
			}
		} else {
			// Default to Iskoces if no address is set
			serviceType = "iskoces"
			translationServiceAddr = "iskoces-service.iskoces.svc:50051"
		}
	}

	// Helper function to create/update translation service client
	createTranslationServiceClient := func(addr string, svcType string, secure bool) (*nanabush.Client, error) {
		if addr == "" {
			return nil, nil
		}

		// Create channel for nanabush status updates to trigger SSE broadcasts
		if nanabushStatusCh == nil {
			nanabushStatusCh = make(chan struct{}, 10) // Buffered to avoid blocking
		}

		// Get pod namespace if available (OpenShift/Kubernetes)
		namespace := os.Getenv("POD_NAMESPACE")
		if namespace == "" {
			namespace = os.Getenv("WATCH_NAMESPACE")
		}

		// Get pod name if available
		podName := os.Getenv("POD_NAME")

		metadata := make(map[string]string)
		if podName != "" {
			metadata["pod_name"] = podName
		}

		// Create a variable to hold the client reference for the callback
		var clientRef *nanabush.Client

		client, err := nanabush.NewClient(nanabush.Config{
			Address:       addr,
			Secure:        secure,
			Timeout:       30 * time.Second,
			ClientName:    "glooscap",
			ClientVersion: os.Getenv("OPERATOR_VERSION"), // Could be set in deployment
			Namespace:     namespace,
			Metadata:      metadata,
			// Set callback to trigger SSE broadcast on status changes
			// Use a closure that captures the client reference
			OnStatusChange: func(status nanabush.Status) {
				// Ensure client is set before triggering broadcast
				// This prevents race conditions where status changes before client is stored
				nanabushClientMu.Lock()
				currentClient := nanabushClient
				nanabushClientMu.Unlock()
				
				// Only trigger if we have a valid client (either the one being created or the stored one)
				if clientRef != nil || currentClient != nil {
					select {
					case nanabushStatusCh <- struct{}{}:
					default:
						// Channel full, skip (non-blocking)
					}
				}
			},
		})
		if err != nil {
			return nil, err
		}

		// Set the client reference for the callback
		clientRef = client

		setupLog.Info("Translation service gRPC client initialized and registered",
			"service_type", svcType,
			"address", addr,
			"client_id", client.ClientID(),
			"namespace", namespace)

		return client, nil
	}

	// Getter function for current nanabush client (for reconciler)
	getNanabushClient := func() *nanabush.Client {
		nanabushClientMu.Lock()
		defer nanabushClientMu.Unlock()
		return nanabushClient
	}

	// Reconfiguration function for runtime updates
	// This runs asynchronously to avoid blocking the HTTP request
	reconfigureTranslationService := func(cfg server.TranslationServiceConfig) error {
		// Close existing client asynchronously (don't block)
		go func() {
			nanabushClientMu.Lock()
			oldClient := nanabushClient
			nanabushClient = nil // Clear immediately so getter returns nil
			nanabushClientMu.Unlock()

			if oldClient != nil {
				setupLog.Info("Closing old translation service client...")
				if err := oldClient.Close(); err != nil {
					setupLog.Error(err, "error closing old translation service client")
				}
				setupLog.Info("Old translation service client closed")
			}

			// If address is empty, just clear the client (already done above)
			if cfg.Address == "" {
				setupLog.Info("Translation service configuration cleared")
				return
			}

			// Create new client asynchronously
			setupLog.Info("Creating new translation service client...",
				"address", cfg.Address,
				"type", cfg.Type,
				"secure", cfg.Secure)

			client, err := createTranslationServiceClient(cfg.Address, cfg.Type, cfg.Secure)
			if err != nil {
				setupLog.Error(err, "failed to create translation service client",
					"address", cfg.Address,
					"type", cfg.Type)
				return
			}

			// Update client atomically
			nanabushClientMu.Lock()
			nanabushClient = client
			nanabushClientMu.Unlock()

			setupLog.Info("Translation service reconfigured successfully",
				"address", cfg.Address,
				"type", cfg.Type,
				"secure", cfg.Secure)
		}()

		// Return immediately - reconfiguration happens in background
		setupLog.Info("Translation service reconfiguration initiated (async)",
			"address", cfg.Address,
			"type", cfg.Type)
		return nil
	}

	// Initialize from environment if configured
	if translationServiceAddr != "" {
		secure := os.Getenv("TRANSLATION_SERVICE_SECURE") == "true" || os.Getenv("NANABUSH_SECURE") == "true"
		client, err := createTranslationServiceClient(translationServiceAddr, serviceType, secure)
		if err != nil {
			setupLog.Error(err, "failed to create translation service client", "service_type", serviceType)
		} else {
			nanabushClient = client
			// Store initial config
			configStore.SetTranslationServiceConfig(&server.TranslationServiceConfig{
				Address: translationServiceAddr,
				Type:    serviceType,
				Secure:  secure,
			})
		}
	} else {
		setupLog.Info("No translation service configured (TRANSLATION_SERVICE_ADDR or NANABUSH_GRPC_ADDR not set)")
	}

	if err := (&controller.TranslationJobReconciler{
		Client:            mgr.GetClient(),
		Scheme:            mgr.GetScheme(),
		Recorder:          eventRecorder,
		Dispatcher:        dispatcher,
		Jobs:              jobStore,
		Catalogue:         catalogStore,
		OutlineClient:     outlineFactory,
		Nanabush:          nanabushClient,    // Initial client (for backward compatibility)
		GetNanabushClient: getNanabushClient, // Getter function for runtime updates
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "TranslationJob")
		os.Exit(1)
	}
	// +kubebuilder:scaffold:builder

	if err := mgr.Add(manager.RunnableFunc(func(ctx context.Context) error {
		addr := os.Getenv("GLOOSCAP_API_ADDR")

		// Create a wrapper function that uses the current nanabushClient
		// This allows runtime reconfiguration
		reconfigureFn := func(cfg server.TranslationServiceConfig) error {
			return reconfigureTranslationService(cfg)
		}

		return server.Start(ctx, server.Options{
			Addr:                          addr,
			Catalogue:                     catalogStore,
			Jobs:                          jobStore,
			Client:                        mgr.GetClient(),
			Nanabush:                      nanabushClient, // Keep for backward compatibility
			GetNanabushClient:             getNanabushClient, // Use getter for runtime updates
			NanabushStatusCh:              nanabushStatusCh,
			ConfigStore:                   configStore,
			ReconfigureTranslationService: reconfigureFn,
			OutlineClientFactory:          outlineFactory,
		})
	})); err != nil {
		setupLog.Error(err, "unable to add API server runnable")
		os.Exit(1)
	}

	if metricsCertWatcher != nil {
		setupLog.Info("Adding metrics certificate watcher to manager")
		if err := mgr.Add(metricsCertWatcher); err != nil {
			setupLog.Error(err, "unable to add metrics certificate watcher to manager")
			os.Exit(1)
		}
	}

	if webhookCertWatcher != nil {
		setupLog.Info("Adding webhook certificate watcher to manager")
		if err := mgr.Add(webhookCertWatcher); err != nil {
			setupLog.Error(err, "unable to add webhook certificate watcher to manager")
			os.Exit(1)
		}
	}

	if err := mgr.AddHealthzCheck("healthz", healthz.Ping); err != nil {
		setupLog.Error(err, "unable to set up health check")
		os.Exit(1)
	}
	if err := mgr.AddReadyzCheck("readyz", healthz.Ping); err != nil {
		setupLog.Error(err, "unable to set up ready check")
		os.Exit(1)
	}

	setupLog.Info("starting manager")
	if err := mgr.Start(ctrl.SetupSignalHandler()); err != nil {
		setupLog.Error(err, "problem running manager")
		os.Exit(1)
	}
}
