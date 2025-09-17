# uc machine update

Update machine configuration in the cluster.

## Synopsis

Update machine configuration in the cluster.

This command allows setting various machine properties including:
- Machine name (--name)
- Public IP address (--public-ip)

At least one flag must be specified to perform an update operation.

```
uc machine update [flags]
```

## Options

```
  -c, --context string     Name of the cluster context. (default is the current context)
  -h, --help               help for update
      --name string        New name for the machine
      --public-ip string   Public IP address of the machine for ingress configuration. Use 'none' or '' to remove the public IP.
```

## Options inherited from parent commands

```
      --connect string          Connect to a remote cluster machine without using the Uncloud configuration file. [$UNCLOUD_CONNECT]
                                Format: [ssh://]user@host[:port] or tcp://host:port
      --uncloud-config string   Path to the Uncloud configuration file. [$UNCLOUD_CONFIG] (default "~/.config/uncloud/config.yaml")
```

## See also

* [uc machine](uc_machine.md)	 - Manage machines in an Uncloud cluster.

