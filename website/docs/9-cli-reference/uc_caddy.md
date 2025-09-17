# uc caddy

Manage Caddy reverse proxy service.

## Options

```
  -h, --help   help for caddy
```

## Options inherited from parent commands

```
      --connect string          Connect to a remote cluster machine without using the Uncloud configuration file. [$UNCLOUD_CONNECT]
                                Format: [ssh://]user@host[:port] or tcp://host:port
      --uncloud-config string   Path to the Uncloud configuration file. [$UNCLOUD_CONFIG] (default "~/.config/uncloud/config.yaml")
```

## See also

* [uc](uc.md)	 - A CLI tool for managing Uncloud resources such as machines, services, and volumes.
* [uc caddy config](uc_caddy_config.md)	 - Show the current Caddy configuration (Caddyfile).
* [uc caddy deploy](uc_caddy_deploy.md)	 - Deploy or upgrade Caddy reverse proxy across all machines in the cluster.

