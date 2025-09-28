# Physical Architecture Discovery Design Document

## Overview

This document outlines the core code design for discovering physical architecture components in Git repositories - identifying services like frontend, backend, database, and their directional relationships.

## Problem Statement

Given a repository, automatically discover and document:

1. **Component observations** - what services, applications, and infrastructure we found and why
2. **Connection patterns** - how components communicate and interact
3. **Infrastructure insights** - deployment, configuration, and operational patterns
4. **Discovery context** - confidence levels, sources, and methodology notes

The output is designed to be consumed by LLMs to generate comprehensive architecture reports for human readers.

## Domain Model

### Core Value Objects

```python
@dataclass
class ArchitectureDiscoveryNotes:
    """Rich, narrative observations about repository architecture for LLM consumption."""
    repository_context: str  # High-level overview and discovery scope
    component_observations: List[str]  # Detailed findings about each component
    connection_observations: List[str]  # How components interact and communicate
    infrastructure_observations: List[str]  # Deployment, config, operational patterns
    discovery_metadata: str  # Methodology, confidence, limitations, timestamp
```

## Discovery Strategy

The discovery process generates rich, narrative observations that capture not just what was found, but the reasoning behind conclusions and the confidence level of each finding.

### 1. Component Observation Generation

**Directory Structure Analysis:**
- Document discovered directories and their likely purposes
- Note naming patterns and conventions used
- Explain reasoning for component role inference
- Include confidence indicators based on supporting evidence

**Code Pattern Recognition:**
- Identify framework usage and architectural patterns
- Document entry points and their significance
- Note technology stack choices and their implications
- Explain how patterns lead to component classification

**Infrastructure Configuration:**
- Parse Docker Compose, Kubernetes manifests, build files
- Document deployment patterns and service definitions
- Note port mappings, volume mounts, environment variables
- Explain how infrastructure supports component roles

### 2. Connection Pattern Documentation

**Inter-service Communication:**
- Document HTTP endpoints, database connections, queue usage
- Note communication protocols and patterns
- Explain data flow and service dependencies
- Include configuration evidence supporting connections

**Network Topology:**
- Document service mesh, load balancer, proxy configurations
- Note ingress patterns and external access points
- Explain security and networking constraints
- Include infrastructure-level connection evidence

## Implementation

### Domain Layer

```python
# src/kodit/domain/services/physical_architecture_service.py
class PhysicalArchitectureService:
    """Core service for discovering physical architecture and generating narrative observations."""

    async def discover_architecture(self, repo_path: Path) -> str:
        """Discover physical architecture and generate rich narrative observations."""

        # Generate repository context overview
        repo_context = await self._analyze_repository_context(repo_path)

        # Collect observations from all detectors
        component_notes = []
        connection_notes = []
        infrastructure_notes = []

        # Run detectors and collect narrative observations
        docker_observations = await self.docker_detector.analyze(repo_path)
        component_notes.extend(docker_observations.component_notes)
        connection_notes.extend(docker_observations.connection_notes)
        infrastructure_notes.extend(docker_observations.infrastructure_notes)

        k8s_observations = await self.k8s_detector.analyze(repo_path)
        component_notes.extend(k8s_observations.component_notes)
        connection_notes.extend(k8s_observations.connection_notes)
        infrastructure_notes.extend(k8s_observations.infrastructure_notes)

        code_observations = await self.code_detector.analyze(repo_path)
        component_notes.extend(code_observations.component_notes)
        connection_notes.extend(code_observations.connection_notes)

        # Generate discovery metadata
        discovery_metadata = self._generate_discovery_metadata(repo_path)

        # Create comprehensive notes
        notes = ArchitectureDiscoveryNotes(
            repository_context=repo_context,
            component_observations=component_notes,
            connection_observations=connection_notes,
            infrastructure_observations=infrastructure_notes,
            discovery_metadata=discovery_metadata
        )

        return self._format_notes_for_llm(notes)

    async def _analyze_repository_context(self, repo_path: Path) -> str:
        """Generate high-level repository context and scope."""

    def _generate_discovery_metadata(self, repo_path: Path) -> str:
        """Document discovery methodology, confidence, and limitations."""

    def _format_notes_for_llm(self, notes: ArchitectureDiscoveryNotes) -> str:
        """Format observations as natural text optimized for LLM consumption."""
```

### Infrastructure Layer

