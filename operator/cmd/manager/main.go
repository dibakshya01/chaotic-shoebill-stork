// Command manager runs the chaos.shoebill.dev controller. It wires the core Safety
// Governor + engine (the only door to injection) to the real in-cluster `netpol`
// detonator and the configured Prometheus/Alertmanager signal sources, then reconciles
// SecurityChaosRun resources.
//
// Promotion history, incident state and the blackout calendar start from safe, static
// defaults (playground runs work out of the box; stage/prod require promotion history,
// which is the intended safety posture). Replace them with real providers for production.
package main

import (
	"flag"
	"os"
	"time"

	"github.com/dibakshya01/chaotic-shoebill-stork/internal/detonator"
	"github.com/dibakshya01/chaotic-shoebill-stork/internal/engine"
	"github.com/dibakshya01/chaotic-shoebill-stork/internal/evidence"
	"github.com/dibakshya01/chaotic-shoebill-stork/internal/model"
	"github.com/dibakshya01/chaotic-shoebill-stork/internal/prober"
	"github.com/dibakshya01/chaotic-shoebill-stork/internal/safety"
	"github.com/dibakshya01/chaotic-shoebill-stork/internal/store"
	"github.com/dibakshya01/chaotic-shoebill-stork/internal/verify"
	"github.com/dibakshya01/chaotic-shoebill-stork/internal/verify/sources"
	v1 "github.com/dibakshya01/chaotic-shoebill-stork/operator/api/v1alpha1"
	"github.com/dibakshya01/chaotic-shoebill-stork/operator/internal/controller"
	"github.com/dibakshya01/chaotic-shoebill-stork/operator/internal/controlplane"
	opdet "github.com/dibakshya01/chaotic-shoebill-stork/operator/internal/detonators"

	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/kubernetes"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
)

const keyID = "shoebill-operator"

var scheme = runtime.NewScheme()

func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	utilruntime.Must(v1.AddToScheme(scheme))
}

func main() {
	var metricsAddr, probeAddr, keyPath, promURL, amURL, evidenceDir string
	flag.StringVar(&metricsAddr, "metrics-bind-address", ":8080", "metrics endpoint")
	flag.StringVar(&probeAddr, "health-probe-bind-address", ":8081", "health probe endpoint")
	flag.StringVar(&keyPath, "signing-key", "/etc/shoebill/evidence.key", "Ed25519 signing key (created if absent)")
	flag.StringVar(&evidenceDir, "evidence-dir", "/var/lib/shoebill/evidence", "signed evidence ledger directory")
	flag.StringVar(&promURL, "prometheus-url", "", "Prometheus base URL (for DETECTED assertions)")
	flag.StringVar(&amURL, "alertmanager-url", "", "Alertmanager base URL (for ALERTED assertions)")
	flag.Parse()

	ctrl.SetLogger(zap.New(zap.UseDevMode(true)))
	log := ctrl.Log.WithName("setup")

	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Scheme:                 scheme,
		HealthProbeBindAddress: probeAddr,
	})
	if err != nil {
		log.Error(err, "unable to start manager")
		os.Exit(1)
	}

	clientset, err := kubernetes.NewForConfig(mgr.GetConfig())
	if err != nil {
		log.Error(err, "unable to build clientset")
		os.Exit(1)
	}

	// One Ed25519 key signs both governor capabilities and evidence receipts (v1).
	priv, err := evidence.LoadOrCreateKey(keyPath)
	if err != nil {
		log.Error(err, "unable to load signing key")
		os.Exit(1)
	}

	// Signal sources.
	reg := sources.Registry{model.SourceHTTP: sources.NewHTTP()}
	if promURL != "" {
		reg[model.SourcePrometheus] = sources.NewPrometheus(promURL)
	}
	if amURL != "" {
		reg[model.SourceAlertmanager] = sources.NewAlertmanager(amURL)
	}

	// Safety Governor with safe static defaults (replace providers for production).
	providers := safety.Providers{
		KillSwitch: safety.StaticKillSwitch{}, Incidents: safety.NoIncidents{},
		Blackout: safety.StaticBlackout{}, Promotions: &safety.MapPromotionHistory{},
		Concurrency: safety.FreeConcurrency{Max: 5}, FaultDomains: safety.NoFaultDomainClash{},
		SteadyState: prober.New(reg),
	}
	gov := safety.NewGovernor(providers, keyID, priv)

	// Detonators: the real in-cluster netpol + the safe core detonators.
	dets := detonator.Registry{
		"netpol":  opdet.NewNetworkPolicyDenyIngress(clientset, gov.PublicKey(), time.Now),
		"observe": detonator.NewObserve(gov.PublicKey(), time.Now),
		"http":    detonator.NewHTTP(gov.PublicKey(), time.Now),
	}

	ledger, err := evidence.OpenLedger(evidenceDir, evidence.NewSigner(keyID, priv))
	if err != nil {
		log.Error(err, "unable to open evidence ledger")
		os.Exit(1)
	}
	eng := engine.New(gov, dets, verify.NewEngine(reg, time.Now), ledger, store.New(), engine.Options{})

	if err := (&controller.SecurityChaosRunReconciler{
		Client:       mgr.GetClient(),
		Scheme:       mgr.GetScheme(),
		ControlPlane: controlplane.New(gov, eng),
	}).SetupWithManager(mgr); err != nil {
		log.Error(err, "unable to set up reconciler")
		os.Exit(1)
	}

	if err := mgr.AddHealthzCheck("healthz", healthz.Ping); err != nil {
		log.Error(err, "unable to add healthz check")
		os.Exit(1)
	}
	if err := mgr.AddReadyzCheck("readyz", healthz.Ping); err != nil {
		log.Error(err, "unable to add readyz check")
		os.Exit(1)
	}

	log.Info("starting chaos.shoebill.dev manager")
	if err := mgr.Start(ctrl.SetupSignalHandler()); err != nil {
		log.Error(err, "manager exited with error")
		os.Exit(1)
	}
}
