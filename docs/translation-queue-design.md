# Translation Queue Design

## Overview

This document outlines the design for the translation queue workflow, from user selection in the UI through to gRPC communication with the Nanabush vLLM service.

## Issues to Address

### Template Detection Issue
- **Problem**: Templates are showing up inconsistently in the UI. Outline has templates in a separate "Templates" section, but we're also seeing template documents in collections.
- **Current Behavior**: We detect templates by checking if the title contains "Template", but this doesn't distinguish between:
  - Actual template definitions (in Outline's Templates section)
  - Documents created FROM templates (in Collections)
- **Solution Needed**: 
  - Check Outline API for template metadata (if available)
  - Better detection logic to identify actual templates vs template-derived documents
  - Filter templates from translation queue (or mark them appropriately)

## Translation Queue Workflow

### Step 1: Validation & Pre-flight Checks

When a user queues a page for translation:

1. **Template Check**
   - Verify the page is NOT a template (templates shouldn't be translated directly)
   - If it's a template, reject with clear error message
   - If it's a document created from a template, proceed but note the template reference

2. **Destination Validation**
   - Check if destination WikiTarget exists and is accessible
   - Verify destination mode allows writes (ReadWrite or PushOnly)
   - Validate destination path prefix is valid
   - Test write permissions (if possible via API)

3. **Duplicate Check**
   - Query destination wiki for existing page with same title/slug
   - If duplicate found:
     - Set job state to `AwaitingApproval`
     - Store duplicate page info in job status
     - Wait for user approval via UI
   - If no duplicate, proceed

### Step 2: Title-Only Pre-flight to Nanabush

Before fetching full content, send a lightweight request to Nanabush:

**Purpose**: Quick validation that Nanabush is ready and can handle the request

**Payload**:
```protobuf
message TitleCheckRequest {
  string title = 1;
  string language_tag = 2;
  string source_language = 3;  // e.g., "EN"
}
```

**Response**:
```protobuf
message TitleCheckResponse {
  bool ready = 1;
  string message = 2;
  int32 estimated_time_seconds = 3;
}
```

### Step 3: Content Fetching (On-the-fly, No Storage)

Once pre-flight passes:

1. **Fetch Source Page Content**
   - Use Outline API to export page as Markdown
   - Outline API endpoint: `/api/documents.export` or `/api/documents.info` (check which provides full content)
   - Stream content directly into memory (no disk writes)
   - Extract metadata: title, slug, collection, template reference

2. **Fetch Template Helper (if available)**
   - If page was created from a template (template field is set):
     - Fetch the template document from Outline
     - Export template as Markdown
     - This provides context to the vLLM about structure/format

### Step 4: gRPC Payload to Nanabush

**Communication Method**: gRPC with mTLS

**Why gRPC?**
- **Performance**: Binary protocol, HTTP/2 multiplexing, lower latency than REST
- **Type Safety**: Protocol buffers provide strong typing
- **Streaming**: Can stream large content if needed
- **Security**: Built-in TLS/mTLS support
- **Alternatives Considered**:
  - REST/JSON: Simpler but slower, larger payloads
  - WebSocket: Good for streaming but more complex state management
  - **Decision**: gRPC is optimal for inter-service communication in Kubernetes

**Security Requirements**:
- mTLS (mutual TLS) for authentication
- Certificate-based handshake
- Service mesh integration (Istio/OSM) for additional security layers
- Network policies to restrict access to nanabush namespace only

**Payload Structure**:

```protobuf
syntax = "proto3";

package nanabush.v1;

import "google/protobuf/timestamp.proto";

service TranslationService {
  // Pre-flight check with title only
  rpc CheckTitle(TitleCheckRequest) returns (TitleCheckResponse);
  
  // Full translation request
  rpc Translate(TranslateRequest) returns (TranslateResponse);
  
  // Stream translation (for large documents)
  rpc TranslateStream(stream TranslateChunk) returns (stream TranslateChunk);
}

enum PrimitiveType {
  PRIMITIVE_UNSPECIFIED = 0;
  PRIMITIVE_TITLE = 1;        // Title-only translation
  PRIMITIVE_DOC_TRANSLATE = 2; // Full document translation
}

message TranslateRequest {
  // Job identification
  string job_id = 1;
  string namespace = 2;
  
  // Primitive type
  PrimitiveType primitive = 3;
  
  // Source content
  oneof source {
    string title = 4;           // For PRIMITIVE_TITLE
    DocumentContent doc = 5;    // For PRIMITIVE_DOC_TRANSLATE
  }
  
  // Template helper (optional)
  DocumentContent template_helper = 6;
  
  // Translation parameters
  string source_language = 7;   // e.g., "EN"
  string target_language = 8;  // e.g., "fr-CA" (BCP 47)
  
  // Metadata
  string source_wiki_uri = 9;
  string page_id = 10;
  string page_slug = 11;
  google.protobuf.Timestamp requested_at = 12;
}

message DocumentContent {
  string title = 1;
  string markdown = 2;
  string slug = 3;
  map<string, string> metadata = 4;  // Collection, template, etc.
}

message TranslateResponse {
  string job_id = 1;
  bool success = 2;
  string translated_title = 3;
  string translated_markdown = 4;
  string error_message = 5;
  google.protobuf.Timestamp completed_at = 6;
  int32 tokens_used = 7;
  double inference_time_seconds = 8;
}
```

## Implementation Plan

### Phase 1: Outline Client Enhancements

1. **Add `GetPageContent(pageID string)` method**
   - Fetches full page content as Markdown
   - Returns: title, markdown content, metadata

2. **Add `GetTemplate(templateID string)` method**
   - Fetches template document if template ID is available
   - Returns template markdown for context

3. **Improve Template Detection**
   - Check Outline API for template metadata
   - Distinguish between template definitions and template-derived documents
   - Update `PageSummary` to include `IsTemplate` boolean

### Phase 2: TranslationJob Controller Updates

1. **Pre-flight Validation**
   - Add validation logic in `Reconcile` method
   - Check template status
   - Validate destination
   - Check for duplicates

2. **New Job States**
   - `AwaitingApproval`: Waiting for user confirmation on duplicate
   - `Validating`: Running pre-flight checks
   - `FetchingContent`: Pulling source content
   - `Dispatching`: Sending to Nanabush

3. **Content Fetching**
   - Integrate Outline client `GetPageContent`
   - Stream content directly (no disk)
   - Fetch template helper if available

### Phase 3: gRPC Client Implementation

1. **Create `pkg/nanabush/client.go`**
   - gRPC client wrapper
   - mTLS configuration
   - Connection pooling
   - Retry logic

2. **Implement Service Interface**
   - `CheckTitle()` for pre-flight
   - `Translate()` for full translation
   - Error handling and timeouts

3. **Update Dispatcher**
   - Replace current TektonJobDispatcher with gRPC-based dispatcher
   - Or create new `GRPCDispatcher` alongside existing one

### Phase 4: Nanabush Integration

1. **Create gRPC Service Definition**
   - Define `.proto` file in `infra/nanabush/proto/`
   - Generate Go/Python stubs

2. **Implement Nanabush Service**
   - Handle `CheckTitle` requests
   - Handle `Translate` requests
   - Orchestrate vLLM backend calls
   - Return translated content

3. **Security Hardening**
   - mTLS certificates
   - Service mesh integration
   - Network policies

## Data Flow

```
User selects pages in UI
  ↓
UI calls POST /api/v1/jobs
  ↓
Operator creates TranslationJob CR
  ↓
TranslationJob Controller Reconcile:
  1. Validate (template check, destination check)
  2. Check for duplicates → AwaitingApproval if found
  3. Title-only pre-flight to Nanabush
  4. Fetch source content (on-the-fly)
  5. Fetch template helper (if available)
  6. gRPC Translate() to Nanabush
  7. Receive translated content
  8. Publish to destination (if mode allows)
  9. Update job status to Completed
```

## Security Considerations

1. **No Data at Rest**: Content is streamed directly from Outline → Operator → Nanabush → Destination
2. **mTLS**: All gRPC calls use mutual TLS
3. **Network Isolation**: NetworkPolicies restrict traffic to nanabush namespace
4. **Audit Trail**: All translation requests logged with OTEL traces
5. **Content Validation**: Nanabush validates responses don't contain external references

## Next Steps

1. Research Outline API for:
   - Full page content export endpoint
   - Template metadata/identification
   - Duplicate detection methods

2. Create gRPC proto definitions
3. Implement Outline client enhancements
4. Update TranslationJob controller
5. Build gRPC client for Nanabush
6. Test end-to-end flow