```python
@dataclass
class DetectorObservations:
    """Narrative observations from a single detector."""
    component_notes: List[str]
    connection_notes: List[str]
    infrastructure_notes: List[str]

# src/kodit/infrastructure/physical_architecture/detectors/docker_compose_detector.py
class DockerComposeDetector:
    async def analyze(self, repo_path: Path) -> DetectorObservations:
        """Generate narrative observations from Docker Compose analysis."""
        component_notes = []
        connection_notes = []
        infrastructure_notes = []

        # Example observations:
        # component_notes.append("Found 'api' service in docker-compose.yml configured as Go application. Service exposes port 8080 and depends on PostgreSQL, suggesting this is the main backend API server.")
        # connection_notes.append("Docker Compose 'depends_on' shows api service requires postgres service to start first, indicating database dependency.")
        # infrastructure_notes.append("Docker Compose configuration includes volume mounts for persistent data and environment variable injection, indicating production-ready deployment setup.")

# src/kodit/infrastructure/physical_architecture/detectors/kubernetes_detector.py
class KubernetesDetector:
    async def analyze(self, repo_path: Path) -> DetectorObservations:
        """Generate narrative observations from Kubernetes manifest analysis."""

# src/kodit/infrastructure/physical_architecture/detectors/code_structure_detector.py
class CodeStructureDetector:
    def __init__(self, slicer: Slicer):
        self.slicer = slicer

    async def analyze(self, repo_path: Path) -> DetectorObservations:
        """Generate narrative observations from code structure and call graph analysis."""
        # Use slicer to build detailed observations about:
        # - Function entry points and their purposes
        # - Import patterns and module boundaries
        # - Framework usage and architectural patterns
        # - Cross-component communication patterns
```

## Observation Consolidation Strategy

Instead of traditional reconciliation, we now focus on **consolidating narrative observations** to create a comprehensive story about the architecture.

### Cross-Source Validation

**Confidence Building:**
- When multiple detectors observe the same component, increase confidence level
- Note when infrastructure and code analysis align on component purposes
- Document discrepancies as areas of uncertainty

**Evidence Aggregation:**
- Combine evidence from multiple sources into richer component descriptions
- Cross-reference configuration with actual code patterns
- Build comprehensive connection stories using multiple evidence sources

**Narrative Enhancement:**
- Use infrastructure findings to explain deployment context for code components
- Leverage code analysis to explain the purpose behind infrastructure configurations
- Create holistic component descriptions that span implementation and deployment

### Observation Quality

**Context-Rich Descriptions:**
```python
# Instead of: "api service, port 8080"
# Generate: "Found 'api' directory containing Go HTTP server with 15 REST endpoints for user management. Docker Compose configuration exposes this on port 8080 with PostgreSQL dependency, and Kubernetes manifests show it's deployed as a horizontally scalable service with 3 replicas."
```

**Confidence Indicators:**
- High confidence: Multiple sources agree (code + infrastructure)
- Medium confidence: Single authoritative source (e.g., Dockerfile)
- Low confidence: Inferred from indirect evidence (e.g., naming patterns)

## Detection Patterns

### Docker Compose Detection

```bash
# Extract service names from Docker Compose
find . -name "docker-compose*.yml" -o -name "docker-compose*.yaml"
grep -E "^\s*[a-zA-Z][a-zA-Z0-9_-]*:" docker-compose.yml | sed 's/://'

# Extract service dependencies
grep -A 10 "depends_on:" docker-compose.yml

# Extract port mappings
grep -E "ports:|expose:" docker-compose.yml
```

### Kubernetes Detection

```bash
# Find Kubernetes manifests
find . -name "*.yaml" -o -name "*.yml" | grep -E "(k8s|kubernetes|manifests|deploy)"

# Extract Deployment names
grep -E "^kind: Deployment" -A 5 *.yaml | grep "name:"

# Extract Service names
grep -E "^kind: Service" -A 5 *.yaml | grep "name:"

# Extract port configurations
grep -A 10 "ports:" *.yaml
```

### Code Structure Detection (Using Slicer)

**Component Detection via Slicer:**

```python
# Leverage existing slicer analysis to find components
def detect_components_from_code(self, repo_path: Path) -> List[ArchitectureComponent]:
    # 1. Use slicer to build call graphs and import maps
    # 2. Identify entry points (main functions, HTTP handlers)
    # 3. Group related functions by module/package
    # 4. Infer component boundaries from import patterns

    # Extract file structure patterns
    component_indicators = self._find_component_directories(repo_path)

    # Use slicer for each language found
    for language in ["python", "go", "javascript", "java"]:
        files = self._find_files_for_language(repo_path, language)
        if files:
            analysis = self.slicer.extract_snippets_from_git_files(files, language)
            # Analyze call graphs to identify service boundaries
            components.extend(self._extract_components_from_analysis(analysis))
```

**Role Inference from Code Analysis:**

