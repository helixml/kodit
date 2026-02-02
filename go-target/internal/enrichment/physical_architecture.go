package enrichment

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// PhysicalArchitectureService discovers physical architecture from repositories.
type PhysicalArchitectureService struct{}

// NewPhysicalArchitectureService creates a new PhysicalArchitectureService.
func NewPhysicalArchitectureService() *PhysicalArchitectureService {
	return &PhysicalArchitectureService{}
}

// Discover analyzes a repository and returns a narrative description of its architecture.
func (s *PhysicalArchitectureService) Discover(ctx context.Context, repoPath string) (string, error) {
	repoContext := s.analyzeRepositoryContext(repoPath)

	componentNotes, connectionNotes, infrastructureNotes := s.analyzeDockerCompose(repoPath)

	metadata := s.generateDiscoveryMetadata()

	return s.formatForLLM(repoContext, componentNotes, connectionNotes, infrastructureNotes, metadata), nil
}

func (s *PhysicalArchitectureService) analyzeRepositoryContext(repoPath string) string {
	var observations []string

	observations = append(observations, fmt.Sprintf("Analyzing repository at %s", repoPath))

	hasDockerCompose := len(s.findDockerComposeFiles(repoPath)) > 0
	hasDockerfile := s.hasFiles(repoPath, "Dockerfile*")
	hasK8s := s.hasFiles(repoPath, "**/k8s/**/*.yaml") || s.hasFiles(repoPath, "**/kubernetes/**/*.yaml")
	hasPackageJSON := s.fileExists(filepath.Join(repoPath, "package.json"))
	hasRequirements := s.fileExists(filepath.Join(repoPath, "requirements.txt"))
	hasGoMod := s.fileExists(filepath.Join(repoPath, "go.mod"))

	var indicators []string
	if hasDockerCompose {
		indicators = append(indicators, "Docker Compose orchestration")
	}
	if hasDockerfile {
		indicators = append(indicators, "containerized deployment")
	}
	if hasK8s {
		indicators = append(indicators, "Kubernetes deployment")
	}
	if hasPackageJSON {
		indicators = append(indicators, "Node.js/JavaScript components")
	}
	if hasRequirements {
		indicators = append(indicators, "Python components")
	}
	if hasGoMod {
		indicators = append(indicators, "Go components")
	}

	if len(indicators) > 0 {
		observations = append(observations, fmt.Sprintf(
			"Repository shows evidence of %s, suggesting a modern containerized application architecture.",
			strings.Join(indicators, ", "),
		))
	} else {
		observations = append(observations,
			"Repository structure analysis shows limited infrastructure configuration. "+
				"This may be a simple application or library without complex deployment requirements.",
		)
	}

	return strings.Join(observations, " ")
}

func (s *PhysicalArchitectureService) analyzeDockerCompose(repoPath string) ([]string, []string, []string) {
	var componentNotes, connectionNotes, infrastructureNotes []string

	composeFiles := s.findDockerComposeFiles(repoPath)
	if len(composeFiles) == 0 {
		return componentNotes, connectionNotes, infrastructureNotes
	}

	for _, composeFile := range composeFiles {
		data, err := os.ReadFile(composeFile)
		if err != nil {
			infrastructureNotes = append(infrastructureNotes,
				fmt.Sprintf("Unable to read Docker Compose file at %s.", composeFile))
			continue
		}

		var composeData map[string]any
		if err := yaml.Unmarshal(data, &composeData); err != nil {
			infrastructureNotes = append(infrastructureNotes,
				fmt.Sprintf("Unable to parse Docker Compose file at %s. File may be malformed.", composeFile))
			continue
		}

		services, ok := composeData["services"].(map[string]any)
		if !ok {
			continue
		}

		fileName := filepath.Base(composeFile)
		infrastructureNotes = append(infrastructureNotes,
			fmt.Sprintf("Found Docker Compose configuration at %s defining %d services. "+
				"This suggests a containerized application architecture with orchestrated service dependencies.",
				fileName, len(services)))

		for serviceName, serviceConfigAny := range services {
			serviceConfig, ok := serviceConfigAny.(map[string]any)
			if !ok {
				continue
			}
			s.analyzeService(serviceName, serviceConfig, &componentNotes)
		}

		s.analyzeServiceDependencies(services, &connectionNotes)
		s.analyzeComposeFeatures(composeData, &infrastructureNotes)
	}

	return componentNotes, connectionNotes, infrastructureNotes
}

