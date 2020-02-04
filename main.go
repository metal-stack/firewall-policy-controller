package main

import (
	"fmt"
	"os/exec"
	"time"

	k8s "k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"

	"io/ioutil"
	"os"

	controller "git.f-i-ts.de/cloud-native/firewall-policy-controller/pkg/controller"
	"git.f-i-ts.de/cloud-native/firewall-policy-controller/pkg/droptailer"
	"git.f-i-ts.de/cloud-native/firewall-policy-controller/pkg/watcher"
	"git.f-i-ts.de/cloud-native/metallib/version"
	"git.f-i-ts.de/cloud-native/metallib/zapup"

	"github.com/mitchellh/go-homedir"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

const (
	moduleName      = "firewall-policy-controller"
	nftFile         = "/etc/nftables/firewall-policy-controller.v4"
	nftBin          = "/usr/sbin/nft"
	nftablesService = "nftables.service"
	systemctlBin    = "/bin/systemctl"
)

var (
	logger = zapup.MustRootLogger().Sugar()
)

var rootCmd = &cobra.Command{
	Use:     moduleName,
	Short:   "a service that assembles and enforces firewall rules based on k8s resources",
	Version: version.V.String(),
	Run: func(cmd *cobra.Command, args []string) {
		run()
	},
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		logger.Error("failed executing root command", "error", err)
	}
}

func init() {
	viper.SetEnvPrefix("firewall")
	homedir, err := homedir.Dir()
	if err != nil {
		logger.Fatal(err)
	}
	rootCmd.PersistentFlags().StringP("kubecfg", "k", homedir+"/.kube/config", "kubecfg path to the cluster to account")
	rootCmd.PersistentFlags().Bool("dry-run", false, "just print the rules that would be enforced without applying them")
	rootCmd.PersistentFlags().Duration("fetch-interval", 10*time.Second, "interval for reassembling firewall rules")
	viper.AutomaticEnv()
	err = viper.BindPFlags(rootCmd.PersistentFlags())
	if err != nil {
		logger.Fatal(err)
	}
}

func run() {
	client, err := loadClient(viper.GetString("kubecfg"))
	if err != nil {
		logger.Errorw("unable to connect to k8s", "error", err)
		os.Exit(1)
	}
	ctr := controller.NewFirewallController(client, logger)
	svcWatcher := watcher.NewServiceWatcher(logger, client)
	npWatcher := watcher.NewNetworkPolicyWatcher(logger, client)
	dropTailer, err := droptailer.NewDropTailer(logger, client)
	if err != nil {
		logger.Errorw("unable to create droptailer client", "error", err)
		os.Exit(1)
	}

	// watch for services and network policies
	c := make(chan bool)
	go svcWatcher.Watch(c)
	go npWatcher.Watch(c)
	go dropTailer.WatchServerIP()
	go dropTailer.WatchClientSecret()

	// regularly trigger fetch of k8s resources
	go func() {
		t := time.NewTicker(viper.GetDuration("fetch-interval"))
		for {
			<-t.C
			c <- true
		}
	}()

	// debounce events and handle fetch
	d := time.Second * 3
	t := time.NewTimer(d)
	var old *controller.FirewallRules
	var new *controller.FirewallRules
	for {
		select {
		case <-c:
			t.Reset(d)
		case <-t.C:
			new, err = ctr.FetchAndAssemble()
			if err != nil {
				logger.Errorw("could not fetch k8s entities to build firewall rules", "error", err)
			}
			if !new.HasChanged(old) {
				old = new
				continue
			}
			logger.Infow("new fw rules to enforce", "ingress", len(new.IngressRules), "egress", len(new.EgressRules))
			for k, i := range new.IngressRules {
				fmt.Printf("%d ingress: %s\n", k+1, i)
			}
			for k, e := range new.EgressRules {
				fmt.Printf("%d egress: %s\n", k+1, e)
			}
			if !viper.GetBool("dry-run") {
				rs, err := new.Render()
				if err != nil {
					logger.Errorw("error rendering nftables rules", "error", err)
					continue
				}
				err = ioutil.WriteFile(nftFile, []byte(rs), 0644)
				if err != nil {
					logger.Errorw("error writing nftables file", "file", nftFile, "error", err)
					continue
				}
				c := exec.Command(nftBin, "-c", "-f", nftFile)
				out, err := c.Output()
				if err != nil {
					logger.Errorw("nftables file is invalid", "file", nftFile, "error", fmt.Sprint(out))
					continue
				}
				c = exec.Command(systemctlBin, "reload", nftablesService)
				err = c.Run()
				if err != nil {
					logger.Errorw("nftables.service file could not be reloaded")
					continue
				}
				logger.Info("applied new set of nftable rules")
			}
		}
	}
}

func loadClient(kubeconfigPath string) (*k8s.Clientset, error) {
	config, err := clientcmd.BuildConfigFromFlags("", kubeconfigPath)
	if err != nil {
		return nil, err
	}
	return k8s.NewForConfig(config)
}
