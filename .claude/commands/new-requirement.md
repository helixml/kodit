# Build Product Requirements Prompt

## Instructions

Create a comprehensive Product Requirement Prompt (PRP) with user-centered requirements gathering for: **$ARGUMENTS**

## Intelligent Workflow Selection

**Analyze `$ARGUMENTS` complexity and choose approach:**

- **Simple/Clear Request** (e.g., "add a login button"): Skip to Phase 3 (Research & PRP Generation)
- **Moderate Request** (e.g., "user authentication system"): Start with Phase 2 (Targeted Questions)
- **Complex/Vague Request** (e.g., "improve user experience"): Start with Phase 1 (Discovery)

## Full Workflow

### Phase 1: Discovery Questions (Complex/Vague Requests Only)

Ask 5 strategic discovery questions to understand the problem space:

```markdown
I need to understand the broader context first. Let me ask 5 key questions:

## Discovery Questions

**Q1: Will users interact with this feature through a visual interface?**
Default: Yes (most features have UI components)

**Q2: Does this involve user data or require authentication?**
Default: Yes (better to plan for security)

**Q3: Do users currently have a workaround for this problem?**
Default: No (assuming this solves a new need)

**Q4: Will this need to integrate with external services or APIs?**
Default: No (simpler implementation)

**Q5: Is this a completely new feature or an enhancement to existing functionality?**
Default: Enhancement (most requests build on existing features)

*You can answer any/all questions or just say "continue" to use the defaults.*
```

### Phase 2: Technical Context Questions (Moderate+ Requests)

Ask 5 targeted technical questions informed by codebase analysis:

```markdown
Now let me ask about technical implementation details:

## Technical Questions

**Q6: [Specific question about existing similar features/components]**
Default: [Codebase-informed default]

**Q7: [Question about data layer/database requirements]**
Default: [Pattern-based default]

**Q8: [Question about API/service layer approach]**
Default: [Architecture-consistent default]

**Q9: [Question about testing/validation approach]**
Default: [Project-standard default]

**Q10: [Question about deployment/configuration needs]**
Default: [Environment-appropriate default]

*Answer what you know, or say "continue" for smart defaults based on your codebase.*
```

### Phase 3: Research & Context Gathering

**Automatically perform comprehensive research:**

1. **Web Research** (when beneficial):
   - Research best practices and current standards
   - Look up relevant library/framework documentation using mcp__kodit__search (if available)
   - Find example implementations and patterns using mcp__kodit__search (if available)
   - Gather security and performance considerations

2. **Codebase Analysis** (when tools available):
   - Analyze overall architecture and patterns
   - Identify similar existing features using mcp__kodit__search (if available)
   - Understand technology stack and conventions
   - Find integration points and dependencies

3. **Technical Context**:
   - Identify affected files and components
   - Understand data flow and business logic
   - Map out user interactions and workflows
   - Document technical constraints and opportunities

### Phase 4: Generate Comprehensive PRP

Create a complete Product Requirement Prompt that combines user requirements with
implementation guidance. Use the following template to create a new markdown file in
docs/requirements/YYYY-MM-DD-HHMM-[slug].md, where `[slug]` is a slug extracted from the
requirement and `YYYY-MM-DD-HHMM` is parsed from the system's `date` command.

---

## PRP Output Template

```markdown
# Product Requirement Prompt: [Feature Name]

**Generated:** [timestamp]  
**Original Request:** $ARGUMENTS  
**Complexity Level:** [Simple/Moderate/Complex]  
**Questions Asked:** [X/10]  

## Executive Summary

### Goal
[Concise statement of what this PRP aims to achieve based on discovery]

### Why
- **Business Value:** [Based on user needs discovered]
- **User Benefits:** [Who benefits and how]
- **Technical Benefits:** [System improvements]
- **Strategic Alignment:** [How it fits with existing functionality]

### What
**Core Functionality:**
[Detailed explanation based on all gathered context including:]
- Primary features and capabilities
- User interaction patterns
- System behaviors and rules
- Integration requirements
- Scope and boundaries

## Requirements Analysis

### User Requirements Summary
[Synthesis of discovery answers and user needs]

### Technical Requirements Summary  
[Synthesis of technical context and codebase analysis]

### Key Decisions Made
- [Decision 1]: [Reasoning based on questions/research]
- [Decision 2]: [Reasoning based on questions/research]
- [Decision 3]: [Reasoning based on questions/research]

## Architecture & Implementation

### Technical Overview
- **Architecture Pattern:** [Based on codebase analysis]
- **Technology Stack:** [Current stack + any additions needed]
- **Integration Points:** [Internal and external connections]
- **Data Flow:** [How information moves through the system]

### Directory Structure

**Current Relevant Structure:**
```

