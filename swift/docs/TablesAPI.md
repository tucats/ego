# TablesAPI

All URIs are relative to *http://localhost:8080/ego*

Method | HTTP request | Description
------------- | ------------- | -------------
[**createTable**](TablesAPI.md#createtable) | **PUT** /tables/{table} | Create a new table
[**listTables**](TablesAPI.md#listtables) | **GET** /tables | List all tables for which the user has access.
[**showTable**](TablesAPI.md#showtable) | **GET** /tables/{table} | Get existing metadata information for columns in a table.


# **createTable**
```swift
    open class func createTable(table: String, columns: ColumnCollection, completion: @escaping (_ data: ColumnCollection?, _ error: Error?) -> Void)
```

Create a new table

### Example
```swift
// The following code samples are still beta. For any issue, please report via http://github.com/OpenAPITools/openapi-generator/issues/new
import OpenAPIClient

let table = "table_example" // String | The name of the table to create
let columns = ColumnCollection(columns: [Column(name: "name_example", type: "type_example", nullable: false, unqiue: false)], count: 123) // ColumnCollection | The array of column types to create for the new table.

// Create a new table
TablesAPI.createTable(table: table, columns: columns) { (response, error) in
    guard error == nil else {
        print(error)
        return
    }

    if (response) {
        dump(response)
    }
}
```

### Parameters

Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------
 **table** | **String** | The name of the table to create | 
 **columns** | [**ColumnCollection**](ColumnCollection.md) | The array of column types to create for the new table. | 

### Return type

[**ColumnCollection**](ColumnCollection.md)

### Authorization

No authorization required

### HTTP request headers

 - **Content-Type**: application/json
 - **Accept**: application/json

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)

# **listTables**
```swift
    open class func listTables(completion: @escaping (_ data: TableCollection?, _ error: Error?) -> Void)
```

List all tables for which the user has access.

### Example
```swift
// The following code samples are still beta. For any issue, please report via http://github.com/OpenAPITools/openapi-generator/issues/new
import OpenAPIClient


// List all tables for which the user has access.
TablesAPI.listTables() { (response, error) in
    guard error == nil else {
        print(error)
        return
    }

    if (response) {
        dump(response)
    }
}
```

### Parameters
This endpoint does not need any parameter.

### Return type

[**TableCollection**](TableCollection.md)

### Authorization

No authorization required

### HTTP request headers

 - **Content-Type**: Not defined
 - **Accept**: application/json

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)

# **showTable**
```swift
    open class func showTable(table: String, start: Int? = nil, limit: Int? = nil, completion: @escaping (_ data: ColumnCollection?, _ error: Error?) -> Void)
```

Get existing metadata information for columns in a table.

### Example
```swift
// The following code samples are still beta. For any issue, please report via http://github.com/OpenAPITools/openapi-generator/issues/new
import OpenAPIClient

let table = "table_example" // String | The name of the table to display
let start = 987 // Int | Starting row of result set to return (optional)
let limit = 987 // Int | Limit on number of rows to return (optional)

// Get existing metadata information for columns in a table.
TablesAPI.showTable(table: table, start: start, limit: limit) { (response, error) in
    guard error == nil else {
        print(error)
        return
    }

    if (response) {
        dump(response)
    }
}
```

### Parameters

Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------
 **table** | **String** | The name of the table to display | 
 **start** | **Int** | Starting row of result set to return | [optional] 
 **limit** | **Int** | Limit on number of rows to return | [optional] 

### Return type

[**ColumnCollection**](ColumnCollection.md)

### Authorization

No authorization required

### HTTP request headers

 - **Content-Type**: Not defined
 - **Accept**: application/json

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)

