# uc caddy config

Show the current Caddy configuration (Caddyfile).

## Synopsis

Display the current Caddy configuration (Caddyfile) from the connected machine or a specified one.

```
uc caddy config [flags]
```

## Options

```
  -c, --context string   Name of the cluster context. (default is the current context)
  -h, --help             help for config
  -m, --machine string   Name or ID of the machine to get the configuration from. (default is connected machine)
      --no-color         Disable syntax highlighting for the output.
```

## Options inherited from parent commands

```
      --connect string          Connect to a remote cluster machine without using the Uncloud configuration file. [$UNCLOUD_CONNECT]
                                Format: [ssh://]user@host[:port] or tcp://host:port
      --uncloud-config string   Path to the Uncloud configuration file. [$UNCLOUD_CONFIG] (default "~/.config/uncloud/config.yaml")
```

## See also

* [uc caddy](uc_caddy.md)	 - Manage Caddy reverse proxy service.

