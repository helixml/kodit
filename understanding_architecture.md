## How to Discover Any Software Architecture

### 1. Start with Orchestration Files (The Service Map)

**Files to examine**: `docker-compose.yaml`, `docker-compose.yml`, `Dockerfile`, `k8s/`, `charts/`, `deployments/`

```bash
# Look at the main services in any project
find . -name "docker-compose*.y*ml" -o -name "Dockerfile*" | head -5
grep -A 5 "services:" docker-compose.yaml 2>/dev/null || echo "No docker-compose found"

# Check for Kubernetes configs
find . -name "*.yaml" -path "*/k8s/*" -o -path "*/charts/*" | head -5
```

**Generic questions to ask**:

- What services are defined?
- Which service seems to be the main API/web server?
- What databases or storage systems are used?
- Are there any background workers or specialized services?

**HelixML example services**:

- `api`: The main control plane API server
- `postgres`: Main database
- `keycloak`: Authentication service
- `gptscript_runner`: Internal runner for script execution
- `typesense`: Search/RAG service

### 2. Find Main Application Entry Points

**Generic approach**: Look for main application files and server initialization

```bash
# Find main entry points (works for most languages)
find . -name "main.*" -o -name "app.*" -o -name "server.*" -o -name "index.*" | head -10

# Language-specific patterns:
# Go: main.go, main packages
find . -name "main.go" -o -path "*/cmd/*" -name "*.go"

# Node.js: index.js, app.js, server.js, package.json
find . -name "package.json" -o -name "index.js" -o -name "app.js" -o -name "server.js"

# Python: main.py, app.py, manage.py, wsgi.py
find . -name "main.py" -o -name "app.py" -o -name "manage.py" -o -name "wsgi.py"

# Java: Application.java, Main.java, pom.xml
find . -name "*Application.java" -o -name "*Main.java" -o -name "pom.xml"
```

**Generic questions to ask**:

- Where does the main HTTP server start?
- What port does it listen on?
- What are the main responsibilities of the core service?
- Are there separate initialization steps?

**HelixML example**:

- `api/main.go` - Entry point for the control plane
- `api/cmd/helix/serve.go` - Main server initialization
- Key responsibilities: HTTP API server (port 8080), runner management, WebSocket connections

### 3. Identify External Components and Workers

**Generic approach**: Look for separate services, workers, or external components

```bash
# Find worker/service directories
find . -maxdepth 3 -type d -name "*worker*" -o -name "*service*" -o -name "*client*" -o -name "*agent*"

# Find configuration for external components
grep -r "connect\|endpoint\|url\|host.*port" --include="*.json" --include="*.yaml" --include="*.env*" . | head -10

# Look for separate executables or CLIs
find . -name "*cmd*" -o -name "*cli*" -o -name "bin" -type d

# Check for background job or queue systems
grep -r "queue\|job\|task\|worker" --include="*.go" --include="*.py" --include="*.js" . | head -5
```

**Generic questions to ask**:

- Are there separate client/worker processes?
- How do external components connect to the main system?
- What protocols are used (HTTP, WebSocket, gRPC, message queues)?
- Are connections initiated by clients or servers?

**HelixML example**:

- `runner-cmd/helix-runner/runner_cmd.go` - Runner CLI entry point
- Runners connect TO the control plane (reverse connection)
- Runners can be external (separate machines/containers)
- Communication via WebSocket connections

### 4. Map Frontend/UI Components

**Generic approach**: Find and understand user-facing interfaces

```bash
# Find frontend/UI directories
find . -maxdepth 2 -type d -name "*frontend*" -o -name "*ui*" -o -name "*web*" -o -name "*client*" -o -name "*app*"

# Look for common frontend frameworks
find . -name "package.json" -o -name "requirements.txt" -o -name "Gemfile" -o -name "build.gradle"

# Find static assets and templates
find . -name "*.html" -o -name "*.css" -o -name "*.js" -o -name "*.tsx" -o -name "*.vue" | head -10

# Look for API integration patterns
grep -r "api\|endpoint\|fetch\|axios\|request" --include="*.js" --include="*.ts" --include="*.tsx" . | head -5
```

**Generic questions to ask**:

- What frontend framework is used (React, Vue, Angular, etc.)?
- How does the frontend communicate with the backend?
- Are there real-time features (WebSocket, SSE)?
- Is the frontend served by the main server or separately?

**HelixML example**:

- `frontend/src/` - React SPA
- Served by the control plane on port 8080
- Real-time updates via WebSocket
- API integration through HTTP calls to `/api/v1/`

## Generic Architecture Discovery Steps

### 1. Find Configuration and Environment Variables

