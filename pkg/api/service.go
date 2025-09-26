package api

import (
	"encoding/json"
	"fmt"
	"maps"
	"regexp"
	"slices"
	"strings"

	"github.com/distribution/reference"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/psviderski/uncloud/internal/machine/api/pb"
)

const (
	ServiceModeReplicated = "replicated"
	ServiceModeGlobal     = "global"

	// PullPolicyAlways means the image is always pulled from the registry.
	PullPolicyAlways = "always"
	// PullPolicyMissing means the image is pulled from the registry only if it's not available on the machine where
	// a container is started. This is the default pull policy.
	// TODO: make each machine aware of the images on other machines and it possible to pull from them.
	// 	Pull from the registry only if the image is missing on all machines.
	PullPolicyMissing = "missing"
	// PullPolicyNever means the image is never pulled from the registry. A service with this pull policy can only be
	// deployed to machines where the image is already available.
	// TODO: see the TODO above for PullPolicyMissing. Pull from other machines in the cluster if available.
	PullPolicyNever = "never"
)

var (
	serviceIDRegexp = regexp.MustCompile("^[0-9a-f]{32}$")
	dnsLabelRegexp  = regexp.MustCompile(`^[a-z0-9]([-a-z0-9]*[a-z0-9])?$`)
)

func ValidateServiceID(id string) bool {
	return serviceIDRegexp.MatchString(id)
}

// ServiceSpec defines the desired state of a service.
// ATTENTION: after changing this struct, verify if deploy.EvalContainerSpecChange needs to be updated.
type ServiceSpec struct {
	// Caddy is the optional Caddy reverse proxy configuration for the service.
	// Caddy and Ports cannot be specified simultaneously.
	Caddy *CaddySpec `json:",omitempty"`
	// Container defines the desired state of each container in the service.
	Container ContainerSpec
	// Mode is the replication mode of the service. Default is ServiceModeReplicated if empty.
	Mode string
	Name string
	// Placement defines the placement constraints for the service.
	Placement Placement
	// Ports defines what service ports to publish to make the service accessible outside the cluster.
	// Caddy and Ports cannot be specified simultaneously.
	Ports []PortSpec
	// Replicas is the number of containers to run for the service. Only valid for a replicated service.
	Replicas uint `json:",omitempty"`
	// Volumes is list of data volumes that can be mounted into the container.
	Volumes []VolumeSpec
	// Configs is list of configuration objects that can be mounted into the container.
	Configs []ConfigSpec
}

// CaddyConfig returns the Caddy reverse proxy configuration for the service or an empty string if it's not defined.
func (s *ServiceSpec) CaddyConfig() string {
	if s.Caddy == nil {
		return ""
	}
	return strings.TrimSpace(s.Caddy.Config)
}

func (s *ServiceSpec) Volume(name string) (VolumeSpec, bool) {
	for _, v := range s.Volumes {
		if v.Name == name {
			return v, true
		}
	}
	return VolumeSpec{}, false
}

func (s *ServiceSpec) Config(name string) (ConfigSpec, bool) {
	for _, c := range s.Configs {
		if c.Name == name {
			return c, true
		}
	}
	return ConfigSpec{}, false
}

// MountedDockerVolumes returns the list of volumes of VolumeTypeVolume type that are mounted into the container.
func (s *ServiceSpec) MountedDockerVolumes() []VolumeSpec {
	volumes := make(map[string]VolumeSpec)
	for _, m := range s.Container.VolumeMounts {
		if v, ok := s.Volume(m.VolumeName); ok && v.Type == VolumeTypeVolume {
			volumes[v.Name] = v
		}
	}

	return slices.Collect(maps.Values(volumes))
}

func (s *ServiceSpec) SetDefaults() ServiceSpec {
	spec := s.Clone()

	if spec.Mode == "" {
		spec.Mode = ServiceModeReplicated
	}
	// Ensure the replicated service has at least one replica.
	if spec.Mode == ServiceModeReplicated && spec.Replicas == 0 {
		spec.Replicas = 1
	}
	spec.Container = spec.Container.SetDefaults()

	for i, v := range spec.Volumes {
		spec.Volumes[i] = v.SetDefaults()
	}

	return spec
}