[Tree representation of current relevant files/directories]

```

**Proposed Changes:**
```

[Tree showing new/modified files with descriptions]
├── [existing_dir]/
│   ├── [existing_file.ext]
│   ├── [new_file.ext] - [purpose and contents description]
│   └── [modified_file.ext] - [changes needed]

```

### Files to Reference
- **[file_path]** (existing) - [How to use as reference/pattern]
- **[documentation_url]** (external) - [Relevant sections for implementation]
- **[similar_feature_path]** (existing) - [Pattern to follow/extend]
- **[library_docs_url]** (external) - [Specific methods/approaches to use]

### Implementation Specifications

#### [Component Category 1: e.g., Data Layer]
**File: `[file_path]`**
Purpose: [Specific role in the implementation]

```[language]
[Sample code structure or key implementation patterns]
// Key considerations:
// - [Specific requirement from discovery]
// - [Pattern to follow from codebase]
// - [Best practice from research]
```

#### [Component Category 2: e.g., Business Logic]

**File: `[file_path]`**
Purpose: [Specific role in the implementation]

```[language]
[Sample code structure or key implementation patterns]
// Key considerations:
// - [User requirement addressed]
// - [Integration with existing systems]
// - [Error handling patterns]
```

#### [Component Category 3: e.g., User Interface]

**File: `[file_path]`**
Purpose: [Specific role in the implementation]

```[language]
[Sample code structure or key implementation patterns]
// Key considerations:
// - [User experience requirements]
// - [Accessibility standards]
// - [Responsive design needs]
```

## API/Endpoints Design

### [Endpoint 1]

- **Method:** [GET/POST/PUT/DELETE]
- **Path:** [/api/path]
- **Purpose:** [What this endpoint accomplishes]
- **Parameters:**
  - `param1` (type) - [description]
  - `param2` (type) - [description]
- **Success Response:**

  ```json
  [Example response structure]
  ```

- **Error Handling:** [Specific error cases and responses]

### [Endpoint 2]

[Similar structure for additional endpoints]

## Implementation Guidelines

### [Technical Domain 1: e.g., Authentication & Security]

**Requirements from Discovery:**

- [Specific security requirement from questions]
- [User data protection needs]

**Implementation Approach:**

- [Specific security patterns to follow]
- [Authentication/authorization strategy]
- [Data validation and sanitization]
- [Error handling for security scenarios]

**Code References:**

- Follow patterns in `[existing_auth_file]`
- Use security helpers from `[security_utils_file]`

### [Technical Domain 2: e.g., Data Management]

**Requirements from Discovery:**

- [Data persistence needs]
- [Performance requirements]

**Implementation Approach:**

- [Database/storage strategy]
- [Data validation rules]
- [Performance optimization techniques]
- [Backup and recovery considerations]

**Code References:**

- Extend patterns from `[existing_data_layer]`
- Use ORM/database helpers from `[db_utils_file]`

### [Technical Domain 3: e.g., User Experience]

**Requirements from Discovery:**

- [UI/UX requirements from questions]
- [Accessibility needs]

**Implementation Approach:**

- [User interface design principles]
- [Interaction patterns to follow]
- [Responsive design requirements]
- [Error messaging and feedback]

**Code References:**

- Follow component patterns in `[ui_components_dir]`
- Use styling conventions from `[styles_dir]`

## Validation & Testing Strategy

### Functional Validation

**Based on User Requirements:**

- [ ] [Specific user workflow works as expected]
- [ ] [User interaction produces correct results]
- [ ] [Error scenarios are handled gracefully]

**Test Commands:**

```bash
[Specific test commands for functional validation]
```

### Technical Validation

**Based on Technical Requirements:**

- [ ] [Performance benchmark met: response < Xms]
- [ ] [Security standard implemented: authentication required]
- [ ] [Integration working: external API connection established]

**Test Commands:**

```bash
[Specific test commands for technical validation]
```

### User Acceptance Criteria

**Derived from Discovery Questions:**

- [ ] [Acceptance criterion based on Q1 answer]
- [ ] [Acceptance criterion based on Q2 answer]
- [ ] [Acceptance criterion based on Q3 answer]
- [ ] [Overall user satisfaction metric]

## Implementation Roadmap

### Checkpoint 1: Foundation Setup

**Tasks:**

- [ ] Set up basic project structure
- [ ] Implement core data models
- [ ] Create basic authentication framework

**Validation:**

```bash
[Test command to verify foundation]
```

**Expected:** [Specific expected outcome]

### Checkpoint 2: Core Feature Implementation

**Tasks:**

- [ ] Implement primary business logic
- [ ] Create user interface components
- [ ] Set up API endpoints

**Validation:**

```bash
[Test command to verify core features]
```

**Expected:** [Specific expected outcome]

### Checkpoint 3: Integration & Polish

**Tasks:**

- [ ] Integrate with existing systems
- [ ] Implement error handling and edge cases
- [ ] Performance optimization and testing

**Validation:**

```bash
[Test command to verify complete implementation]
```

**Expected:** [Specific expected outcome]

## Research References

### Best Practices Sources

- [URL/Resource 1]: [Key insights relevant to implementation]
- [URL/Resource 2]: [Specific techniques or patterns found]
- [URL/Resource 3]: [Standards or conventions to follow]

### Technical Documentation

- [Library/Framework Docs]: [Specific sections relevant to implementation]
- [API Documentation]: [Integration details and examples]
- [Security Guidelines]: [Relevant security considerations]

### Example Implementations

- [GitHub/StackOverflow URL]: [Pattern or approach to reference]
- [Code Example URL]: [Specific implementation technique]
- [Tutorial/Guide URL]: [Step-by-step process to follow]

## Risk Assessment & Mitigation

### Technical Risks

- **Risk:** [Potential technical challenge]
  **Likelihood:** [High/Medium/Low]
  **Impact:** [High/Medium/Low]  
  **Mitigation:** [Specific strategy to address]

### Business Risks

- **Risk:** [Potential business/user impact]
  **Likelihood:** [High/Medium/Low]
  **Impact:** [High/Medium/Low]
  **Mitigation:** [Specific strategy to address]

### Dependencies & Assumptions

**External Dependencies:**

- [Dependency 1]: [Version and reliability considerations]
- [Dependency 2]: [Integration complexity and fallback options]

**Key Assumptions:**

- [ASSUMED]: [Assumption based on unanswered questions]
- [ASSUMED]: [Technical assumption based on codebase patterns]
- [ASSUMED]: [Business assumption based on discovery defaults]

## Success Metrics & Monitoring

### Key Performance Indicators

- **[Metric 1]:** [Target value] - [How to measure]
- **[Metric 2]:** [Target value] - [How to measure]  
- **[Metric 3]:** [Target value] - [How to measure]

### Monitoring Strategy

- [What to track automatically]
- [User feedback collection methods]
- [Performance monitoring approach]
- [Error tracking and alerting]

### Success Definition

**MVP Success:**

- [ ] [Minimum viable implementation criteria]
- [ ] [Basic user satisfaction threshold]
- [ ] [Technical performance baseline]

**Full Success:**

- [ ] [Complete feature functionality]
- [ ] [Optimal user experience metrics]
- [ ] [Technical excellence standards]

---

## Quick Implementation Guide

### Getting Started (Copy/Paste Ready)

```bash
# 1. Set up development environment
[Specific setup commands based on tech stack]