func (s *PhysicalArchitectureService) analyzeService(serviceName string, config map[string]any, componentNotes *[]string) {
	observation := fmt.Sprintf("Found '%s' service in Docker Compose configuration.", serviceName)

	if image, ok := config["image"].(string); ok && image != "" {
		observation += fmt.Sprintf(" Service uses '%s' Docker image", image)
		if strings.Contains(image, ":") {
			parts := strings.Split(image, ":")
			observation += fmt.Sprintf(" with tag '%s'", parts[len(parts)-1])
		}
		observation += "."
	} else if build := config["build"]; build != nil {
		buildStr := fmt.Sprintf("%v", build)
		observation += fmt.Sprintf(" Service builds from local source at '%s'.", buildStr)
	}

	ports := s.extractPorts(config)
	if len(ports) > 0 {
		portStrs := make([]string, len(ports))
		for i, p := range ports {
			portStrs[i] = strconv.Itoa(p)
		}
		observation += fmt.Sprintf(" Exposes ports %s", strings.Join(portStrs, ", "))
		if protocolInfo := s.inferProtocolDescription(ports); protocolInfo != "" {
			observation += fmt.Sprintf(" suggesting %s", protocolInfo)
		}
		observation += "."
	}

	*componentNotes = append(*componentNotes, observation)
}

func (s *PhysicalArchitectureService) analyzeServiceDependencies(services map[string]any, connectionNotes *[]string) {
	serviceNames := make(map[string]bool)
	for name := range services {
		serviceNames[name] = true
	}

	communicationPattern := regexp.MustCompile(
		`(?:https?|tcp|grpc|ws|wss|amqp|kafka|redis|memcached|` +
			`postgres(?:ql)?(?:\+\w+)?|mysql|mongodb)://` +
			`(?:[^@/]+@)?` +
			`([\w\-\.]+(?::\d+)?)`,
	)

	for serviceName, serviceConfigAny := range services {
		serviceConfig, ok := serviceConfigAny.(map[string]any)
		if !ok {
			continue
		}

		dependsOn := serviceConfig["depends_on"]
		var dependencies []string

		switch dep := dependsOn.(type) {
		case []any:
			for _, d := range dep {
				if depStr, ok := d.(string); ok {
					dependencies = append(dependencies, depStr)
				}
			}
		case map[string]any:
			for d := range dep {
				dependencies = append(dependencies, d)
			}
		}

		if len(dependencies) > 0 {
			*connectionNotes = append(*connectionNotes,
				fmt.Sprintf("Docker Compose 'depends_on' configuration shows '%s' "+
					"requires '%s' to start first, indicating service startup dependency "+
					"and likely runtime communication pattern.",
					serviceName, strings.Join(dependencies, "', '")))
		}

		s.checkEnvironmentForConnections(serviceName, serviceConfig, serviceNames, communicationPattern, connectionNotes)
	}
}

func (s *PhysicalArchitectureService) checkEnvironmentForConnections(
	serviceName string,
	config map[string]any,
	serviceNames map[string]bool,
	pattern *regexp.Regexp,
	connectionNotes *[]string,
) {
	recordedConnections := make(map[string]bool)

	checkText := func(text, sourceType string) {
		matches := pattern.FindAllStringSubmatch(text, -1)
		for _, match := range matches {
			if len(match) > 1 {
				hostname := strings.Split(match[1], ":")[0]
				if serviceNames[hostname] && hostname != serviceName {
					key := serviceName + "->" + hostname
					if !recordedConnections[key] {
						*connectionNotes = append(*connectionNotes,
							fmt.Sprintf("'%s' has a communication address referencing '%s' in its %s, "+
								"indicating a direct runtime dependency.",
								serviceName, hostname, sourceType))
						recordedConnections[key] = true
					}
				}
			}
		}
	}

	env := config["environment"]
	switch e := env.(type) {
	case []any:
		for _, v := range e {
			if str, ok := v.(string); ok {
				checkText(str, "environment variable")
			}
		}
	case map[string]any:
		for _, v := range e {
			if str, ok := v.(string); ok {
				checkText(str, "environment variable")
			}
		}
	}
}

func (s *PhysicalArchitectureService) analyzeComposeFeatures(composeData map[string]any, infrastructureNotes *[]string) {
	if networks, ok := composeData["networks"].(map[string]any); ok && len(networks) > 0 {
		*infrastructureNotes = append(*infrastructureNotes,
			fmt.Sprintf("Docker Compose defines %d custom networks, indicating network segmentation "+
				"and controlled service communication.", len(networks)))
	}
}

