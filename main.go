/*
Copyright 2023.

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
	"strings"

	// Import all Kubernetes client auth plugins (e.g. Azure, GCP, OIDC, etc.)
	// to ensure that exec-entrypoint and run can make use of them.
	_ "k8s.io/client-go/plugin/pkg/client/auth"

	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	"sigs.k8s.io/controller-runtime/pkg/webhook"

	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"

	rabbitmqclusterv2 "github.com/rabbitmq/cluster-operator/v2/api/v1beta1"
	"k8s.io/client-go/kubernetes"
	"sigs.k8s.io/controller-runtime/pkg/client/config"

	k8s_networkv1 "github.com/k8snetworkplumbingwg/network-attachment-definition-client/pkg/apis/k8s.cni.cncf.io/v1"
	frrk8sv1 "github.com/metallb/frr-k8s/api/v1beta1"
	ocp_configv1 "github.com/openshift/api/config/v1"
	instancehav1 "github.com/openstack-k8s-operators/infra-operator/apis/instanceha/v1beta1"
	memcachedv1 "github.com/openstack-k8s-operators/infra-operator/apis/memcached/v1beta1"
	networkv1 "github.com/openstack-k8s-operators/infra-operator/apis/network/v1beta1"
	rabbitmqv1beta1 "github.com/openstack-k8s-operators/infra-operator/apis/rabbitmq/v1beta1"
	redisv1 "github.com/openstack-k8s-operators/infra-operator/apis/redis/v1beta1"
	topologyv1beta1 "github.com/openstack-k8s-operators/infra-operator/apis/topology/v1beta1"
	instancehacontrollers "github.com/openstack-k8s-operators/infra-operator/controllers/instanceha"
	memcachedcontrollers "github.com/openstack-k8s-operators/infra-operator/controllers/memcached"
	networkcontrollers "github.com/openstack-k8s-operators/infra-operator/controllers/network"
	rabbitmqcontrollers "github.com/openstack-k8s-operators/infra-operator/controllers/rabbitmq"
	rediscontrollers "github.com/openstack-k8s-operators/infra-operator/controllers/redis"
	"github.com/openstack-k8s-operators/lib-common/modules/common/operator"
	//+kubebuilder:scaffold:imports
)

var (
	scheme   = runtime.NewScheme()
	setupLog = ctrl.Log.WithName("setup")
)

func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))

	utilruntime.Must(rabbitmqv1beta1.AddToScheme(scheme))
	utilruntime.Must(rabbitmqclusterv2.AddToScheme(scheme))
	utilruntime.Must(memcachedv1.AddToScheme(scheme))
	utilruntime.Must(instancehav1.AddToScheme(scheme))
	utilruntime.Must(redisv1.AddToScheme(scheme))
	utilruntime.Must(networkv1.AddToScheme(scheme))
	utilruntime.Must(frrk8sv1.AddToScheme(scheme))
	utilruntime.Must(k8s_networkv1.AddToScheme(scheme))
	utilruntime.Must(topologyv1beta1.AddToScheme(scheme))
	utilruntime.Must(ocp_configv1.AddToScheme(scheme))
	//+kubebuilder:scaffold:scheme
}

func main() {
	var metricsAddr string
	var enableLeaderElection bool
	var probeAddr string
	var pprofBindAddress string
	var enableHTTP2 bool
	flag.BoolVar(&enableHTTP2, "enable-http2", enableHTTP2, "If HTTP/2 should be enabled for the metrics and webhook servers.")
	flag.StringVar(&metricsAddr, "metrics-bind-address", ":8080", "The address the metric endpoint binds to.")
	flag.StringVar(&probeAddr, "health-probe-bind-address", ":8081", "The address the probe endpoint binds to.")
	flag.StringVar(&pprofBindAddress, "pprof-bind-address", "", "The address the pprof endpoint binds to. Set to empty to disable pprof.")
	flag.BoolVar(&enableLeaderElection, "leader-elect", false,
		"Enable leader election for controller manager. "+
			"Enabling this will ensure there is only one active controller manager.")
	opts := zap.Options{
		Development: true,
	}
	opts.BindFlags(flag.CommandLine)
	flag.Parse()

	ctrl.SetLogger(zap.New(zap.UseFlagOptions(&opts)))

	disableHTTP2 := func(c *tls.Config) {
		if enableHTTP2 {
			return
		}
		c.NextProtos = []string{"http/1.1"}
	}

	options := ctrl.Options{
		Scheme: scheme,
		Metrics: metricsserver.Options{
			BindAddress: metricsAddr,
		},
		HealthProbeBindAddress: probeAddr,
		LeaderElection:         enableLeaderElection,
		LeaderElectionID:       "c8c223a1.openstack.org",
		PprofBindAddress:       pprofBindAddress,
		WebhookServer: webhook.NewServer(
			webhook.Options{
				Port:    9443,
				TLSOpts: []func(config *tls.Config){disableHTTP2},
			}),
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
	}

	err := operator.SetManagerOptions(&options, setupLog)
	if err != nil {
		setupLog.Error(err, "unable to set manager options")
		os.Exit(1)
	}

	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), options)
	if err != nil {
		setupLog.Error(err, "unable to start manager")
		os.Exit(1)
	}

	cfg, err := config.GetConfig()
	if err != nil {
		setupLog.Error(err, "")
		os.Exit(1)
	}
	kclient, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		setupLog.Error(err, "")
		os.Exit(1)
	}

	if err = (&rabbitmqcontrollers.TransportURLReconciler{
		Client:  mgr.GetClient(),
		Scheme:  mgr.GetScheme(),
		Kclient: kclient,
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "TransportURL")
		os.Exit(1)
	}
	if err = (&memcachedcontrollers.Reconciler{
		Client:  mgr.GetClient(),
		Kclient: kclient,
		Scheme:  mgr.GetScheme(),
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "Memcached")
		os.Exit(1)
	}
	if err = (&instancehacontrollers.Reconciler{
		Client:  mgr.GetClient(),
		Kclient: kclient,
		Scheme:  mgr.GetScheme(),
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "InstanceHa")
		os.Exit(1)
	}
	if err = (&rediscontrollers.Reconciler{
		Client:  mgr.GetClient(),
		Kclient: kclient,
		Scheme:  mgr.GetScheme(),
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "Redis")
		os.Exit(1)
	}
	if err = (&networkcontrollers.DNSMasqReconciler{
		Client:  mgr.GetClient(),
		Kclient: kclient,
		Scheme:  mgr.GetScheme(),
	}).SetupWithManager(context.Background(), mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "DNSMasq")
		os.Exit(1)
	}

	if err = (&networkcontrollers.DNSDataReconciler{
		Client:  mgr.GetClient(),
		Kclient: kclient,
		Scheme:  mgr.GetScheme(),
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "DNSData")
		os.Exit(1)
	}

	if err = (&networkcontrollers.ServiceReconciler{
		Client:  mgr.GetClient(),
		Kclient: kclient,
		Scheme:  mgr.GetScheme(),
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "Service")
		os.Exit(1)
	}
	if err = (&networkcontrollers.IPSetReconciler{
		Client:  mgr.GetClient(),
		Kclient: kclient,
		Scheme:  mgr.GetScheme(),
	}).SetupWithManager(context.Background(), mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "IPSet")
		os.Exit(1)
	}

	if err = (&networkcontrollers.BGPConfigurationReconciler{
		Client:  mgr.GetClient(),
		Kclient: kclient,
		Scheme:  mgr.GetScheme(),
	}).SetupWithManager(context.Background(), mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "BGPConfiguration")
		os.Exit(1)
	}

	if err = (&rabbitmqcontrollers.Reconciler{
		Client:  mgr.GetClient(),
		Kclient: kclient,
		Scheme:  mgr.GetScheme(),
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "RabbitMq")
		os.Exit(1)
	}

	// Acquire environmental defaults and initialize operator defaults with them
	memcachedv1.SetupDefaults()
	redisv1.SetupDefaults()
	networkv1.SetupDefaults()

	// Setup webhooks if requested
	checker := healthz.Ping
	if strings.ToLower(os.Getenv("ENABLE_WEBHOOKS")) != "false" {

		if err = (&memcachedv1.Memcached{}).SetupWebhookWithManager(mgr); err != nil {
			setupLog.Error(err, "unable to create webhook", "webhook", "Memcached")
			os.Exit(1)
		}
		if err = (&instancehav1.InstanceHa{}).SetupWebhookWithManager(mgr); err != nil {
			setupLog.Error(err, "unable to create webhook", "webhook", "InstanceHa")
			os.Exit(1)
		}
		if err = (&redisv1.Redis{}).SetupWebhookWithManager(mgr); err != nil {
			setupLog.Error(err, "unable to create webhook", "webhook", "Redis")
			os.Exit(1)
		}
		if err = (&networkv1.DNSMasq{}).SetupWebhookWithManager(mgr); err != nil {
			setupLog.Error(err, "unable to create webhook", "webhook", "DNSMasq")
			os.Exit(1)
		}
		if err = (&networkv1.NetConfig{}).SetupWebhookWithManager(mgr); err != nil {
			setupLog.Error(err, "unable to create webhook", "webhook", "NetConfig")
			os.Exit(1)
		}
		if err = (&networkv1.Reservation{}).SetupWebhookWithManager(mgr); err != nil {
			setupLog.Error(err, "unable to create webhook", "webhook", "Reservation")
			os.Exit(1)
		}
		if err = (&networkv1.IPSet{}).SetupWebhookWithManager(mgr); err != nil {
			setupLog.Error(err, "unable to create webhook", "webhook", "IPSet")
			os.Exit(1)
		}
		if err = (&rabbitmqv1beta1.RabbitMq{}).SetupWebhookWithManager(mgr); err != nil {
			setupLog.Error(err, "unable to create webhook", "webhook", "RabbitMq")
			os.Exit(1)
		}
		checker = mgr.GetWebhookServer().StartedChecker()
	}

	//+kubebuilder:scaffold:builder

	if err := mgr.AddHealthzCheck("healthz", checker); err != nil {
		setupLog.Error(err, "unable to set up health check")
		os.Exit(1)
	}
	if err := mgr.AddReadyzCheck("readyz", checker); err != nil {
		setupLog.Error(err, "unable to set up ready check")
		os.Exit(1)
	}

	setupLog.Info("starting manager")
	if err := mgr.Start(ctrl.SetupSignalHandler()); err != nil {
		setupLog.Error(err, "problem running manager")
		os.Exit(1)
	}
}