```bash
# Find configuration files (any language/framework)
find . -name "*config*" -o -name "*.env*" -o -name "*settings*" -o -name "*.ini" -o -name "*.toml" | head -10

# Look for environment variable usage patterns
grep -r "getenv\|ENV\|process\.env\|os\.environ" --include="*.go" --include="*.py" --include="*.js" --include="*.java" . | head -5

# Find property/configuration files
find . -name "*.properties" -o -name "*.yml" -o -name "*.yaml" -o -name "application.*" | head -5
```

**Generic questions to ask**:

- What can be configured?
- What are the default ports/endpoints?
- What external services are referenced?
- Are there environment-specific configs?

### 2. Discover Network Communication Patterns

```bash
# Find network-related code (any language)
grep -r "websocket\|socket\|grpc\|http\|tcp\|udp" --include="*.go" --include="*.py" --include="*.js" --include="*.java" . | head -10

# Look for API routes and endpoints
grep -r "route\|endpoint\|handler\|controller" --include="*.go" --include="*.py" --include="*.js" . | head -5

# Find port configurations
grep -r ":8080\|:3000\|:5000\|:9000\|PORT\|port" --include="*.go" --include="*.py" --include="*.js" . | head -5
```

**Generic questions to ask**:

- What protocols are used for communication?
- Who initiates connections (client vs server)?
- Are there real-time communication channels?
- What are the main API endpoints?

### 3. Understand Data Storage and Models

```bash
# Find database/storage related code
find . -name "*model*" -o -name "*schema*" -o -name "*store*" -o -name "*repository*" -o -name "*dao*" | head -10

# Look for ORM/database patterns
grep -r "SELECT\|INSERT\|database\|db\|sql\|mongo\|redis" --include="*.go" --include="*.py" --include="*.js" --include="*.java" . | head -5

# Find migration files
find . -name "*migration*" -o -name "*migrate*" -o -path "*/migrations/*" | head -5
```

**Generic questions to ask**:

- What type of database is used?
- What are the main data entities?
- How is data accessed (ORM, raw SQL, etc.)?
- Are there data migration patterns?

**HelixML example**:

- PostgreSQL for main data storage
- Key entities: Sessions, Runners, RunnerSlots, ApiKeys
- Uses GORM for database access

## How to Trace Complete Data Flows

### Generic Approach: Pick a Key User Action

**Method**: Choose a core user action and trace it end-to-end

```bash
# Find request handling patterns
grep -r "handler\|route\|endpoint" --include="*.go" --include="*.py" --include="*.js" . | head -5

# Look for middleware and request processing
grep -r "middleware\|interceptor\|filter" --include="*.go" --include="*.py" --include="*.js" . | head -5

# Find background processing patterns
grep -r "queue\|job\|task\|async\|background" --include="*.go" --include="*.py" --include="*.js" . | head -5
```

**Generic tracing questions**:

1. Where does the user action start? (UI/API endpoint)
2. How is the request validated and processed?
3. What business logic is triggered?
4. How does data flow between components?
5. Where is state stored/updated?
6. How does the response flow back?

### HelixML Example: "How does a user query get processed?"

1. **User submits query** → Frontend (`frontend/src/`)
2. **Frontend sends API request** → Control Plane API (`api/cmd/helix/serve.go`)
3. **Control plane schedules work** → Scheduler (`api/pkg/scheduler/`)
4. **Scheduler assigns to runner** → Runner Management (`api/pkg/scheduler/runner.go`)
5. **Runner receives work via WebSocket** → Runner WebSocket (`api/pkg/server/websocket_*`)
6. **Runner processes with model** → External runner process
7. **Response flows back** → Same path in reverse

## Common Architectural Patterns to Watch For

### 1. Connection Direction and Initialization

**Questions to ask**:

- Who initiates connections between components?
- Are there reverse proxy or reverse connection patterns?
- How do components discover each other?
- What happens when connections are lost?

```bash
# Look for connection initialization patterns
grep -r "connect\|dial\|listen\|accept" --include="*.go" --include="*.py" --include="*.js" . | head -5
```

### 2. Dynamic Resource Management

**Questions to ask**:

- Are resources (workers, instances, containers) created dynamically?
- How is resource lifecycle managed?
- What triggers resource creation/destruction?
- How is resource state tracked?

```bash
# Look for lifecycle management patterns
grep -r "create\|destroy\|start\|stop\|lifecycle" --include="*.go" --include="*.py" --include="*.js" . | head -5
```

### 3. Security and Authentication Boundaries

**Questions to ask**:

- Where does authentication happen?
- How are different security domains separated?
- What tokens/keys are required?
- How are permissions enforced?

```bash
# Find authentication patterns
grep -r "auth\|token\|permission\|security" --include="*.go" --include="*.py" --include="*.js" . | head -5
```

**HelixML specific examples**:

- Runners connect TO the control plane (reverse connection)
- Models are dynamically loaded/unloaded on runners
- Runner authentication happens during WebSocket connection

## Universal Architecture Discovery Toolkit

### 1. Language-Agnostic Exploration Commands