```python
# Use slicer's function analysis to infer roles
def infer_component_role(self, analysis_result) -> str:
    # Web server: functions with HTTP handler patterns
    if self._has_http_handlers(analysis_result):
        return "web_server"

    # Frontend: imports/calls to DOM, React, Vue etc.
    if self._has_frontend_patterns(analysis_result):
        return "client_app"

    # Database: SQL queries, ORM patterns
    if self._has_database_patterns(analysis_result):
        return "database_client"

    # Worker: queue/job processing patterns
    if self._has_worker_patterns(analysis_result):
        return "background_worker"
```

**Connection Detection via Call Graph:**

```python
# Use slicer's call graph to detect inter-component communication
def detect_connections_from_code(self, components) -> List[ComponentConnection]:
    # 1. Analyze import dependencies between components
    # 2. Find HTTP client calls that cross component boundaries
    # 3. Identify database connections
    # 4. Map function calls between different modules/packages
```

## Example Output

For a typical web application, stored as narrative text blob in enrichment:

```
# Repository Architecture Discovery

## Repository Context
This appears to be a modern web application with a React frontend, Go backend API, and PostgreSQL database. The repository follows a standard microservices pattern with clear separation between client and server components. Infrastructure as code is provided through Docker Compose for local development and Kubernetes manifests for production deployment.

## Component Observations

**Frontend Application (High Confidence)**
Found 'frontend/' directory containing a React TypeScript application with entry point at frontend/src/index.tsx. The package.json indicates this is a single-page application using modern React patterns with routing and state management. Docker Compose configuration serves this on port 3000 during development. Build process creates static assets that can be served by any web server in production.

**API Backend Service (High Confidence)**
Discovered 'api/' directory with Go HTTP server implementation in api/main.go. Code analysis reveals 15 REST endpoints handling user management, authentication, and business logic. Framework analysis shows usage of Gin HTTP router with middleware for CORS and authentication. Docker Compose exposes port 8080, and Kubernetes manifests configure horizontal scaling with 3 replicas behind a load balancer.

**PostgreSQL Database (Medium Confidence)**
Docker Compose configuration defines PostgreSQL service on port 5432 with persistent volume mounts. Database schema migrations found in api/migrations/ directory suggest this is the primary data store. Environment variables in both Docker and Kubernetes configurations confirm database connection parameters.

## Connection Observations

**Frontend to API Communication (High Confidence)**
Frontend code contains HTTP client calls to '/api' endpoints with axios library. Environment configuration shows API_BASE_URL pointing to backend service. CORS configuration in API allows requests from frontend origin, confirming client-server relationship.

**API to Database Communication (High Confidence)**
Go backend code includes PostgreSQL driver imports and connection pooling setup. Database connection string constructed from environment variables matches PostgreSQL service configuration. SQL queries and ORM usage patterns confirm this is a traditional client-server database relationship.

## Infrastructure Observations

**Development Environment**
Docker Compose configuration provides complete local development stack with hot reloading for frontend and API service restart on code changes. Volume mounts enable live development workflow.

**Production Deployment**
Kubernetes manifests indicate production deployment with services exposed through ingress controller. ConfigMaps and Secrets management show production-ready secret handling. Resource limits and health checks configured for reliability.

## Discovery Metadata
Analysis completed using Docker Compose parsing, Kubernetes manifest analysis, and code structure examination via Tree-sitter. High confidence in component identification due to multiple source validation. Moderate confidence in deployment patterns based on infrastructure configuration analysis. Discovery performed on [timestamp] with methodology version 1.0.
```

## File Structure and Isolation

This feature will be integrated into the existing kodit architecture following DDD principles:

```
# Domain Layer - Core business logic
src/kodit/domain/value_objects/
└── physical_architecture.py             # ArchitectureDiscoveryNotes, DetectorObservations

src/kodit/domain/services/
└── physical_architecture_service.py     # Main narrative generation service

# Application Layer - Use cases and workflows
src/kodit/application/services/
└── architecture_discovery_service.py    # Integration with existing enrichment system

# Infrastructure Layer - External concerns and implementations
src/kodit/infrastructure/physical_architecture/
├── __init__.py
├── detectors/
│   ├── __init__.py
│   ├── docker_compose_detector.py
│   ├── kubernetes_detector.py
│   ├── code_structure_detector.py
│   └── build_system_detector.py
├── formatters/
│   ├── __init__.py
│   └── narrative_formatter.py           # LLM-optimized text formatting
└── consolidators/
    ├── __init__.py
    └── observation_consolidator.py       # Cross-source narrative enhancement

# Tests following the same structure
tests/kodit/domain/services/
└── physical_architecture_service_test.py

tests/kodit/infrastructure/physical_architecture/
├── detectors/
│   ├── docker_compose_detector_test.py
│   └── code_structure_detector_test.py
├── formatters/
│   └── narrative_formatter_test.py
├── end_to_end_test.py                   # Full discovery on real fixtures
└── fixtures/
    ├── simple_web_app/
    │   ├── docker-compose.yml
    │   └── api/main.go
    └── microservices/
        ├── docker-compose.yml
        └── k8s/deployment.yaml
```

