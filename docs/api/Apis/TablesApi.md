# TablesApi

All URIs are relative to *http://localhost:8080*

Method | HTTP request | Description
------------- | ------------- | -------------
[**createTable**](TablesApi.md#createTable) | **POST** /tables | Create a table
[**listTables**](TablesApi.md#listTables) | **GET** /tables | List all tables
[**showTable**](TablesApi.md#showTable) | **GET** /tables/{table} | Info for a table


<a name="createTable"></a>
# **createTable**
> createTable()

Create a table

### Parameters
This endpoint does not need any parameter.

### Return type

null (empty response body)

### Authorization

No authorization required

### HTTP request headers

- **Content-Type**: Not defined
- **Accept**: application/json

<a name="listTables"></a>
# **listTables**
> List listTables()

List all tables

### Parameters
This endpoint does not need any parameter.

### Return type

[**List**](../Models/Table.md)

### Authorization

No authorization required

### HTTP request headers

- **Content-Type**: Not defined
- **Accept**: application/json

<a name="showTable"></a>
# **showTable**
> List showTable(table)

Info for a table

### Parameters

Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------
 **table** | **String**| The name of the table to display | [default to null]

### Return type

[**List**](../Models/Table.md)

### Authorization

No authorization required

### HTTP request headers

- **Content-Type**: Not defined
- **Accept**: application/json

