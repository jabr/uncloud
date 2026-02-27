package compose

import (
	"fmt"
	"slices"

	"github.com/compose-spec/compose-go/v2/types"
)

type Warning struct {
	Service string
	Key     string
	Message string
}

func (w Warning) String() string {
	if w.Service == "" {
		return w.Message
	}
	return fmt.Sprintf("service '%s': %s", w.Service, w.Message)
}

func CheckProjectWarnings(project *types.Project) []Warning {
	var warnings []Warning

	for _, name := range project.ServiceNames() {
		service := project.Services[name]
		warnings = append(warnings, checkServiceWarnings(project, name, service)...)
	}

	return warnings
}

// hasUserDefinedNetworks returns true if the service has explicitly user-defined networks
// (not just the default network created by compose-go).
func hasUserDefinedNetworks(project *types.Project, service types.ServiceConfig) bool {
	// The default network name is the project name.
	defaultNetworkName := project.Name

	// If service.Networks is empty, compose-go is using the default network only.
	if len(service.Networks) == 0 {
		return false
	}

	for networkName := range service.Networks {
		// If any network is not the default network, it's user-defined.
		// Also check for the full name format if there's a project prefix.
		if networkName != defaultNetworkName && networkName != "default" {
			return true
		}
	}

	return false
}

