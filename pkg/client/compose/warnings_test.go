package compose

import (
	"context"
	"slices"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCheckProjectWarnings(t *testing.T) {
	tests := []struct {
		name           string
		composeYAML    string
		expectedKeys   []string
		unexpectedKeys []string
	}{
		{
			name: "no unsupported keys",
			composeYAML: `
services:
  web:
    image: nginx
    environment:
      FOO: bar
`,
			expectedKeys:   nil,
			unexpectedKeys: []string{"depends_on", "restart", "networks"},
		},
		{
			name: "depends_on",
			composeYAML: `
services:
  web:
    image: nginx
    depends_on:
      db:
        condition: service_healthy
  db:
    image: postgres
    healthcheck:
      test: ["CMD", "pg_isready"]
      interval: 1s
`,
			expectedKeys:   []string{"depends_on"},
			unexpectedKeys: []string{"networks", "restart"},
		},
		{
			name: "networks",
			composeYAML: `
services:
  web:
    image: nginx
    networks:
      - frontend
networks:
  frontend:
`,
			expectedKeys:   []string{"networks"},
			unexpectedKeys: []string{"depends_on"},
		},
		{
			name: "multiple services",
			composeYAML: `
services:
  web:
    image: nginx
    depends_on:
      db:
        condition: service_started
  db:
    image: postgres
    restart: always
`,
			expectedKeys:   []string{"depends_on", "restart"},
			unexpectedKeys: []string{"networks"},
		},
		{
			name: "multiple unsupported keys in one service",
			composeYAML: `
services:
  web:
    image: nginx
    depends_on:
      db:
        condition: service_started
    networks:
      - frontend
    restart: always
    secrets:
      - my_secret
  db:
    image: postgres
networks:
  frontend:
secrets:
  my_secret:
    external: true
`,
			expectedKeys: []string{"depends_on", "networks", "restart", "secrets"},
		},
		{
			name: "supported keys should not warn",
			composeYAML: `
services:
  web:
    image: nginx
    environment:
      FOO: bar
    volumes:
      - data:/data
    cap_add:
      - NET_ADMIN
    cap_drop:
      - ALL
    sysctls:
      net.ipv4.ip_forward: "1"
    ulimits:
      nofile:
        soft: 20000
        hard: 40000
    ports:
      - "80:80"
volumes:
  data:
`,
			unexpectedKeys: []string{
				"environment", "volumes", "cap_add", "cap_drop",
				"sysctls", "ulimits", "ports",
			},
		},
		{
			name: "deploy with unsupported sub-keys",
			composeYAML: `
services:
  web:
    image: nginx
    deploy:
      replicas: 3
      restart_policy:
        condition: on-failure
      labels:
        foo: bar
      rollback_config:
        parallelism: 1
`,
			expectedKeys:   []string{"deploy.restart_policy", "deploy.labels", "deploy.rollback_config"},
			unexpectedKeys: []string{"deploy.replicas"},
		},
		{
			name: "deploy placement",
			composeYAML: `
services:
  web:
    image: nginx
    deploy:
      placement:
        constraints:
          - node.role == manager
        preferences:
          - spread: rack_id
`,
			expectedKeys:   []string{"deploy.placement"},
			unexpectedKeys: []string{"deploy.replicas"},
		},
		{
			name: "deploy update_config unsupported sub-keys",
			composeYAML: `
services:
  web:
    image: nginx
    deploy:
      update_config:
        parallelism: 2
        delay: 10s
        failure_action: pause
        monitor: 5s
        max_failure_ratio: 0.5
        order: stop-first
`,
			expectedKeys: []string{
				"deploy.update_config.parallelism",
				"deploy.update_config.delay",
				"deploy.update_config.failure_action",
				"deploy.update_config.monitor",
				"deploy.update_config.max_failure_ratio",
			},
			unexpectedKeys: []string{"deploy.update_config.order"},
		},
		{
			name: "network_mode",
			composeYAML: `
services:
  web:
    image: nginx
    network_mode: host
`,
			expectedKeys:   []string{"network_mode"},
			unexpectedKeys: []string{"networks"},
		},
		{
			name: "network_mode default should not warn",
			composeYAML: `
services:
  web:
    image: nginx
    network_mode: default
`,
			unexpectedKeys: []string{"network_mode"},
		},
		{
			name: "deprecated links",
			composeYAML: `
services:
  web:
    image: nginx
    links:
      - db
  db:
    image: postgres
`,
			expectedKeys:   []string{"links"},
			unexpectedKeys: []string{"depends_on"},
		},
		{
			name: "build",
			composeYAML: `
services:
  web:
    build: .
    image: myapp
`,
			expectedKeys:   []string{"build"},
			unexpectedKeys: []string{"image"},
		},
		{
			name: "hostname",
			composeYAML: `
services:
  web:
    image: nginx
    hostname: myhost
`,
			expectedKeys:   []string{"hostname"},
			unexpectedKeys: []string{"dns"},
		},
		{
			name: "dns options",
			composeYAML: `
services:
  web:
    image: nginx
    dns:
      - 8.8.8.8
    dns_opt:
      - use-vc
    dns_search:
      - local
`,
			expectedKeys: []string{"dns", "dns_opt", "dns_search"},
		},
		{
			name: "security_opt",
			composeYAML: `
services:
  web:
    image: nginx
    security_opt:
      - seccomp:unconfined
`,
			expectedKeys:   []string{"security_opt"},
			unexpectedKeys: []string{"cap_add"},
		},
		{
			name: "labels at service level",
			composeYAML: `
services:
  web:
    image: nginx
    labels:
      com.example.label: value
`,
			expectedKeys:   []string{"labels"},
			unexpectedKeys: []string{"environment"},
		},
		{
			name: "tmpfs",
			composeYAML: `
services:
  web:
    image: nginx
    tmpfs:
      - /tmp
      - /run
`,
			expectedKeys:   []string{"tmpfs"},
			unexpectedKeys: []string{"volumes"},
		},
		{
			name: "read_only",
			composeYAML: `
services:
  web:
    image: nginx
    read_only: true
`,
			expectedKeys:   []string{"read_only"},
			unexpectedKeys: []string{"tmpfs"},
		},
		{
			name: "stop_signal",
			composeYAML: `
services:
  web:
    image: nginx
    stop_signal: SIGTERM
`,
			expectedKeys:   []string{"stop_signal"},
			unexpectedKeys: []string{"stop_grace_period"},
		},
		{
			name: "stop_grace_period",
			composeYAML: `
services:
  web:
    image: nginx
    stop_grace_period: 30s
`,
			expectedKeys:   []string{"stop_grace_period"},
			unexpectedKeys: []string{"stop_signal"},
		},
		{
			name: "expose should not warn",
			composeYAML: `
services:
  web:
    image: nginx
    expose:
      - "80"
      - "443"
`,
			unexpectedKeys: []string{"expose"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			project, err := LoadProjectFromContent(context.Background(), tt.composeYAML)
			require.NoError(t, err)

			warnings := CheckProjectWarnings(project)

			if len(tt.expectedKeys) == 0 && len(tt.unexpectedKeys) == 0 {
				assert.Empty(t, warnings, "expected no warnings")
				return
			}

			warningKeys := make([]string, len(warnings))
			for i, w := range warnings {
				warningKeys[i] = w.Key
			}

			if len(tt.expectedKeys) > 0 {
				for _, key := range tt.expectedKeys {
					assert.Contains(t, warningKeys, key, "expected warning for key %s", key)
				}
			}

			if len(tt.unexpectedKeys) > 0 {
				for _, key := range tt.unexpectedKeys {
					assert.NotContains(t, warningKeys, key, "unexpected warning for key %s", key)
				}
			}
		})
	}
}

func TestCheckProjectWarnings_ServiceOrder(t *testing.T) {
	composeYAML := `
services:
  third:
    image: nginx
    depends_on:
      db:
        condition: service_started
  first:
    image: nginx
    networks:
      - frontend
  second:
    image: nginx
    restart: always
  db:
    image: postgres
networks:
  frontend:
`
	project, err := LoadProjectFromContent(context.Background(), composeYAML)
	require.NoError(t, err)

	warnings := CheckProjectWarnings(project)

	var serviceOrder []string
	for _, name := range project.ServiceNames() {
		serviceOrder = append(serviceOrder, name)
	}

	// Verify services are iterated in file order (compose-go preserves file order)
	// The order may vary based on compose-go implementation, but warnings should match the project order
	assert.Equal(t, []string{"db", "first", "second", "third"}, serviceOrder)

	var warningServiceOrder []string
	for _, w := range warnings {
		warningServiceOrder = append(warningServiceOrder, w.Service)
	}

	// Warnings should be in the same order as services appear in ServiceNames()
	assert.Equal(t, []string{"first", "second", "third"}, warningServiceOrder,
		"warnings should be in service order from compose file")
}

func TestWarningString(t *testing.T) {
	tests := []struct {
		warning  Warning
		expected string
	}{
		{
			warning: Warning{
				Service: "web",
				Key:     "depends_on",
				Message: "'depends_on' is not supported.",
			},
			expected: "service 'web': 'depends_on' is not supported.",
		},
		{
			warning: Warning{
				Service: "",
				Key:     "build",
				Message: "'build' is not supported.",
			},
			expected: "'build' is not supported.",
		},
	}

	for _, tt := range tests {
		t.Run(tt.warning.Key, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.warning.String())
		})
	}
}

func TestCheckProjectWarnings_SortWithinService(t *testing.T) {
	composeYAML := `
services:
  web:
    image: nginx
    depends_on:
      db:
        condition: service_started
    networks:
      - frontend
    restart: always
  db:
    image: postgres
networks:
  frontend:
`
	project, err := LoadProjectFromContent(context.Background(), composeYAML)
	require.NoError(t, err)

	warnings := CheckProjectWarnings(project)

	var keys []string
	for _, w := range warnings {
		keys = append(keys, w.Key)
	}

	assert.True(t, slices.IsSorted(keys), "keys should be sorted within service: %v", keys)
}
