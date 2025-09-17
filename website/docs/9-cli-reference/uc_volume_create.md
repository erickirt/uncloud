# uc volume create

Create a volume on a specific machine.

```
uc volume create VOLUME_NAME [flags]
```

## Options

```
  -c, --context string   Name of the cluster context. (default is the current context)
  -d, --driver string    Volume driver to use. (default "local")
  -h, --help             help for create
  -l, --label strings    Labels to assign to the volume in the form of 'key=value' pairs. Can be specified multiple times.
  -m, --machine string   Name or ID of the machine to create the volume on.
  -o, --opt strings      Driver specific options in the form of 'key=value' pairs. Can be specified multiple times.
```

## Options inherited from parent commands

```
      --connect string          Connect to a remote cluster machine without using the Uncloud configuration file. [$UNCLOUD_CONNECT]
                                Format: [ssh://]user@host[:port] or tcp://host:port
      --uncloud-config string   Path to the Uncloud configuration file. [$UNCLOUD_CONFIG] (default "~/.config/uncloud/config.yaml")
```

## See also

* [uc volume](uc_volume.md)	 - Manage volumes in an Uncloud cluster.