**Key Architecture Principles:**

1. **Domain-Driven Design**: Follows existing kodit DDD structure with clear separation of concerns
2. **Narrative-First Design**: Optimized for LLM consumption and human report generation
3. **Evidence-Based Observations**: Rich context and reasoning included in all findings
4. **Minimal Dependencies**: Only imports from standard library, existing kodit domain entities, and the slicer
5. **Clean Interface**: Application service integrates with existing enrichment system
6. **Layered Testing**: Tests mirror the layered architecture structure

**Integration Points (Minimal):**

- Import existing `GitFile` entity for slicer integration
- Import existing `Slicer` class for code analysis
- Application service integrates with enrichment system via string serialization
- Narrative output optimized for downstream LLM report generation

## Implementation Plan

### Phase 1: Infrastructure Foundation (Week 1-2)

**1.1 Core Value Objects**

- Implement `ArchitectureDiscoveryNotes` and `DetectorObservations` data classes in `src/kodit/domain/value_objects/physical_architecture.py`
- Add narrative text formatting methods
- **Test**: No dedicated tests needed - will be tested through detector usage

**1.2 Docker Compose Detector (Start Simple)**

- Implement `DockerComposeDetector.analyze()` in `src/kodit/infrastructure/physical_architecture/detectors/docker_compose_detector.py`
- Parse single docker-compose.yml, generate narrative observations about services
- **Test**: Single test with minimal docker-compose.yml fixture in `tests/kodit/infrastructure/physical_architecture/detectors/docker_compose_detector_test.py`

**1.3 Narrative Formatter (Essential Logic Only)**

- Implement `NarrativeFormatter` in `src/kodit/infrastructure/physical_architecture/formatters/narrative_formatter.py`
- Convert observations into LLM-optimized text format
- **Test**: Single test with sample observations in `tests/kodit/infrastructure/physical_architecture/formatters/narrative_formatter_test.py`

### Phase 2: Minimal Expansion (Week 3)

**2.1 Add One More Detector**

- Implement `KubernetesDetector` OR `CodeStructureDetector` (choose simpler)
- **Test**: Add to existing end-to-end test

**2.2 End-to-End Integration**

- Implement `PhysicalArchitectureService.discover_architecture()` in `src/kodit/domain/services/physical_architecture_service.py`
- Run detectors, reconcile, return string
- **Test**: Single end-to-end test with simple fixture in `tests/kodit/infrastructure/physical_architecture/end_to_end_test.py`

### Phase 3: Polish and Ship (Week 4)

**3.1 Basic Connection Detection**

- Add simple connection detection to Docker Compose detector (`depends_on`)
- **Test**: Extend existing test to verify connections

**3.2 Integration Point**

- Implement application service for enrichment system integration in `src/kodit/application/services/architecture_discovery_service.py`
- **Test**: Verify string serialization works in end-to-end test

**3.3 Optional Enhancements (If Time Permits)**

- Add more detectors (K8s, build systems, slicer integration)
- Each addition should be minimal and focus on high-value patterns
- **Test**: Extend end-to-end test only

## Minimal Testing Strategy

### Essential Tests Only

**3 Test Files Total:**

1. `tests/kodit/infrastructure/physical_architecture/detectors/docker_compose_detector_test.py` - Parse simple YAML, extract services
2. `tests/kodit/infrastructure/physical_architecture/reconcilers/component_reconciler_test.py` - Deduplicate 2-3 components by name
3. `tests/kodit/infrastructure/physical_architecture/end_to_end_test.py` - Full discovery on minimal fixture

### Single Test Fixture

```yaml
# tests/kodit/infrastructure/physical_architecture/fixtures/simple_web_app/docker-compose.yml
services:
  api:
    build: ./api
    ports: ["8080:8080"]
    depends_on: [postgres]
  frontend:
    build: ./frontend
    ports: ["3000:3000"]
  postgres:
    image: postgres
    ports: ["5432:5432"]
```

### Test Guidelines for Engineers

- **Start with the end-to-end test** - it will drive the implementation
- **Don't test value objects** - they're just data structures
- **Don't test every detector** - prove the pattern works with one, then add others
- **Don't test error conditions** - focus on the happy path first
- **Don't test performance** - premature optimization
- **Each test should be <20 lines** - if longer, the code is too complex

This minimal approach gets you working software faster and catches real bugs through actual usage rather than theoretical edge cases.
