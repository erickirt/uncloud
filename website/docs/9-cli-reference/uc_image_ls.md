# uc image ls

List images on machines in the cluster.

## Synopsis

List images on machines in the cluster. By default, on all machines. Optionally filter by image name.

```
uc image ls [REPO:[TAG]] [flags]
```

## Examples

```
  # List all images on all machines.
  uc image ls

  # List images on specific machine.
  uc image ls -m machine1

  # List images on multiple machines.
  uc image ls -m machine1,machine2

  # List images filtered by name (with any tag) on all machines.
  uc image ls myapp

  # List images filtered by name pattern on specific machine.
  uc image ls "myapp:1.*" -m machine1
```

## Options

```
  -c, --context string    Name of the cluster context. (default is the current context)
  -h, --help              help for ls
  -m, --machine strings   Filter images by machine name or ID. Can be specified multiple times or as a comma-separated list. (default is include all machines)
```

## Options inherited from parent commands

```
      --connect string          Connect to a remote cluster machine without using the Uncloud configuration file. [$UNCLOUD_CONNECT]
                                Format: [ssh://]user@host[:port] or tcp://host:port
      --uncloud-config string   Path to the Uncloud configuration file. [$UNCLOUD_CONFIG] (default "~/.config/uncloud/config.yaml")
```

## See also

* [uc image](uc_image.md)	 - Manage images on machines in the cluster.

