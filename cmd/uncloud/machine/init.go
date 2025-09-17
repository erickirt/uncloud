package machine

import (
	"context"
	"fmt"
	"net/netip"

	"github.com/docker/compose/v2/pkg/progress"
	"github.com/psviderski/uncloud/cmd/uncloud/caddy"
	"github.com/psviderski/uncloud/cmd/uncloud/dns"
	"github.com/psviderski/uncloud/internal/cli"
	"github.com/psviderski/uncloud/internal/cli/config"
	"github.com/psviderski/uncloud/internal/machine/api/pb"
	"github.com/psviderski/uncloud/internal/machine/cluster"
	"github.com/psviderski/uncloud/pkg/api"
	"github.com/spf13/cobra"
)

type initOptions struct {
	dnsEndpoint string
	name        string
	network     string
	noCaddy     bool
	noDNS       bool
	noInstall   bool
	publicIP    string
	sshKey      string
	version     string
	context     string
}

func NewInitCommand() *cobra.Command {
	opts := initOptions{}
	cmd := &cobra.Command{
		Use:   "init [USER@HOST:PORT]",
		Short: "Initialise a new cluster with a remote machine as the first member.",
		Long: "Initialise a new cluster by setting up a remote machine as the first member.\n" +
			"This command creates a new context in your Uncloud config to manage the cluster.",
		Example: `  # Initialise a new cluster with default settings.
  uc machine init root@<your-server-ip>

  # Initialise with a context name 'prod' in the Uncloud config (~/.config/uncloud/config.yaml) and machine name 'vps1'.
  uc machine init root@<your-server-ip> -c prod -n vps1

  # Initialise with a non-root user and custom SSH port and key.
  uc machine init ubuntu@<your-server-ip>:2222 -i ~/.ssh/mykey

  # Initialise without Caddy (no reverse proxy) and without an automatically managed domain name (xxxxxx.cluster.uncloud.run).
  # You can deploy Caddy with 'uc caddy deploy' and reserve a domain with 'uc dns reserve' later.
  uc machine init root@<your-server-ip> --no-caddy --no-dns`,
		// TODO: support initialising a cluster on the local machine.
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			uncli := cmd.Context().Value("cli").(*cli.CLI)

			var remoteMachine *cli.RemoteMachine
			if len(args) > 0 {
				user, host, port, err := config.SSHDestination(args[0]).Parse()
				if err != nil {
					return fmt.Errorf("parse remote machine: %w", err)
				}
				remoteMachine = &cli.RemoteMachine{
					User:    user,
					Host:    host,
					Port:    port,
					KeyPath: opts.sshKey,
				}
			}

			return initCluster(cmd.Context(), uncli, remoteMachine, opts)
		},
	}
	cmd.Flags().StringVar(&opts.dnsEndpoint, "dns-endpoint", dns.DefaultUncloudDNSAPIEndpoint,
		"API endpoint for the Uncloud DNS service.")
	cmd.Flags().StringVarP(
		&opts.name, "name", "n", "",
		"Assign a name to the machine.",
	)
	cmd.Flags().StringVar(
		&opts.network, "network", cluster.DefaultNetwork.String(),
		"IPv4 network CIDR to use for machines and services.",
	)
	cmd.Flags().BoolVar(
		&opts.noCaddy, "no-caddy", false,
		"Don't deploy Caddy reverse proxy service to the machine. You can deploy it later with 'uc caddy deploy'.",
	)
	cmd.Flags().BoolVar(
		&opts.noDNS, "no-dns", false,
		"Don't reserve a cluster domain in Uncloud DNS. You can reserve it later with 'uc dns reserve'.",
	)
	cmd.Flags().BoolVar(
		&opts.noInstall, "no-install", false,
		"Skip installation of Docker, Uncloud daemon, and dependencies on the machine. "+
			"Assumes they're already installed and running.",
	)
	cmd.Flags().StringVar(
		&opts.publicIP, "public-ip", "auto",
		"Public IP address of the machine for ingress configuration. Use 'auto' for automatic detection, "+
			fmt.Sprintf("blank '' or '%s' to disable ingress on this machine, or specify an IP address.", PublicIPNone),
	)
	cmd.Flags().StringVarP(
		&opts.sshKey, "ssh-key", "i", "",
		fmt.Sprintf("Path to SSH private key for remote login (if not already added to SSH agent). (default %q)",
			cli.DefaultSSHKeyPath),
	)
	cmd.Flags().StringVar(
		&opts.version, "version", "latest",
		"Version of the Uncloud daemon to install on the machine.",
	)
	cmd.Flags().StringVarP(
		&opts.context, "context", "c", cli.DefaultContextName,
		"Name of the new context to be created in the Uncloud config to manage the cluster.",
	)

	return cmd
}

func initCluster(ctx context.Context, uncli *cli.CLI, remoteMachine *cli.RemoteMachine, opts initOptions) error {
	netPrefix, err := netip.ParsePrefix(opts.network)
	if err != nil {
		return fmt.Errorf("parse network CIDR: %w", err)
	}

	var publicIP *netip.Addr
	switch opts.publicIP {
	case "auto":
		publicIP = &netip.Addr{}
	case "", PublicIPNone:
		publicIP = nil
	default:
		ip, err := netip.ParseAddr(opts.publicIP)
		if err != nil {
			return fmt.Errorf("parse public IP: %w", err)
		}
		publicIP = &ip
	}
	client, err := uncli.InitCluster(ctx, cli.InitClusterOptions{
		Context:       opts.context,
		MachineName:   opts.name,
		Network:       netPrefix,
		PublicIP:      publicIP,
		RemoteMachine: remoteMachine,
		SkipInstall:   opts.noInstall,
		Version:       opts.version,
	})
	if err != nil {
		return err
	}
	defer client.Close()

	if opts.noCaddy && opts.noDNS {
		return nil
	}

	// Deploy the Caddy service to the initialised machine.
	// The creation of a deployment plan talks to cluster API. Since the API needs a few moments to become available
	// after cluster initialisation, we keep the user informed during this wait.
	fmt.Println("Waiting for the machine to be ready...")
	fmt.Println()

	if !opts.noDNS {
		domain, err := client.ReserveDomain(ctx, &pb.ReserveDomainRequest{Endpoint: opts.dnsEndpoint})
		if err != nil {
			return fmt.Errorf("reserve cluster domain in Uncloud DNS: %w", err)
		}
		fmt.Printf("Reserved cluster domain: %s\n", domain.Name)
	}

	if !opts.noCaddy {
		d, err := client.NewCaddyDeployment("", "", api.Placement{})
		if err != nil {
			return fmt.Errorf("create caddy deployment: %w", err)
		}

		err = progress.RunWithTitle(ctx, func(ctx context.Context) error {
			if _, err = d.Run(ctx); err != nil {
				return fmt.Errorf("deploy caddy: %w", err)
			}
			return nil
		}, uncli.ProgressOut(), fmt.Sprintf("Deploying service %s", d.Spec.Name))
		if err != nil {
			return err
		}

		fmt.Println()
		return caddy.UpdateDomainRecords(ctx, client, uncli.ProgressOut())
	}

	return nil
}