func (s *PhysicalArchitectureService) extractPorts(config map[string]any) []int {
	portSet := make(map[int]bool)

	ports, _ := config["ports"].([]any)
	for _, portSpec := range ports {
		switch p := portSpec.(type) {
		case string:
			if strings.Contains(p, ":") {
				parts := strings.Split(p, ":")
				if port, err := strconv.Atoi(parts[0]); err == nil {
					portSet[port] = true
				}
			} else {
				if port, err := strconv.Atoi(p); err == nil {
					portSet[port] = true
				}
			}
		case int:
			portSet[p] = true
		case float64:
			portSet[int(p)] = true
		}
	}

	expose, _ := config["expose"].([]any)
	for _, exposeSpec := range expose {
		switch e := exposeSpec.(type) {
		case string:
			if port, err := strconv.Atoi(e); err == nil {
				portSet[port] = true
			}
		case int:
			portSet[e] = true
		case float64:
			portSet[int(e)] = true
		}
	}

	var result []int
	for p := range portSet {
		result = append(result, p)
	}
	sort.Ints(result)
	return result
}

func (s *PhysicalArchitectureService) inferProtocolDescription(ports []int) string {
	var protocols []string

	httpPorts := map[int]bool{80: true, 8080: true, 3000: true, 4200: true, 5000: true, 8000: true, 8443: true, 443: true}
	grpcPorts := map[int]bool{9090: true, 50051: true}
	dbPorts := map[int]bool{5432: true, 3306: true, 27017: true}

	for _, port := range ports {
		if httpPorts[port] {
			protocols = append(protocols, "HTTP/HTTPS web traffic")
			break
		}
	}

	for _, port := range ports {
		if grpcPorts[port] {
			protocols = append(protocols, "gRPC API communication")
			break
		}
	}

	for _, port := range ports {
		if port == 6379 {
			protocols = append(protocols, "cache service")
			break
		}
	}

	for _, port := range ports {
		if dbPorts[port] {
			protocols = append(protocols, "database service")
			break
		}
	}

	if len(protocols) > 0 {
		return strings.Join(protocols, " and ")
	}
	if len(ports) > 0 {
		return "TCP-based service communication"
	}
	return ""
}

func (s *PhysicalArchitectureService) findDockerComposeFiles(repoPath string) []string {
	var files []string

	patterns := []string{"docker-compose*.yml", "docker-compose*.yaml"}
	for _, pattern := range patterns {
		matches, _ := filepath.Glob(filepath.Join(repoPath, pattern))
		files = append(files, matches...)
	}

	return files
}

func (s *PhysicalArchitectureService) hasFiles(repoPath, pattern string) bool {
	matches, _ := filepath.Glob(filepath.Join(repoPath, pattern))
	return len(matches) > 0
}

func (s *PhysicalArchitectureService) fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func (s *PhysicalArchitectureService) generateDiscoveryMetadata() string {
	timestamp := time.Now().UTC().Format(time.RFC3339)

	return fmt.Sprintf(
		"Analysis completed on %s using physical architecture discovery system version 1.0. "+
			"Discovery methodology: Docker Compose parsing and infrastructure configuration analysis. "+
			"Detection sources: Docker Compose file analysis. "+
			"Confidence levels: High confidence for infrastructure-defined components, "+
			"medium confidence for inferred roles based on naming and configuration patterns. "+
			"Current limitations: analysis limited to Docker Compose configurations, "+
			"code-level analysis not yet implemented, runtime behavior patterns not captured.",
		timestamp,
	)
}

func (s *PhysicalArchitectureService) formatForLLM(
	repoContext string,
	componentNotes, connectionNotes, infrastructureNotes []string,
	metadata string,
) string {
	var sections []string

	sections = append(sections, "# Architecture Discovery Report\n")

	sections = append(sections, "## Repository Context\n")
	sections = append(sections, repoContext+"\n")

	if len(componentNotes) > 0 {
		sections = append(sections, "## Component Observations\n")
		for _, note := range componentNotes {
			sections = append(sections, "- "+note+"\n")
		}
	}

	if len(connectionNotes) > 0 {
		sections = append(sections, "## Connection Observations\n")
		for _, note := range connectionNotes {
			sections = append(sections, "- "+note+"\n")
		}
	}

	if len(infrastructureNotes) > 0 {
		sections = append(sections, "## Infrastructure Observations\n")
		for _, note := range infrastructureNotes {
			sections = append(sections, "- "+note+"\n")
		}
	}

	sections = append(sections, "## Discovery Metadata\n")
	sections = append(sections, metadata+"\n")

	return strings.Join(sections, "\n")
}
