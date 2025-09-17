# uc volume

Manage volumes in an Uncloud cluster.

## Options

```
  -h, --help   help for volume
```

## Options inherited from parent commands

```
      --connect string          Connect to a remote cluster machine without using the Uncloud configuration file. [$UNCLOUD_CONNECT]
                                Format: [ssh://]user@host[:port] or tcp://host:port
      --uncloud-config string   Path to the Uncloud configuration file. [$UNCLOUD_CONFIG] (default "~/.config/uncloud/config.yaml")
```

## See also

* [uc](uc.md)	 - A CLI tool for managing Uncloud resources such as machines, services, and volumes.
* [uc volume create](uc_volume_create.md)	 - Create a volume on a specific machine.
* [uc volume inspect](uc_volume_inspect.md)	 - Display detailed information on a volume.
* [uc volume ls](uc_volume_ls.md)	 - List volumes across all machines in the cluster.
* [uc volume rm](uc_volume_rm.md)	 - Remove one or more volumes.