func (s *ServiceSpec) Validate() error {
	if err := s.Container.Validate(); err != nil {
		return err
	}

	switch s.Mode {
	case "", ServiceModeGlobal, ServiceModeReplicated:
	default:
		return fmt.Errorf("invalid mode: %q", s.Mode)
	}

	if s.Name != "" {
		if len(s.Name) > 63 {
			return fmt.Errorf("service name too long (max 63 characters): %q", s.Name)
		}
		if !dnsLabelRegexp.MatchString(s.Name) {
			return fmt.Errorf("invalid service name: %q. must be 1-63 characters, lowercase letters, numbers, "+
				"and dashes only; must start and end with a letter or number", s.Name)
		}
	}

	for _, p := range s.Ports {
		if (p.Mode == "" || p.Mode == PortModeIngress) &&
			p.Protocol != ProtocolHTTP && p.Protocol != ProtocolHTTPS {
			return fmt.Errorf("unsupported protocol for ingress port %d: %s", p.ContainerPort, p.Protocol)
		}
	}

	// TODO: validate there is no conflict between ports.

	// Validate that Caddy and Ports are not used together, unless all ports are host mode.
	if s.Caddy != nil && strings.TrimSpace(s.Caddy.Config) != "" && len(s.Ports) > 0 {
		// Check if all ports are in host mode.
		hasIngressPort := false
		for _, p := range s.Ports {
			if p.Mode == "" || p.Mode == PortModeIngress {
				hasIngressPort = true
				break
			}
		}
		if hasIngressPort {
			return fmt.Errorf("ingress ports and Caddy configuration cannot be specified simultaneously: " +
				"Caddy config is auto-generated from ingress ports, use only one of them. " +
				"Host mode ports can be used with Caddy config")
		}
	}

	// Validate volumes
	volumeNames := make(map[string]struct{})
	for _, v := range s.Volumes {
		if err := v.Validate(); err != nil {
			return fmt.Errorf("invalid volume: %w", err)
		}
		if _, ok := volumeNames[v.Name]; ok {
			return fmt.Errorf("duplicate volume name: '%s'", v.Name)
		}
		volumeNames[v.Name] = struct{}{}
	}

	for _, m := range s.Container.VolumeMounts {
		if !slices.ContainsFunc(s.Volumes, func(v VolumeSpec) bool {
			return v.Name == m.VolumeName
		}) {
			return fmt.Errorf("volume mount references a volume that doesn't exist in the service spec: '%s'",
				m.VolumeName)
		}
	}

	// Validate configs
	if err := ValidateConfigsAndMounts(s.Configs, s.Container.ConfigMounts); err != nil {
		return fmt.Errorf("validate service configs and mounts: %w", err)
	}

	return nil
}

func (s *ServiceSpec) Clone() ServiceSpec {
	spec := *s

	if s.Caddy != nil {
		caddyCopy := *s.Caddy
		spec.Caddy = &caddyCopy
	}
	spec.Container = s.Container.Clone()

	if s.Ports != nil {
		spec.Ports = make([]PortSpec, len(s.Ports))
		copy(spec.Ports, s.Ports)
	}

	if s.Volumes != nil {
		spec.Volumes = make([]VolumeSpec, len(s.Volumes))
		for i, v := range s.Volumes {
			spec.Volumes[i] = v.Clone()
		}
	}

	return spec
}

// ContainerSpec defines the desired state of a container in a service.
// ATTENTION: after changing this struct, verify if deploy.EvalContainerSpecChange needs to be updated.
type ContainerSpec struct {
	// Command overrides the default CMD of the image to be executed when running a container.
	Command []string
	// Entrypoint overrides the default ENTRYPOINT of the image.
	Entrypoint []string
	// Env defines the environment variables to set inside the container.
	Env   EnvVars
	Image string
	// Run a custom init inside the container. If nil, use the daemon's configured settings.
	Init *bool
	// LogDriver overrides the default logging driver for the container. Each Docker daemon can have its own default.
	LogDriver *LogDriver
	// Privileged gives extended privileges to the container. This is a security risk and should be used with caution.
	Privileged bool
	// PullPolicy determines when to pull the image from the registry or use the image already available in the cluster.
	// Default is PullPolicyMissing if empty.
	PullPolicy string
	// Resource allocation for the container.
	Resources ContainerResources
	// User overrides the default user of the image used to run the container. Format: user|UID[:group|GID].
	User string
	// VolumeMounts specifies how volumes are mounted into the container filesystem.
	// Each mount references a volume defined in ServiceSpec.Volumes.
	VolumeMounts []VolumeMount
	// ConfigMounts specifies how configs are mounted into the container filesystem.
	// Each mount references a config defined in ServiceSpec.Configs.
	ConfigMounts []ConfigMount
	// Volumes is list of data volumes to mount into the container.
	// TODO(lhf): delete all usage, has been replaced with []VolumeMounts.
	Volumes []string
}

// SetDefaults returns a copy of the container spec with default values set.
func (s *ContainerSpec) SetDefaults() ContainerSpec {
	spec := s.Clone()
	if spec.LogDriver == nil {
		spec.LogDriver = &LogDriver{
			Name:    "local",
			Options: map[string]string{},
		}
	}
	if spec.PullPolicy == "" {
		spec.PullPolicy = PullPolicyMissing
	}

	return spec
}

func (s *ContainerSpec) Validate() error {
	if _, err := reference.ParseDockerRef(s.Image); err != nil {
		return fmt.Errorf("invalid image '%s': %w", s.Image, err)
	}

	for _, m := range s.VolumeMounts {
		if err := m.Validate(); err != nil {
			return fmt.Errorf("invalid volume mount: %w", err)
		}
	}

	return nil
}

