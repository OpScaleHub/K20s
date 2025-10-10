/*
Copyright 2025.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUTHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package main

import (
	"bytes"
	"crypto/tls"
	"flag"
	"html/template"
	"net/http"
	"os"

	// Import all Kubernetes client auth plugins (e.g. Azure, GCP, OIDC, etc.)
	// to ensure that exec-entrypoint and run can make use of them.
	_ "k8s.io/client-go/plugin/pkg/client/auth"

	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	"sigs.k8s.io/controller-runtime/pkg/metrics/server"
	"sigs.k8s.io/controller-runtime/pkg/webhook"

	optimizerv1 "github.com/OpScaleHub/K20s/api/v1"
	"github.com/OpScaleHub/K20s/internal/controller"
	// +kubebuilder:scaffold:imports
)

var (
	scheme   = runtime.NewScheme()
	setupLog = ctrl.Log.WithName("setup")
)

func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))

	utilruntime.Must(optimizerv1.AddToScheme(scheme))
	// +kubebuilder:scaffold:scheme
}

func main() {
	var metricsAddr string
	var enableLeaderElection bool
	var probeAddr string
	var secureMetrics bool
	var enableHTTP2 bool
	flag.StringVar(&metricsAddr, "metrics-bind-address", ":8080", "The address the metric endpoint binds to.")
	flag.StringVar(&probeAddr, "health-probe-bind-address", ":8081", "The address the probe endpoint binds to.")
	flag.BoolVar(&enableLeaderElection, "leader-elect", false,
		"Enable leader election for controller manager. "+
			"Enabling this will ensure there is only one active controller manager.")
	flag.BoolVar(&secureMetrics, "metrics-secure", false,
		"If set the metrics endpoint is served securely")
	flag.BoolVar(&enableHTTP2, "enable-http2", false,
		"If set, HTTP/2 will be enabled for the metrics and webhook servers")
	opts := zap.Options{
		Development: true,
	}
	opts.BindFlags(flag.CommandLine)
	flag.Parse()

	ctrl.SetLogger(zap.New(zap.UseFlagOptions(&opts)))

	// if the enable-http2 flag is false (the default), http/2 should be disabled
	// due to its vulnerabilities. More info: https://github.com/kubernetes/kubernetes/issues/115413
	var tlsOpts []func(*tls.Config)
	if !enableHTTP2 {
		tlsOpts = append(tlsOpts, func(c *tls.Config) {
			c.NextProtos = []string{"http/1.1"}
		})
	}

	webs := webhook.NewServer(webhook.Options{
		Port:    9443,
		TLSOpts: tlsOpts,
	})

	// Create the status page handler. We will inject the client later to break a dependency cycle.
	statusHandler := &StatusPageHandler{}

	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Scheme: scheme,
		Metrics: server.Options{
			BindAddress:   metricsAddr,
			SecureServing: secureMetrics,
			TLSOpts:       tlsOpts,
			ExtraHandlers: map[string]http.Handler{
				"/status": statusHandler,
			},
		},
		WebhookServer:          webs,
		HealthProbeBindAddress: probeAddr,
		LeaderElection:         enableLeaderElection,
		LeaderElectionID:       "f8c0d2a.example.com",
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

	// Inject the client into the status handler now that the manager is created.
	statusHandler.Client = mgr.GetClient()
	setupLog.Info("status page handler registered", "path", "/status")

	if err = (&controller.ResourceOptimizerProfileReconciler{
		Client: mgr.GetClient(),
		Scheme: mgr.GetScheme(),
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "ResourceOptimizerProfile")
		os.Exit(1)
	}

	// +kubebuilder:scaffold:builder

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

// StatusPageHandler serves a simple HTML page with the status of all ResourceOptimizerProfiles.
type StatusPageHandler struct {
	Client client.Client
}

const statusPageTemplate = `
<!DOCTYPE html>
<html>
<head>
    <title>K20s Controller Status</title>
    <style>
        body { font-family: sans-serif; margin: 2em; }
        table { border-collapse: collapse; width: 100%; }
        th, td { border: 1px solid #ddd; padding: 8px; text-align: left; }
        th { background-color: #f2f2f2; }
		h1 { color: #333; }
    </style>
</head>
<body>
    <h1>K20s Controller Status</h1>
    <h2>Resource Optimizer Profiles</h2>
    <table>
        <tr>
            <th>Namespace</th>
            <th>Name</th>
            <th>Policy</th>
            <th>Last Action</th>
            <th>Observed CPU</th>
            <th>Recommendation</th>
        </tr>
        {{range .Items}}
        <tr>
            <td>{{.Namespace}}</td>
            <td>{{.Name}}</td>
            <td>{{.Spec.OptimizationPolicy}}</td>
            <td>{{if .Status.LastAction}}{{.Status.LastAction.Type}} @ {{.Status.LastAction.Timestamp.Format "2006-01-02 15:04:05"}}{{else}}None{{end}}</td>
            <td>{{if .Status.ObservedMetrics}}{{.Status.ObservedMetrics.cpu_usage}}%{{else}}N/A{{end}}</td>
            <td>{{if .Status.Recommendations}}{{range .Status.Recommendations}}{{.}}{{end}}{{else}}None{{end}}</td>
        </tr>
        {{end}}
    </table>
</body>
</html>
`

func (h *StatusPageHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	logger := ctrl.Log.WithName("status-handler")

	var profiles optimizerv1.ResourceOptimizerProfileList
	if err := h.Client.List(ctx, &profiles); err != nil {
		logger.Error(err, "failed to list ResourceOptimizerProfiles")
		http.Error(w, "Failed to list resources", http.StatusInternalServerError)
		return
	}

	tmpl, err := template.New("status").Parse(statusPageTemplate)
	if err != nil {
		logger.Error(err, "failed to parse HTML template")
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, profiles); err != nil {
		logger.Error(err, "failed to execute HTML template")
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/html")
	w.Write(buf.Bytes())
}