func checkServiceWarnings(project *types.Project, name string, service types.ServiceConfig) []Warning {
	var warnings []Warning

	if service.Build != nil {
		warnings = append(warnings, Warning{
			Service: name,
			Key:     "build",
			Message: "'build' is not supported. Pre-build images and specify 'image' instead.",
		})
	}

	if service.ContainerName != "" {
		warnings = append(warnings, Warning{
			Service: name,
			Key:     "container_name",
			Message: "'container_name' is not supported. Uncloud generates container names automatically.",
		})
	}

	// Only warn about depends_on if it wasn't auto-created by links.
	// When links is used, compose-go auto-populates depends_on, so we don't want
	// to double-warn.
	if len(service.DependsOn) > 0 && len(service.Links) == 0 {
		warnings = append(warnings, Warning{
			Service: name,
			Key:     "depends_on",
			Message: "'depends_on' is not supported. Services start independently.",
		})
	}

	// Only warn about networks if user has explicitly defined networks (not the default
	// network that compose-go creates automatically).
	if len(service.Networks) > 0 && hasUserDefinedNetworks(project, service) {
		warnings = append(warnings, Warning{
			Service: name,
			Key:     "networks",
			Message: "'networks' is not supported. Uncloud uses a flat mesh network with built-in service discovery.",
		})
	}

	if service.NetworkMode != "" && service.NetworkMode != "default" {
		warnings = append(warnings, Warning{
			Service: name,
			Key:     "network_mode",
			Message: "'network_mode' is not supported. Uncloud uses a flat mesh network.",
		})
	}

	if service.Restart != "" {
		warnings = append(warnings, Warning{
			Service: name,
			Key:     "restart",
			Message: "'restart' is not supported. Uncloud manages container lifecycle automatically.",
		})
	}

	if len(service.Secrets) > 0 {
		warnings = append(warnings, Warning{
			Service: name,
			Key:     "secrets",
			Message: "'secrets' is not supported. Use environment variables or configs instead.",
		})
	}

	if len(service.Profiles) > 0 {
		warnings = append(warnings, Warning{
			Service: name,
			Key:     "profiles",
			Message: "'profiles' is not supported. Specify services explicitly during deploy.",
		})
	}

	if len(service.Links) > 0 {
		warnings = append(warnings, Warning{
			Service: name,
			Key:     "links",
			Message: "'links' is deprecated and not supported. Use service names for discovery.",
		})
	}

	if len(service.ExternalLinks) > 0 {
		warnings = append(warnings, Warning{
			Service: name,
			Key:     "external_links",
			Message: "'external_links' is deprecated and not supported.",
		})
	}

	if len(service.VolumesFrom) > 0 {
		warnings = append(warnings, Warning{
			Service: name,
			Key:     "volumes_from",
			Message: "'volumes_from' is not supported. Define volumes explicitly.",
		})
	}

	if service.Develop != nil {
		warnings = append(warnings, Warning{
			Service: name,
			Key:     "develop",
			Message: "'develop' is not supported. This is a development-only feature.",
		})
	}

	if service.Hostname != "" {
		warnings = append(warnings, Warning{
			Service: name,
			Key:     "hostname",
			Message: "'hostname' is not supported. Use service name for DNS resolution.",
		})
	}

	if len(service.DNS) > 0 {
		warnings = append(warnings, Warning{
			Service: name,
			Key:     "dns",
			Message: "'dns' is not supported. Uncloud provides built-in DNS.",
		})
	}

	if len(service.DNSOpts) > 0 {
		warnings = append(warnings, Warning{
			Service: name,
			Key:     "dns_opt",
			Message: "'dns_opt' is not supported.",
		})
	}

	if len(service.DNSSearch) > 0 {
		warnings = append(warnings, Warning{
			Service: name,
			Key:     "dns_search",
			Message: "'dns_search' is not supported.",
		})
	}

	if len(service.ExtraHosts) > 0 {
		warnings = append(warnings, Warning{
			Service: name,
			Key:     "extra_hosts",
			Message: "'extra_hosts' is not supported.",
		})
	}

	if len(service.SecurityOpt) > 0 {
		warnings = append(warnings, Warning{
			Service: name,
			Key:     "security_opt",
			Message: "'security_opt' is not supported.",
		})
	}

	if service.Platform != "" {
		warnings = append(warnings, Warning{
			Service: name,
			Key:     "platform",
			Message: "'platform' is not supported.",
		})
	}

	if service.WorkingDir != "" {
		warnings = append(warnings, Warning{
			Service: name,
			Key:     "working_dir",
			Message: "'working_dir' is not supported.",
		})
	}

	if len(service.Tmpfs) > 0 {
		warnings = append(warnings, Warning{
			Service: name,
			Key:     "tmpfs",
			Message: "'tmpfs' at service level is not supported. Use volumes with tmpfs type instead.",
		})
	}

	if service.ReadOnly {
		warnings = append(warnings, Warning{
			Service: name,
			Key:     "read_only",
			Message: "'read_only' is not supported.",
		})
	}

	if service.ShmSize > 0 {
		warnings = append(warnings, Warning{
			Service: name,
			Key:     "shm_size",
			Message: "'shm_size' is not supported.",
		})
	}

	if service.CPUSet != "" {
		warnings = append(warnings, Warning{
			Service: name,
			Key:     "cpuset",
			Message: "'cpuset' is not supported.",
		})
	}

	if service.MemSwapLimit > 0 {
		warnings = append(warnings, Warning{
			Service: name,
			Key:     "memswap_limit",
			Message: "'memswap_limit' is not supported.",
		})
	}

	if service.Pid != "" {
		warnings = append(warnings, Warning{
			Service: name,
			Key:     "pid",
			Message: "'pid' is not supported.",
		})
	}

	if service.Ipc != "" {
		warnings = append(warnings, Warning{
			Service: name,
			Key:     "ipc",
			Message: "'ipc' is not supported.",
		})
	}

	if service.Uts != "" {
		warnings = append(warnings, Warning{
			Service: name,
			Key:     "uts",
			Message: "'uts' is not supported.",
		})
	}

	if service.UserNSMode != "" {
		warnings = append(warnings, Warning{
			Service: name,
			Key:     "userns_mode",
			Message: "'userns_mode' is not supported.",
		})
	}

	if service.CgroupParent != "" {
		warnings = append(warnings, Warning{
			Service: name,
			Key:     "cgroup_parent",
			Message: "'cgroup_parent' is not supported.",
		})
	}

	if service.Cgroup != "" {
		warnings = append(warnings, Warning{
			Service: name,
			Key:     "cgroup",
			Message: "'cgroup' is not supported.",
		})
	}

	if service.Isolation != "" {
		warnings = append(warnings, Warning{
			Service: name,
			Key:     "isolation",
			Message: "'isolation' is not supported.",
		})
	}

	if service.Runtime != "" {
		warnings = append(warnings, Warning{
			Service: name,
			Key:     "runtime",
			Message: "'runtime' is not supported.",
		})
	}

	if service.StopSignal != "" {
		warnings = append(warnings, Warning{
			Service: name,
			Key:     "stop_signal",
			Message: "'stop_signal' is not supported.",
		})
	}

	if service.StopGracePeriod != nil {
		warnings = append(warnings, Warning{
			Service: name,
			Key:     "stop_grace_period",
			Message: "'stop_grace_period' is not supported.",
		})
	}

	if service.MacAddress != "" {
		warnings = append(warnings, Warning{
			Service: name,
			Key:     "mac_address",
			Message: "'mac_address' is not supported.",
		})
	}

	if service.Tty {
		warnings = append(warnings, Warning{
			Service: name,
			Key:     "tty",
			Message: "'tty' is not supported.",
		})
	}

	if service.StdinOpen {
		warnings = append(warnings, Warning{
			Service: name,
			Key:     "stdin_open",
			Message: "'stdin_open' is not supported.",
		})
	}

	if service.OomKillDisable {
		warnings = append(warnings, Warning{
			Service: name,
			Key:     "oom_kill_disable",
			Message: "'oom_kill_disable' is not supported.",
		})
	}

	if service.OomScoreAdj != 0 {
		warnings = append(warnings, Warning{
			Service: name,
			Key:     "oom_score_adj",
			Message: "'oom_score_adj' is not supported.",
		})
	}

	if service.PidsLimit != 0 {
		warnings = append(warnings, Warning{
			Service: name,
			Key:     "pids_limit",
			Message: "'pids_limit' is not supported.",
		})
	}

	if len(service.StorageOpt) > 0 {
		warnings = append(warnings, Warning{
			Service: name,
			Key:     "storage_opt",
			Message: "'storage_opt' is not supported.",
		})
	}

	if len(service.DeviceCgroupRules) > 0 {
		warnings = append(warnings, Warning{
			Service: name,
			Key:     "device_cgroup_rules",
			Message: "'device_cgroup_rules' is not supported.",
		})
	}

	if service.CredentialSpec != nil {
		warnings = append(warnings, Warning{
			Service: name,
			Key:     "credential_spec",
			Message: "'credential_spec' is not supported.",
		})
	}

	if len(service.GroupAdd) > 0 {
		warnings = append(warnings, Warning{
			Service: name,
			Key:     "group_add",
			Message: "'group_add' is not supported.",
		})
	}

	if service.BlkioConfig != nil {
		warnings = append(warnings, Warning{
			Service: name,
			Key:     "blkio_config",
			Message: "'blkio_config' is not supported.",
		})
	}

	if service.CPUCount > 0 {
		warnings = append(warnings, Warning{
			Service: name,
			Key:     "cpu_count",
			Message: "'cpu_count' is not supported.",
		})
	}

	if service.CPUPercent > 0 {
		warnings = append(warnings, Warning{
			Service: name,
			Key:     "cpu_percent",
			Message: "'cpu_percent' is not supported.",
		})
	}

	if service.CPUPeriod > 0 {
		warnings = append(warnings, Warning{
			Service: name,
			Key:     "cpu_period",
			Message: "'cpu_period' is not supported.",
		})
	}

	if service.CPUQuota > 0 {
		warnings = append(warnings, Warning{
			Service: name,
			Key:     "cpu_quota",
			Message: "'cpu_quota' is not supported.",
		})
	}

	if service.CPURTPeriod > 0 {
		warnings = append(warnings, Warning{
			Service: name,
			Key:     "cpu_rt_period",
			Message: "'cpu_rt_period' is not supported.",
		})
	}

	if service.CPURTRuntime > 0 {
		warnings = append(warnings, Warning{
			Service: name,
			Key:     "cpu_rt_runtime",
			Message: "'cpu_rt_runtime' is not supported.",
		})
	}

	if service.CPUShares != 0 {
		warnings = append(warnings, Warning{
			Service: name,
			Key:     "cpu_shares",
			Message: "'cpu_shares' is not supported.",
		})
	}

	if service.MemSwappiness > 0 {
		warnings = append(warnings, Warning{
			Service: name,
			Key:     "mem_swappiness",
			Message: "'mem_swappiness' is not supported.",
		})
	}

	if service.DomainName != "" {
		warnings = append(warnings, Warning{
			Service: name,
			Key:     "domainname",
			Message: "'domainname' is not supported.",
		})
	}

	if service.Attach != nil && !*service.Attach {
		warnings = append(warnings, Warning{
			Service: name,
			Key:     "attach",
			Message: "'attach' is not supported.",
		})
	}

	if len(service.Labels) > 0 {
		warnings = append(warnings, Warning{
			Service: name,
			Key:     "labels",
			Message: "'labels' at service level is not supported.",
		})
	}

	if len(service.Annotations) > 0 {
		warnings = append(warnings, Warning{
			Service: name,
			Key:     "annotations",
			Message: "'annotations' is not supported.",
		})
	}

	if service.Extends != nil {
		warnings = append(warnings, Warning{
			Service: name,
			Key:     "extends",
			Message: "'extends' is not supported.",
		})
	}

	if len(service.PostStart) > 0 {
		warnings = append(warnings, Warning{
			Service: name,
			Key:     "post_start",
			Message: "'post_start' is not supported.",
		})
	}

	if len(service.PreStop) > 0 {
		warnings = append(warnings, Warning{
			Service: name,
			Key:     "pre_stop",
			Message: "'pre_stop' is not supported.",
		})
	}

	if service.Provider != nil {
		warnings = append(warnings, Warning{
			Service: name,
			Key:     "provider",
			Message: "'provider' is not supported.",
		})
	}

	if len(service.Models) > 0 {
		warnings = append(warnings, Warning{
			Service: name,
			Key:     "models",
			Message: "'models' is not supported.",
		})
	}

	if service.VolumeDriver != "" {
		warnings = append(warnings, Warning{
			Service: name,
			Key:     "volume_driver",
			Message: "'volume_driver' is not supported.",
		})
	}

	if service.UseAPISocket {
		warnings = append(warnings, Warning{
			Service: name,
			Key:     "use_api_socket",
			Message: "'use_api_socket' is not supported.",
		})
	}

	if service.Net != "" {
		warnings = append(warnings, Warning{
			Service: name,
			Key:     "net",
			Message: "'net' is deprecated and not supported. Use 'network_mode' instead.",
		})
	}

	if service.Deploy != nil {
		warnings = append(warnings, checkDeployWarnings(name, service.Deploy)...)
	}

	slices.SortFunc(warnings, func(a, b Warning) int {
		if a.Key < b.Key {
			return -1
		} else if a.Key > b.Key {
			return 1
		}
		return 0
	})

	return warnings
}