func (s *ContainerSpec) Equals(spec ContainerSpec) bool {
	orig := s.SetDefaults()
	spec = spec.SetDefaults()

	slices.Sort(orig.Volumes)
	slices.Sort(spec.Volumes)

	sortVolumeMounts(orig.VolumeMounts)
	sortVolumeMounts(spec.VolumeMounts)

	return cmp.Equal(orig, spec, cmpopts.EquateEmpty())
}

func (s *ContainerSpec) Clone() ContainerSpec {
	spec := *s

	if s.Command != nil {
		spec.Command = make([]string, len(s.Command))
		copy(spec.Command, s.Command)
	}
	if s.Entrypoint != nil {
		spec.Entrypoint = make([]string, len(s.Entrypoint))
		copy(spec.Entrypoint, s.Entrypoint)
	}
	if s.LogDriver != nil {
		logDriver := *s.LogDriver
		if s.LogDriver.Options != nil {
			logDriver.Options = maps.Clone(s.LogDriver.Options)
		}
		spec.LogDriver = &logDriver
	}
	if s.Volumes != nil {
		spec.Volumes = make([]string, len(s.Volumes))
		copy(spec.Volumes, s.Volumes)
	}
	if s.VolumeMounts != nil {
		spec.VolumeMounts = make([]VolumeMount, len(s.VolumeMounts))
		copy(spec.VolumeMounts, s.VolumeMounts)
	}

	return spec
}

type EnvVars map[string]string

// ToSlice converts the environment variables to a slice of strings in the format "key=value".
func (e EnvVars) ToSlice() []string {
	env := make([]string, 0, len(e))
	for k, v := range e {
		if k == "" {
			continue
		}
		env = append(env, fmt.Sprintf("%s=%s", k, v))
	}
	return env
}

type LogDriver struct {
	// Name of the logging driver to use.
	Name string
	// Options is the configuration options to pass to the logging driver.
	Options map[string]string
}

type RunServiceResponse struct {
	ID   string
	Name string
}

type Service struct {
	ID         string
	Name       string
	Mode       string
	Containers []MachineServiceContainer
}

type MachineServiceContainer struct {
	MachineID string
	Container ServiceContainer
}

// Images returns a sorted list of unique images used by the service containers.
func (s *Service) Images() []string {
	images := make(map[string]struct{})
	for _, ctr := range s.Containers {
		images[ctr.Container.Config.Image] = struct{}{}
	}
	return slices.Sorted(maps.Keys(images))
}

// Endpoints returns the exposed HTTP and HTTPS endpoints of the service.
func (s *Service) Endpoints() []string {
	endpoints := make(map[string]struct{})

	// Container specs may differ between containers in the same service, e.g. during a rolling update,
	// so we need to collect all unique endpoints.
	for _, ctr := range s.Containers {
		ports, err := ctr.Container.ServicePorts()
		if err != nil {
			continue
		}

		for _, port := range ports {
			protocol := ""
			switch port.Protocol {
			case ProtocolHTTP:
				protocol = "http"
			case ProtocolHTTPS:
				protocol = "https"
			default:
				continue
			}

			if port.Hostname == "" {
				// There shouldn't be http(s) ports without a hostname but just in case ignore them.
				continue
			}

			endpoint := fmt.Sprintf("%s://%s", protocol, port.Hostname)
			if port.PublishedPort != 0 {
				// For non-standard ports (80/443), include the port in the URL.
				if !(port.Protocol == ProtocolHTTP && port.PublishedPort == 80) &&
					!(port.Protocol == ProtocolHTTPS && port.PublishedPort == 443) {
					endpoint += fmt.Sprintf(":%d", port.PublishedPort)
				}
			}

			endpoint += fmt.Sprintf(" → :%d", port.ContainerPort)
			endpoints[endpoint] = struct{}{}
		}
	}

	return slices.Sorted(maps.Keys(endpoints))
}

func ServiceFromProto(s *pb.Service) (Service, error) {
	var err error
	containers := make([]MachineServiceContainer, len(s.Containers))
	for i, sc := range s.Containers {
		containers[i], err = machineContainerFromProto(sc)
		if err != nil {
			return Service{}, err
		}
	}

	return Service{
		ID:         s.Id,
		Name:       s.Name,
		Mode:       s.Mode,
		Containers: containers,
	}, nil
}

func machineContainerFromProto(sc *pb.Service_Container) (MachineServiceContainer, error) {
	var c Container
	if err := json.Unmarshal(sc.Container, &c); err != nil {
		return MachineServiceContainer{}, fmt.Errorf("unmarshal container: %w", err)
	}

	return MachineServiceContainer{
		MachineID: sc.MachineId,
		Container: ServiceContainer{Container: c},
	}, nil
}
