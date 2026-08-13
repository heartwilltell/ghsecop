package main

import (
	"crypto/tls"
	"flag"
	"os"
	"time"

	// Import all Kubernetes client auth plugins (e.g. Azure, GCP, OIDC, ...).
	_ "k8s.io/client-go/plugin/pkg/client/auth"

	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"
	"sigs.k8s.io/controller-runtime/pkg/webhook"

	ghsecopv1 "github.com/heartwilltell/ghsecop/api/v1"
	"github.com/heartwilltell/ghsecop/internal/controller"
	opconnect "github.com/heartwilltell/ghsecop/internal/connect"
	ghsec "github.com/heartwilltell/ghsecop/internal/github"
)

var (
	scheme   = runtime.NewScheme()
	setupLog = ctrl.Log.WithName("setup")
)

func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	utilruntime.Must(ghsecopv1.AddToScheme(scheme))
}

func main() {
	var (
		metricsAddr          string
		probeAddr            string
		enableLeaderElection bool
		secureMetrics        bool
		pollingInterval      time.Duration
	)

	flag.StringVar(&metricsAddr, "metrics-bind-address", ":8080", "The address the metric endpoint binds to.")
	flag.StringVar(&probeAddr, "health-probe-bind-address", ":8081", "The address the probe endpoint binds to.")
	flag.BoolVar(&enableLeaderElection, "leader-elect", true, "Enable leader election for controller manager.")
	flag.BoolVar(&secureMetrics, "metrics-secure", false, "Serve metrics endpoint securely via HTTPS.")
	flag.DurationVar(&pollingInterval, "polling-interval", 10*time.Minute,
		"Default interval for re-checking 1Password Connect for item changes (overridable per CR).")
	opts := zap.Options{Development: true}
	opts.BindFlags(flag.CommandLine)
	flag.Parse()

	ctrl.SetLogger(zap.New(zap.UseFlagOptions(&opts)))

	connectHost := envOr("OP_CONNECT_HOST", "")
	connectToken := envOr("OP_CONNECT_TOKEN", "")
	githubToken := envOr("GITHUB_TOKEN", "")

	if connectHost == "" || connectToken == "" {
		setupLog.Error(nil, "OP_CONNECT_HOST and OP_CONNECT_TOKEN are required")
		os.Exit(1)
	}
	if githubToken == "" {
		setupLog.Error(nil, "GITHUB_TOKEN is required (classic PAT or fine-grained token with Actions secrets write + repo list)")
		os.Exit(1)
	}

	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Scheme: scheme,
		Metrics: metricsserver.Options{
			BindAddress:   metricsAddr,
			SecureServing: secureMetrics,
			TLSOpts: []func(*tls.Config){
				func(c *tls.Config) { c.MinVersion = tls.VersionTLS12 },
			},
		},
		WebhookServer: webhook.NewServer(webhook.Options{
			Port: 9443,
		}),
		HealthProbeBindAddress: probeAddr,
		LeaderElection:         enableLeaderElection,
		LeaderElectionID:       "ghsecop.io",
	})
	if err != nil {
		setupLog.Error(err, "unable to start manager")
		os.Exit(1)
	}

	connectClient := opconnect.New(connectHost, connectToken)
	githubClient := ghsec.NewClient(githubToken)

	if err = (&controller.GitHubSecretSyncReconciler{
		Client:              mgr.GetClient(),
		Scheme:              mgr.GetScheme(),
		Connect:             connectClient,
		GitHub:              githubClient,
		DefaultSyncInterval: pollingInterval,
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "GitHubSecretSync")
		os.Exit(1)
	}

	if err := mgr.AddHealthzCheck("healthz", healthz.Ping); err != nil {
		setupLog.Error(err, "unable to set up health check")
		os.Exit(1)
	}
	if err := mgr.AddReadyzCheck("readyz", healthz.Ping); err != nil {
		setupLog.Error(err, "unable to set up ready check")
		os.Exit(1)
	}

	setupLog.Info("starting ghsecop manager",
		"connectHost", connectHost,
		"pollingInterval", pollingInterval.String(),
	)
	if err := mgr.Start(ctrl.SetupSignalHandler()); err != nil {
		setupLog.Error(err, "problem running manager")
		os.Exit(1)
	}
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