func checkDeployWarnings(serviceName string, deploy *types.DeployConfig) []Warning {
	var warnings []Warning

	if len(deploy.Labels) > 0 {
		warnings = append(warnings, Warning{
			Service: serviceName,
			Key:     "deploy.labels",
			Message: "'deploy.labels' is not supported.",
		})
	}

	if deploy.RollbackConfig != nil {
		warnings = append(warnings, Warning{
			Service: serviceName,
			Key:     "deploy.rollback_config",
			Message: "'deploy.rollback_config' is not supported.",
		})
	}

	if deploy.RestartPolicy != nil {
		warnings = append(warnings, Warning{
			Service: serviceName,
			Key:     "deploy.restart_policy",
			Message: "'deploy.restart_policy' is not supported. Uncloud manages container lifecycle.",
		})
	}

	if deploy.EndpointMode != "" {
		warnings = append(warnings, Warning{
			Service: serviceName,
			Key:     "deploy.endpoint_mode",
			Message: "'deploy.endpoint_mode' is not supported.",
		})
	}

	if deploy.Placement.Constraints != nil || len(deploy.Placement.Preferences) > 0 {
		warnings = append(warnings, Warning{
			Service: serviceName,
			Key:     "deploy.placement",
			Message: "'deploy.placement' is not supported. Use 'x-machines' extension for machine placement.",
		})
	}

	if deploy.UpdateConfig != nil {
		if deploy.UpdateConfig.Parallelism != nil {
			warnings = append(warnings, Warning{
				Service: serviceName,
				Key:     "deploy.update_config.parallelism",
				Message: "'deploy.update_config.parallelism' is not supported.",
			})
		}

		if deploy.UpdateConfig.Delay > 0 {
			warnings = append(warnings, Warning{
				Service: serviceName,
				Key:     "deploy.update_config.delay",
				Message: "'deploy.update_config.delay' is not supported.",
			})
		}

		if deploy.UpdateConfig.FailureAction != "" {
			warnings = append(warnings, Warning{
				Service: serviceName,
				Key:     "deploy.update_config.failure_action",
				Message: "'deploy.update_config.failure_action' is not supported.",
			})
		}

		if deploy.UpdateConfig.Monitor > 0 {
			warnings = append(warnings, Warning{
				Service: serviceName,
				Key:     "deploy.update_config.monitor",
				Message: "'deploy.update_config.monitor' is not supported.",
			})
		}

		if deploy.UpdateConfig.MaxFailureRatio > 0 {
			warnings = append(warnings, Warning{
				Service: serviceName,
				Key:     "deploy.update_config.max_failure_ratio",
				Message: "'deploy.update_config.max_failure_ratio' is not supported.",
			})
		}
	}

	return warnings
}
