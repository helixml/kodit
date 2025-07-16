---
title: REST API Reference
description: Kodit REST API Documentation
weight: 30
---



# Base URL

| URL | Description |
|-----|-------------|


# Authentication



## Security Schemes

| Name              | Type              | Description              | Scheme              | Bearer Format             |
|-------------------|-------------------|--------------------------|---------------------|---------------------------|
| APIKeyHeader | apiKey |  |  |  |

# APIs

## GET /

Root

Return a welcome message for the kodit API.




### Responses

#### 200


Successful Response


object







#### 500


Internal server error




## GET /healthz

Healthz

Return a health check for the kodit API.




### Responses

#### 200


Successful Response


object







#### 500


Internal server error




## GET /api/v1/indexes

List Indexes

List all indexes.




### Responses

#### 200


Successful Response


[IndexListResponse](#indexlistresponse)







#### 500


Internal server error




#### 401


Unauthorized




#### 422


Invalid request




## POST /api/v1/indexes

Create Index

Create a new index and start async indexing.




### Request Body

[IndexCreateRequest](#indexcreaterequest)







### Responses

#### 202


Successful Response


[IndexResponse](#indexresponse)







#### 500


Internal server error




#### 401


Unauthorized




#### 422


Invalid request




## GET /api/v1/indexes/{index_id}

Get Index

Get index details.


### Parameters

| Name | Type | Required | Description |
|------|------|----------|-------------|
| index_id | integer | True |  |


### Responses

#### 200


Successful Response


[IndexDetailResponse](#indexdetailresponse)







#### 500


Internal server error




#### 401


Unauthorized




#### 422


Invalid request




#### 404


Index not found




## DELETE /api/v1/indexes/{index_id}

Delete Index

Delete an index.


### Parameters

| Name | Type | Required | Description |
|------|------|----------|-------------|
| index_id | integer | True |  |


### Responses

#### 204


Successful Response




#### 500


Internal server error




#### 401


Unauthorized




#### 422


Invalid request




#### 404


Index not found




## POST /api/v1/search

Search Snippets

Search code snippets with filters matching MCP tool.




### Request Body

[SearchRequest](#searchrequest)







### Responses

#### 200


Successful Response


[SearchResponse](#searchresponse)







#### 500


Internal server error




#### 422


Validation Error


[HTTPValidationError](#httpvalidationerror)







# Components



## HTTPValidationError



| Field | Type | Description |
|-------|------|-------------|
| detail | array |  |


## IndexAttributes


Index attributes for JSON:API responses.


| Field | Type | Description |
|-------|------|-------------|
| created_at | string |  |
| updated_at | string |  |
| uri | string |  |


## IndexCreateAttributes


Attributes for creating an index.


| Field | Type | Description |
|-------|------|-------------|
| uri | string | URI of the source to index |


## IndexCreateData


Data for creating an index.


| Field | Type | Description |
|-------|------|-------------|
| type | string |  |
| attributes |  |  |


## IndexCreateRequest


JSON:API request for creating an index.


| Field | Type | Description |
|-------|------|-------------|
| data |  |  |


## IndexData


Index data for JSON:API responses.


| Field | Type | Description |
|-------|------|-------------|
| type | string |  |
| id | string |  |
| attributes |  |  |


## IndexDetailResponse


JSON:API response for index details with included resources.


| Field | Type | Description |
|-------|------|-------------|
| data |  |  |


## IndexListResponse


JSON:API response for index list.


| Field | Type | Description |
|-------|------|-------------|
| data | array |  |


## IndexResponse


JSON:API response for single index.


| Field | Type | Description |
|-------|------|-------------|
| data |  |  |


## SearchAttributes


Search attributes for JSON:API requests.


| Field | Type | Description |
|-------|------|-------------|
| keywords |  | Search keywords |
| code |  | Code search query |
| text |  | Text search query |
| limit |  | Maximum number of results to return |
| filters |  | Search filters |


## SearchData


Search data for JSON:API requests.


| Field | Type | Description |
|-------|------|-------------|
| type | string |  |
| attributes |  |  |


## SearchFilters


Search filters for JSON:API requests.


| Field | Type | Description |
|-------|------|-------------|
| languages |  | Programming languages to filter by |
| authors |  | Authors to filter by |
| start_date |  | Filter snippets created after this date |
| end_date |  | Filter snippets created before this date |
| sources |  | Source repositories to filter by |
| file_patterns |  | File path patterns to filter by |


## SearchRequest


JSON:API request for searching snippets.


| Field | Type | Description |
|-------|------|-------------|
| data |  |  |


## SearchResponse


JSON:API response for search results.


| Field | Type | Description |
|-------|------|-------------|
| data | array |  |


## SnippetAttributes


Snippet attributes for JSON:API responses.


| Field | Type | Description |
|-------|------|-------------|
| content | string |  |
| created_at | string |  |
| updated_at | string |  |
| original_scores | array |  |
| source_uri | string |  |
| relative_path | string |  |
| language | string |  |
| authors | array |  |
| summary | string |  |


## SnippetData


Snippet data for JSON:API responses.


| Field | Type | Description |
|-------|------|-------------|
| type | string |  |
| id | integer |  |
| attributes |  |  |


## ValidationError



| Field | Type | Description |
|-------|------|-------------|
| loc | array |  |
| msg | string |  |
| type | string |  |