# 2. Create basic structure  
[Commands to create necessary directories/files]

# 3. Implement core logic
[Key implementation steps in order]

# 4. Test implementation
[Test commands to verify each step]
```

### Code Patterns to Follow

**Pattern 1: [Name]**

```[language]
// Location: [existing_file_reference]
[Code example showing pattern to follow]
```

**Pattern 2: [Name]**  

```[language]
// Location: [existing_file_reference]
[Code example showing pattern to follow]
```

### Common Pitfalls & Solutions

- **Pitfall:** [Common mistake in this type of implementation]
  **Solution:** [Specific approach to avoid the mistake]
  **Example:** [Code or configuration example]

```

## Execution Strategy

### Request Analysis
1. **Parse `$ARGUMENTS`** for complexity indicators:
   - **Simple:** Contains specific technical terms, clear scope
   - **Moderate:** General feature description with some context  
   - **Complex:** Vague goals, broad scope, unclear requirements

2. **Determine Question Strategy:**
   - **Simple:** Skip to research and generation
   - **Moderate:** Ask 5 technical questions only
   - **Complex:** Ask all 10 questions (5 discovery + 5 technical)

### Research Integration
- **Always perform web research** for best practices and standards
- **Use codebase tools when available** for context and patterns
- **Synthesize findings** into actionable implementation guidance
- **Provide concrete examples** and reference materials

### Output Quality Assurance
- **Ensure actionability:** Every section should be implementable
- **Maintain specificity:** Use actual file paths, concrete examples
- **Balance comprehensiveness with clarity:** Cover all aspects without overwhelming
- **Validate completeness:** Check that PRP provides sufficient context for implementation