```bash
# Find all potential entry points
find . -name "main.*" -o -name "app.*" -o -name "server.*" -o -name "index.*" -o -name "manage.*"

# Find service and handler definitions
grep -r "serve\|handler\|controller\|router" --include="*.go" --include="*.py" --include="*.js" --include="*.java" . | head -10

# Find network communication patterns
grep -r "websocket\|socket\|http\|grpc" --include="*.go" --include="*.py" --include="*.js" --include="*.java" . | head -10

# Find data models and schemas
find . -name "*model*" -o -name "*schema*" -o -name "*entity*" -o -name "*dto*" | head -10
```

### 2. Universal Configuration Discovery

```bash
# Find all configuration files
find . -name "*.env*" -o -name "*config*" -o -name "*.ini" -o -name "*.toml" -o -name "*.properties" -o -name "*.yaml" -o -name "*.yml"

# Find environment variable patterns (multiple languages)
grep -r "getenv\|ENV\|process\.env\|os\.environ\|System\.getenv" --include="*.go" --include="*.py" --include="*.js" --include="*.java" . | head -10

# Find port and network configurations
grep -r "PORT\|:8080\|:3000\|:5000\|:9000\|localhost\|127\.0\.0\.1" --include="*.go" --include="*.py" --include="*.js" --include="*.java" . | head -10
```

### 3. Universal Data Flow Analysis

```bash
# Find routing patterns (any framework)
grep -r "route\|endpoint\|path\|url" --include="*.go" --include="*.py" --include="*.js" --include="*.java" . | head -10

# Find middleware and processing layers
grep -r "middleware\|interceptor\|filter\|decorator" --include="*.go" --include="*.py" --include="*.js" --include="*.java" . | head -5

# Find database/storage access patterns
grep -r "SELECT\|INSERT\|query\|find\|save\|create" --include="*.go" --include="*.py" --include="*.js" --include="*.java" . | head -10
```

### 4. Dependency and Service Discovery

```bash
# Find dependency management files
find . -name "package.json" -o -name "requirements.txt" -o -name "pom.xml" -o -name "build.gradle" -o -name "Cargo.toml" -o -name "go.mod"

# Find Docker and deployment configurations
find . -name "Dockerfile*" -o -name "docker-compose*.y*ml" -o -name "*.k8s.yaml" -o -name "deployment.yaml"

# Find API documentation
find . -name "*api*" -o -name "*swagger*" -o -name "*openapi*" -o -name "*.postman*"
```

## Quick Architecture Verification Checklist

To verify your understanding of ANY software system, try to answer these universal questions:

### Core System Questions

1. **Where does the main application server start?** (Find main entry point)
2. **What port(s) does the system listen on?** (Check configs and code)
3. **What is the primary database/storage system?** (Check dependencies and configs)
4. **How do external components connect to the system?** (Find connection patterns)
5. **What are the main API endpoints or routes?** (Find routing definitions)

### Communication Questions

6. **Who initiates connections between components?** (Client vs server patterns)
7. **What protocols are used for inter-component communication?** (HTTP, WebSocket, gRPC, etc.)
8. **Are there real-time features?** (WebSocket, SSE, polling)
9. **How is authentication handled?** (Tokens, sessions, certificates)

### Data Flow Questions

10. **How does a typical user request flow through the system?** (Trace end-to-end)
11. **Where is application state stored?** (Database, cache, memory)
12. **Are there background workers or async processing?** (Jobs, queues, tasks)

### HelixML Example Answers

1. Main server: `api/cmd/helix/serve.go`
2. Primary port: 8080
3. Database: PostgreSQL
4. External connection: Runners connect via WebSocket
5. API routes: `/api/v1/*` endpoints
6. Connection initiation: Runners connect TO control plane
7. Protocols: HTTP API + WebSocket for real-time
8. Real-time: Yes, WebSocket for runner communication
9. Authentication: Runner tokens + Keycloak for users
10. User flow: Frontend → API → Scheduler → Runner → Response
11. State storage: PostgreSQL + in-memory runner state
12. Background work: Yes, runner scheduling and model management

## Next Steps for Any Project

1. **Set up the development environment** (use Docker/compose if available)
2. **Trace one complete user action** from UI to database and back
3. **Identify the "critical path"** - the most important user flows
4. **Map external dependencies** - what happens if each external service fails?
5. **Find the configuration that matters** - what can break the system?
6. **Understand the deployment model** - how does it run in production?

## General Architecture Discovery Principles

- **Start with the obvious** (main files, configs, documentation)
- **Follow the data** (trace how information flows through the system)
- **Understand the boundaries** (what's internal vs external, sync vs async)
- **Map the failure modes** (what can go wrong and how is it handled)
- **Look for the magic** (complex or non-obvious parts that make the system unique)

This systematic approach works for any complex software system, regardless of language, framework, or domain.
