# VSAAS REST Framework

VSAAS REST Framework is a web framework built on Echo v4 for building REST APIs in Go. It includes features for authentication, authorization, auditing, request validation, structured logging, advanced filtering, file uploads, per-endpoint timeouts, and rate limiting.

## Main Features

- High-performance HTTP server (Echo v4)
- Static file serving with SPA (Single Page Application) support
- Extensible database connectors (MongoDB supported)
- Database-agnostic index management with automatic comparison and warnings
- Role-based authentication and authorization
- Automatic operation auditing
- Redis-based rate limiting
- Secure file uploads with validation
- Request validation with go-playground/validator
- LoopBack 3-compatible query filters
- Per-endpoint timeouts
- Structured logging (slog)
- Flexible HTTP header configuration for static files

## Installation

```bash
go mod init myapp
go get github.com/xompass/vsaas-rest
```

## Quick Example

```go
package main

import (
    rest "github.com/xompass/vsaas-rest"
)

func main() {
    // Create application
    app := rest.NewRestApp(rest.RestAppOptions{
        Name:              "My API",
        Port:              3000,
        LogLevel:          rest.LogLevelDebug,
    })

    endpoint:= &rest.Endpoint{
        Name:       "MyEndpoint",
        Method:     rest.MethodGET,
        Path:       "/my-endpoint",
        Handler:    func(ctx *rest.EndpointContext) error {
            return ctx.JSON(map[string]string{"message": "Hello, World!"})
        },
    }

    // Create route group
    api := app.Group("/api")

    // Register endpoint
    app.RegisterEndpoint(endpoint, api)

    // Start server
    err := app.Start()
    if err != nil {
        panic(err)
    }
}
```

## Database, Datasource, and Models

### Datasource and Connectors

The `Datasource` is the central component that centralizes database connections and model registration. A Datasource can have multiple connectors for different databases.

The `Connector` is responsible for establishing the connection to the specific database.

> **Note**: Currently the framework only supports MongoDB as a database engine. Support for other database engines **might** be implemented in future versions if needed.

#### Configure Datasource and Connectors

```go
import (
    "github.com/xompass/vsaas-rest/database"
)

func main() {
    // Option 1: Use default configuration
    // This uses environment variables:
    // MONGO_URI: MongoDB connection URI, default: `mongodb://localhost:27017`
    // MONGO_DATABASE: Database name. If not set, it will use the default database from the URI or "test".
    // The connector name will be "mongodb"
    mongoConnector, err := database.NewDefaultMongoConnector()
    if err != nil {
        panic(err)
    }

    // Option 2: Custom configuration
    // opts := &database.MongoConnectorOpts{
    //     ClientOptions: *options.Client().ApplyURI("mongodb://localhost:27017"),
    //     Name:          "mongodb",
    //     Database:      "my_database",
    // }
    // mongoConnector, err := database.NewMongoConnector(opts)
    // The `NewMongoConnector` function creates the connector and establishes the database connection.

    // Create datasource and add connector
    datasource := database.Datasource{}
    datasource.AddConnector(mongoConnector)

    // Use in application
    app := rest.NewRestApp(rest.RestAppOptions{
        Datasource: &datasource,
        // ... other options
    })
}
```

### Models

#### IModel Interface

The `IModel` interface defines the methods that all models must implement. This allows the framework to handle CRUD operations generically and flexibly.

```go
type IModel interface {
    GetTableName() string     // Name of the table/collection in the database
    GetModelName() string     // Unique model name for registration
    GetConnectorName() string // Name of the database connector to use
    GetId() any              // ID of the document/record
}
```

#### Creating a Model

Complete example of a model that implements `IModel`:

```go
package main

import (
    "time"
    "go.mongodb.org/mongo-driver/v2/bson"
    "github.com/xompass/vsaas-rest/database"
)

type Product struct {
    ID       bson.ObjectID `json:"id" bson:"_id,omitempty"`
    Name     string        `json:"name" bson:"name"`
    Price    float64       `json:"price" bson:"price"`

    // Automatic fields (optional)
    Created  *time.Time `json:"created,omitempty" bson:"created,omitempty"`
    Modified *time.Time `json:"modified,omitempty" bson:"modified,omitempty"`
    Deleted  *time.Time `json:"deleted,omitempty" bson:"deleted,omitempty"`
}

func (p Product) GetId() any               { return p.ID }
func (p Product) GetTableName() string     { return "products" }
func (p Product) GetModelName() string     { return "Product" }
func (p Product) GetConnectorName() string { return "mongodb" }

// Optional hooks
func (p *Product) BeforeCreate() error { return nil }
```

### Repositories

#### Creating a Repository

```go
package main

import (
    "github.com/xompass/vsaas-rest/database"
)

type ProductRepository database.Repository[Product]

func NewProductRepository(ds *database.Datasource) (ProductRepository, error) {
    // Create repository with options
    repository, err := database.NewMongoRepository[Product](ds, database.RepositoryOptions{
        Created:  true,  // Automatically adds the "created" field
        Modified: true,  // Automatically adds the "modified" field
        Deleted:  true,  // Enables soft delete with "deleted" field
    })

    if err != nil {
        return nil, err
    }

    return repository, nil
}
```

#### Registering Repositories

```go
package repositories

import (
    rest "github.com/xompass/vsaas-rest"
    "github.com/xompass/vsaas-rest/database"
)

func main(){
    // Create datasource and connectors
    datasource := database.Datasource{}
    mongoConnector, err := database.NewDefaultMongoConnector()
    if err != nil {
        panic(err)
    }
    datasource.AddConnector(mongoConnector)

    app := rest.NewRestApp(rest.RestAppOptions{
        Datasource: &datasource,
        // ... other options
    })

    _, err= NewProductRepository(&datasource)

    if err != nil {
        panic(err)
    }

    // Register endpoints and start server
}
```

#### Repository Operations

Repositories provide an interface for performing CRUD operations and queries on models. Here are some examples of how to use a repository:

```go
// Find all records
products, err := repo.Find(ctx, nil)

// Find with filter
filter := database.NewFilter().WithWhere(database.NewWhere().Gte("price", 100))
products, err := repo.Find(ctx, filter)

// Find one
filter = database.NewFilter().WithWhere(database.NewWhere().Eq("name", "Product A"))
product, err := repo.FindOne(ctx, filter)

// Find by ID
product, err := repo.FindById(ctx, productID, nil)

// Create
newProduct := Product{Name: "Product B", Price: 99.99}
insertID, err := repo.Insert(ctx, newProduct)

// Create and return the created document
createdItem, err := repo.Create(ctx, newProduct)

type UpdateItem struct {
    Name  string   `bson:"name"`
    Price *float64 `bson:"price,omitempty"` // omitempty makes this field not update if empty
}

// Update by ID
err = repo.UpdateById(ctx, itemID, UpdateItem{
    Name:  "Updated Name",
})

// Basic update one
// UpdateOne updates the first document that matches the filter and updates only the specified fields
err = repo.UpdateOne(ctx, filter, UpdateItem{
    Name:  "Updated Product",
})

// Find and update
// FindOneAndUpdate finds a document and updates it, returning the updated document
updatedItem, err := repo.FindOneAndUpdate(ctx, filter, UpdateItem{
    Name:  "Updated Product",
    Price: 19.99,
})

// Advanced update
// UpdateOne, UpdateById and FindOneAndUpdate allow for more complex updates with MongoDB operators
err = repo.UpdateOne(ctx, filter, bson.M{"$set": bson.M{"name": "Updated Product"}, "$inc": bson.M{"price": 10}})

// Count documents
count, err := repo.Count(ctx, filter)

// Check existence
exists, err := repo.Exists(ctx, itemID)

// Delete (soft delete if enabled)
err = repo.DeleteById(ctx, itemID)

// Delete multiple
deletedCount, err := repo.DeleteMany(ctx, filter)
```

### Database Indexes

The framework provides a database-agnostic way to define and manage indexes for your models. Currently, MongoDB is fully supported with all index types.

#### Defining Indexes

Implement the `MongoIndexableModel` interface in your model:

```go
import "github.com/xompass/vsaas-rest/database"

func (p Product) DefineMongoIndexes() []database.MongoIndexDefinition {
    return []database.MongoIndexDefinition{
        // Simple unique index
        database.NewMongoSimpleIndex("sku", true),

        // Compound index
        database.NewMongoCompoundIndex(
            "category_price_idx",
            []database.IndexField{
                {Name: "category", Order: 1},
                {Name: "price", Order: -1},
            },
            false,
        ),

        // Text search index
        database.NewMongoTextIndex("name_desc_text", []string{"name", "description"}).
            WithWeights(map[string]int32{"name": 10, "description": 5}).
            WithDefaultLanguage("spanish"),

        // Geospatial index (2dsphere)
        database.NewMongo2DSphereIndex("location"),

        // TTL index (auto-delete after 30 days)
        database.NewMongoTTLIndex("expiresAt", 30*24*time.Hour),

        // Compound TTL index (recommended for better query performance)
        database.NewMongoCompoundTTLIndex(
            "userId_expiresAt_ttl",
            []database.IndexField{
                {Name: "userId", Order: 1},
                {Name: "expiresAt", Order: 1},
            },
            24*time.Hour,
        ),

        // Advanced: Partial index with sparse option
        database.NewMongoCompoundIndex(
            "active_products_idx",
            []database.IndexField{
                {Name: "isActive", Order: 1},
                {Name: "price", Order: 1},
            },
            false,
        ).WithPartialFilter(map[string]any{
            "deleted": nil,
            "price": map[string]any{"$gt": 0},
        }).WithSparse(true),
    }
}
```

#### Available Index Types

- **Simple**: `NewMongoSimpleIndex(field, unique)`
- **Compound**: `NewMongoCompoundIndex(name, fields, unique)`
- **Text**: `NewMongoTextIndex(name, fields)`
- **TTL**: `NewMongoTTLIndex(field, duration)` or `NewMongoCompoundTTLIndex(name, fields, duration)`
- **Geospatial**: `NewMongo2DSphereIndex(field)`
- **Hashed**: `NewMongoHashedIndex(field)`

#### Fluent API Configuration

Chain methods to configure index options:

```go
database.NewMongoCompoundIndex(name, fields, false).
    WithTTL(90*24*time.Hour).                    // Add TTL
    WithPartialFilter(filter).                    // Partial index
    WithSparse(true).                             // Sparse index
    WithHidden(true).                             // Hidden from query planner
    WithCollation(&database.MongoCollation{...}). // Collation options
    WithWeights(weights)                          // Text search weights
```

#### Ensuring Indexes

Call `EnsureIndexes()` after registering all models:

```go
func main() {
    ds := &database.Datasource{}
    ds.AddConnector(mongoConnector)

    // Register your repositories here

    // Create indexes and compare with existing ones
    if err := ds.EnsureIndexes(); err != nil {
        log.Printf("Warning: Failed to ensure indexes: %v", err)
    }

    // Alternative: Ensure indexes for a specific model
    // ds.EnsureIndexesForModel(&Product{})
}
```

#### Index Comparison and Warnings

When `EnsureIndexes()` runs, it automatically compares defined indexes with existing ones in the database and logs warnings:

```
Index warnings for Product:
  [missing_in_code] Index 'old_index_1' exists in database but is not defined in code
  [missing_in_db] Index 'new_index_1' is defined in code but does not exist in database
  [different] Index 'sku_1' differs: unique constraint differs
Successfully ensured 8 indexes for Product: [sku_1 category_price_idx ...]
```

Warning types:

- `missing_in_code`: Index exists in DB but not in code definition
- `missing_in_db`: Index defined in code but not created in DB yet
- `different`: Index exists in both but with different options

### Filters and Queries

The framework provides an advanced filter system based on **LoopBack 3** syntax (Node.js framework) that allows creating complex queries both programmatically and through query parameters. This syntax is familiar to developers who have worked with LoopBack and provides a consistent and powerful interface for filtering data.

#### Field Names in Filters

When constructing filters, the field names used in `where`, `fields`, and `order` must match the names specified in the `json` tags of your model's struct. For example, if your model has a field defined as:

```go
type Product struct {
    Name  string `json:"name"`
    Price float64 `json:"price"`
}
```

You should use `"name"` and `"price"` in your filters, not the Go struct field names (`Name`, `Price`).

#### LoopBack 3 Compatibility

The `vsaas-rest` filter system is based on LoopBack 3, providing familiar and powerful syntax:

**Compatible features:**

- **where**: Condition filters with operators like `gt`, `lt`, `gte`, `lte`, `eq`, `neq`, `in`, `nin`, `like`, `nlike`
- **order**: Ascending/descending sorting by multiple fields
- **limit/skip**: Standard pagination
- **fields**: Field projection (include/exclude)

> Note: The `include` option has not been implemented yet

#### Using FilterBuilder Programmatically

```go
import "github.com/xompass/vsaas-rest/database"

func GetUsers(ctx *rest.EndpointContext) error {
    // Create filter programmatically
    filter := database.NewFilter().
        WithWhere(database.NewWhere().
            Eq("isActive", true).
            Gt("age", 18),
        ).
        OrderByAsc("name").
        Limit(10).
        Skip(20)

    repo, err := database.GetDatasourceModelRepository(ctx.App.Datasource, MyModel{})
    if err != nil {
        return rest.NewErrorResponse(500, "Database error")
    }

    items, err := repo.Find(ctx.Context(), filter)
    if err != nil {
        return rest.NewErrorResponse(500, "Failed to fetch items")
    }

    return ctx.RespondAndLog(users, nil, rest.ResponseTypeJSON)
}
```

#### Using Filters from Query Parameters

When you define a parameter of type `QueryParamTypeFilter`, the framework automatically parses the JSON and gives you a `FilterBuilder`:

```go
{
    Name:    "GetAllUsers",
    Method:  rest.MethodGET,
    Path:    "/users",
    Handler: GetUsers,
    Accepts: []rest.Param{
        rest.NewQueryParam("filter", rest.QueryParamTypeFilter),
        rest.NewQueryParam("search", rest.QueryParamTypeString),
        rest.NewQueryParam("limit", rest.QueryParamTypeInt),
    },
}

func GetUsers(ctx *rest.EndpointContext) error {
    // The framework already parsed the filter from query parameter
    filter, err := ctx.GetFilterParam()
    if err != nil {
        return err
    }

    // If no filter, create a new one
    if filter == nil {
        filter = database.NewFilter()
    }

    // Get other parsed parameters
    if search, ok := ctx.ParsedQuery["search"].(string); ok && search != "" {
        // Add text search to existing filter
        filter = filter.WithWhere(database.NewWhere().
            Like("name", search, "i"), // "i" for case-insensitive
        )
    }

    if limit, ok := ctx.ParsedQuery["limit"].(int); ok && limit > 0 {
        filter = filter.Limit(uint(limit))
    }

    repo, err := database.GetDatasourceModelRepository(ctx.App.Datasource, MyModel{})
    if err != nil {
        return rest.NewErrorResponse(500, "Database error")
    }

    items, err := repo.Find(ctx.Context(), filter)
    if err != nil {
        return rest.NewErrorResponse(500, "Failed to fetch items")
    }

    return ctx.RespondAndLog(items, nil, rest.ResponseTypeJSON)
}
```

#### Request Examples with Filters

```bash
# Basic filter
GET /api/items?filter={"where":{"isActive":true,"priority":{"gt":1}},"limit":10,"order":"name ASC"}

# Filter with additional search
GET /api/items?filter={"where":{"isActive":true}}&search=test&limit=5

# Complex filter with multiple conditions
GET /api/items?filter={"where":{"and":[{"priority":{"gte":1}},{"status":"active"}]},"order":"name ASC","limit":20,"skip":40}

# Only specific field projection
GET /api/items?filter={"fields":{"name":true,"description":true},"limit":10}
```

#### FilterBuilder API

**Construction methods:**

```go
filter := database.NewFilter()

// WHERE conditions
filter.WithWhere(database.NewWhere().Eq("status", "active"))
filter.WithWhere(database.NewWhere().Gt("age", 18))

// Sorting
filter.OrderByAsc("name")
filter.OrderByDesc("createdAt")

// Pagination
filter.Limit(10)
filter.Skip(20)
filter.Page(2, 10) // page 2, 10 elements per page

// Field projection
filter.Fields(map[string]bool{
    "name":  true,
    "email": true,
    "_id":   false, // exclude _id
})
```

#### WhereBuilder API

**Comparison operators:**

```go
where := database.NewWhere()

// Equality
where.Eq("status", "active")
where.Neq("status", "inactive")

// Numeric comparison
where.Gt("age", 18)
where.Gte("age", 18)
where.Lt("age", 65)
where.Lte("age", 65)

// Arrays
where.In("category", []string{"tech", "science"})
where.Nin("status", []string{"deleted", "banned"})

// Ranges
where.Between("age", 18, 65, false) // inclusive
where.Between("score", 80, 100, true) // exclusive

// Text search (regex)
where.Like("name", "john", "i") // case-insensitive

// Null values
where.IsNull("deletedAt")
where.IsNotNull("email")
```

**Logical operators:**

```go
// AND (automatically combined)
where := database.NewWhere().
    Eq("isActive", true).
    Gt("age", 18)

// Explicit OR
where := database.NewWhere().Or(
    database.NewWhere().Eq("role", "admin"),
    database.NewWhere().Eq("role", "moderator"),
)

// Complex combinations
where := database.NewWhere().
    Eq("isActive", true).
    Or(
        database.NewWhere().Eq("role", "admin"),
        database.NewWhere().And(
            database.NewWhere().Eq("role", "user"),
            database.NewWhere().Gt("score", 100),
        ),
    )
```

### Endpoints

To define endpoints in your API, you can use the framework's `Endpoint` structure. Each endpoint defines an HTTP method, a route, a handler, and other parameters like roles, action type, and validation.

### HTTP Methods Available

The framework supports the following HTTP methods:

```go
rest.MethodGET    // To get resources
rest.MethodPOST   // To create resources
rest.MethodPUT    // To update complete resources
rest.MethodPATCH  // To partially update resources
rest.MethodDELETE // To delete resources
rest.MethodHEAD   // To get headers without body
```

### Available Action Types

For auditing and logging, the framework defines several action types:

```go
rest.ActionTypeRead           // Read operations
rest.ActionTypeCreate         // Create operations
rest.ActionTypeUpdate         // Update operations
rest.ActionTypeDelete         // Delete operations
rest.ActionTypeLogin          // Login
rest.ActionTypeLogout         // Logout
rest.ActionTypeResetPassword  // Password reset
rest.ActionTypeChangePassword // Password change
rest.ActionTypeFileUpload     // File upload
```

```go
package main

import (
    rest "github.com/xompass/vsaas-rest"
)

// Define request structure
type CreateProductRequest struct {
    Name        string  `json:"name" validate:"required,min=2,max=100"`
    Price       float64 `json:"price" validate:"required,gte=0"`
}

func (r *CreateProductRequest) Validate(ctx *rest.EndpointContext) error {
    return ctx.ValidateStruct(r)
}

// Define endpoints
var myEndpoints = []*rest.Endpoint{
    {
        Name:       "GetAllProducts",
        Method:     rest.MethodGET,
        Path:       "/products",
        Handler:    GetProducts,
        Roles:      []rest.EndpointRole{PermReadProducts},
        ActionType: string(rest.ActionTypeRead),
        Model:      "Product",
        Accepts: []rest.Param{
            rest.NewQueryParam("filter", rest.QueryParamTypeFilter),
            rest.NewQueryParam("limit", rest.QueryParamTypeInt),
            rest.NewQueryParam("offset", rest.QueryParamTypeInt),
        },
    }
    {
        Name:       "CreateProduct",
        Method:     rest.MethodPOST,
        Path:       "/products",
        Handler:    CreateProduct,
        BodyParams: func() rest.Validable { return &CreateProductRequest{} },
        Roles:      []rest.EndpointRole{PermCreateProduct},
        ActionType: string(rest.ActionTypeCreate),
        Model:      "MyModel",
    }
}

func GetProducts(ctx *rest.EndpointContext) error {
    filter, err := ctx.GetFilterParam() // Get filter from query params
    if err != nil {
        return err
    }

    // Get product repository
    repo, err := database.GetDatasourceModelRepository(ctx.App.Datasource, Product{})
    if err != nil {
        return rest.NewErrorResponse(500, "Database error")
    }

    // Query data
    products, err := repo.Find(ctx.Context(), filter)
    if err != nil {
        return rest.NewErrorResponse(500, "Failed to fetch products")
    }

    // Read operation with auditing for traceability
    return ctx.JSON(products)
}

func CreateProduct(ctx *rest.EndpointContext) error {
    req := ctx.ParsedBody.(*CreateProductRequest)

    item := Product{
        Name:  req.Name,
        Price: req.Price,
    }

    repo, err := database.GetDatasourceModelRepository(ctx.App.Datasource, Product{})
    if err != nil {
        return rest.NewErrorResponse(500, "Database error")
    }

    result, err := repo.Insert(ctx.Context(), item)
    if err != nil {
        return rest.NewErrorResponse(500, "Failed to create item")
    }

    // Before responding, you can add auditing logic or other additional logic.
    // See Configure Auditing
    return ctx.RespondAndLog(result, result.InsertedID, rest.ResponseTypeJSON, 201)
}
```

### Configure Auditing

The framework includes a configurable auditing system that allows you to automatically log operations performed in your API.

#### AuditLogConfig

Auditing is configured through `AuditLogConfig`:

```go
app := rest.NewRestApp(rest.RestAppOptions{
    // ... other options
    AuditLogConfig: &rest.AuditLogConfig{
        Enabled: true,          // Enable auditing
        Handler: MyAuditHandler, // Custom function to handle auditing
    },
})
```

The `Handler` is a function you define to process audit information as you see fit:

```go
// Audit function that decides what to do with logs
func MyAuditHandler(ctx *rest.EndpointContext, response any, affectedModelId any) error {
    principal := ctx.Principal
    // You can implement any logic you need to execute before responding

    // Example audit logic:
    auditLog := map[string]any{
        "userId":         principal.GetPrincipalID(),
        "userType":       principal.GetPrincipalRole(),
        "endpointName":   ctx.Endpoint.Name,
        "actionType":     ctx.Endpoint.ActionType,
        "modelName":      ctx.Endpoint.Model,
        "modelId":        affectedModelId,
        "ipAddress":      ctx.IpAddress,
        "timestamp":      time.Now(),
    }

    // Save to database
    // Send to external system
    // Write to log file
    // Send to metrics/monitoring
    return nil
}
```

#### Endpoint Parameters

In the endpoint you can define the `Accepts` field to specify the parameters it accepts, including route, query and header parameters. The parameters defined in `Accepts` are automatically parsed and available in the context:

##### Path Parameters

```go
rest.NewPathParam("id", rest.PathParamTypeObjectID)
rest.NewPathParam("count", rest.PathParamTypeInt)
```

#### Query Parameters

```go
rest.NewQueryParam("filter", rest.QueryParamTypeFilter)          // LoopBack-style filter
rest.NewQueryParam("limit", rest.QueryParamTypeInt)              // Integer
rest.NewQueryParam("search", rest.QueryParamTypeString)          // String
rest.NewQueryParam("active", rest.QueryParamTypeBool)            // Boolean
rest.NewQueryParam("date", rest.QueryParamTypeDate)              // Date
```

#### Header Parameters

```go
rest.NewHeaderParam("X-Custom-Header", rest.HeaderParamTypeString, true) // Required
rest.NewHeaderParam("X-Request-ID", rest.HeaderParamTypeString, false)
```

#### Access to Parsed Parameters

```go
{
    Name: "MyEndpointWithParams",
    Path: "/:id",
    Accepts: []rest.Param{
        rest.NewPathParam("id", rest.PathParamTypeObjectID),
        rest.NewQueryParam("filter", rest.QueryParamTypeFilter),
    },
}

func GetUserPosts(ctx *rest.EndpointContext) error {
    // id is a route parameter, already parsed automatically
    id := ctx.ParsedPath["id"].(bson.ObjectID)

    // Build filter combining query param filter with other parameters
    filter, err := ctx.GetFilterParam() // or ctx.ParsedQuery["filter"]
    if err != nil {
        return err
    }

    if filter == nil {
        filter = database.NewFilter()
    }

    // Do something with id and filter
    return ctx.JSON(map[string]any{
        "id": id.Hex(),
        "filter": filter,
    })
}
```

### Error Handling

The framework provides a set of predefined error responses to handle common HTTP errors in a standardized way. These functions, located in the `http_errors` package, simplify error handling and ensure consistent error formats.

Instead of manually creating an `ErrorResponse` with a status code and message, you can use these helper functions:

```go
import "github.com/xompass/vsaas-rest/http_errors"

func GetProduct(ctx *rest.EndpointContext) error {
    product, err := repo.FindById(ctx.Context(), ctx.ParsedPath["id"])
    if err != nil {
        // Product not found
        return http_errors.NotFoundError("Product not found")
    }

    // User does not have permission
    if !user.HasPermission("read:products") {
        return http_errors.ForbiddenError("You don't have permission to access this resource")
    }

    return ctx.JSON(product)
}
```

**Available Error Functions:**

| Function                   | Status Code | Description                               |
| -------------------------- | ----------- | ----------------------------------------- |
| `BadRequestError`          | 400         | For malformed requests or invalid syntax. |
| `UnauthorizedError`        | 401         | When authentication is required.          |
| `ForbiddenError`           | 403         | When the user is not authorized.          |
| `NotFoundError`            | 404         | When a resource is not found.             |
| `ConflictError`            | 409         | For conflicts with the current state.     |
| `UnprocessableEntityError` | 422         | For validation errors.                    |
| `TooManyRequestsError`     | 429         | For rate limiting.                        |
| `InternalServerError`      | 500         | For unexpected server errors.             |

All error functions accept an optional `details` parameter to provide more context about the error.

### Body Processing

The framework provides powerful features for processing the request body, including normalization, sanitization, and validation. These operations are executed in the following order:

1.  **Normalization**
2.  **Sanitization**
3.  **Validation**

This ensures that incoming data is first cleaned and standardized before being validated against your business rules.

#### Body Normalization and Sanitization

The framework includes a powerful system for normalizing and sanitizing request body data before it reaches your handlers. This is done using struct tags, allowing you to define how each field should be processed.

**How to Use Tags:**

You can apply normalization and sanitization rules to your request body structs using the `normalize` and `sanitize` tags:

```go
type CreateUserRequest struct {
    Username string   `json:"username" normalize:"trim,lowercase" sanitize:"alphanumeric"`
    Comment  string   `json:"comment" normalize:"trim" sanitize:"html"`
    Website  string   `json:"website" normalize:"trim"`
    Tags     []string `json:"tags" normalize:"dive,trim,lowercase"`
}
```

**Available Processors:**

**Normalization:**

- `trim`: Removes leading and trailing whitespace.
- `lowercase`: Converts the string to lowercase.
- `uppercase`: Converts the string to uppercase.
- `unaccent`: Removes diacritics (accents) from the string.
- `unicode`: Normalizes the string to its NFC Unicode form.

**Sanitization:**

- `html`: Removes HTML tags using a strict policy (bluemonday's `UGCPolicy`).
- `alphanumeric`: Removes all non-alphanumeric characters.
- `numeric`: Removes all non-digit characters.

**Processing Nested Fields with `dive`:**

The `dive` tag is a special directive that allows you to apply processors to elements within slices, arrays, and maps.

- **For Slices and Arrays:** When `dive` is used on a slice of strings, the specified processors are applied to each string in the slice.
- **For Structs:** If a field is a struct or a slice of structs, `dive` will process the fields of the nested struct(s) according to their own tags.

**Registering Custom Functions:**

You can extend the framework's capabilities by registering your own custom normalization and sanitization functions using `RegisterBodyNormalizer` and `RegisterBodySanitizer`.

**Example:**

Let's say you want to create a custom sanitizer that removes all vowels from a string.

1.  **Define the function:**

    ```go
    import (
        "reflect"
        "strings"
        rest "github.com/xompass/vsaas-rest"
    )

    func removeVowels(v reflect.Value) {
        if v.Kind() == reflect.String {
            original := v.String()
            sanitized := strings.NewReplacer("a", "", "e", "", "i", "", "o", "", "u", "").Replace(original)
            v.SetString(sanitized)
        }
    }
    ```

2.  **Register the function:**

    ```go
    func main() {
        err := rest.RegisterBodySanitizer("novowels", removeVowels)
        if err != nil {
            panic(err)
        }
        // ... start your application
    }
    ```

3.  **Use it in your struct:**

    ```go
    type MyRequest struct {
        SomeText string `json:"some_text" sanitize:"novowels"`
    }
    ```

    Now, any request with the `MyRequest` body will have the `SomeText` field sanitized by removing all vowels.

#### Interface-based Processing

For more complex scenarios where simple tag-based processing is not enough, you can implement the `Normalizeable`, `Sanitizeable`, and `Validable` interfaces on your request body structs. Implementing any of these interfaces gives you **full control** over that specific step of the processing pipeline.

**When an interface is implemented, the corresponding tag-based processing for that step is automatically disabled.** This prevents unpredictable behavior from double-execution and ensures your custom logic is the single source of truth.

**Interfaces:**

```go
// If implemented, tag-based normalization is skipped.
type Normalizeable interface {
    Normalize(ctx *EndpointContext) error
}

// If implemented, tag-based sanitization is skipped.
type Sanitizeable interface {
    Sanitize(ctx *EndpointContext) error
}

// If implemented, tag-based validation is skipped.
type Validable interface {
    Validate(ctx *EndpointContext) error
}
```

**Execution Order:**

The body processing pipeline executes in the following order:

1.  **Normalization:**
    - If the struct implements `Normalizeable`, its `Normalize()` method is called.
    - Otherwise, standard tag-based normalization (`normalize:"..."`) is performed.
2.  **Sanitization:**
    - If the struct implements `Sanitizeable`, its `Sanitize()` method is called.
    - Otherwise, standard tag-based sanitization (`sanitize:"..."`) is performed.
3.  **Validation:**
    - If the struct implements `Validable`, its `Validate()` method is called.
    - Otherwise, standard tag-based validation (`validate:"..."`) is performed.

**Example:**

```go
type AdvancedRequest struct {
    Username string `json:"username" normalize:"trim,lowercase"`
    Data     string `json:"data"`
}

// Implement Sanitizeable for complex cleaning logic.
// Because this is implemented, any `sanitize` tags on AdvancedRequest will be ignored.
func (r *AdvancedRequest) Sanitize(ctx *rest.EndpointContext) error {
    // Example: Sanitize the 'Data' field based on user role
    if !isAdmin(ctx.Principal) {
        // Non-admins get stricter sanitization
        r.Data = myCustomSanitizationLogic(r.Data)
    }
    return nil
}
```

In the example above, the `Username` field will still be normalized based on its `normalize` tag, but the entire struct will only be sanitized by the `Sanitize` method.

#### Manual Tag-based Processing

If you need to trigger tag-based processing from within your own custom interface methods, you can use the helper functions on the `EndpointContext`:

- `ctx.NormalizeStruct(v any) error`
- `ctx.SanitizeStruct(v any) error`

This is useful when you want to perform some custom logic _before_ or _after_ the standard tag-based processing.

**Example:**

```go
func (r *AdvancedRequest) Sanitize(ctx *rest.EndpointContext) error {
    // 1. Perform some custom logic first
    if r.Data == "some_special_value" {
        r.Data = ""
    }

    // 2. Now, run the standard tag-based sanitizers
    if err := ctx.SanitizeStruct(r); err != nil {
        return err
    }

    // 3. Perform more custom logic after
    if len(r.Data) > 100 {
        r.Data = r.Data[:100]
    }

    return nil
}
```

#### Request Body Validation

The framework includes a request body validation system using the `Validable` interface and the `go-playground/validator` validator.

##### Validable Interface

All structs used as body parameters must implement the `Validable` interface:

```go
type Validable interface {
    Validate(ctx *EndpointContext) error
}
```

##### Basic Example

```go
type CreateProductRequest struct {
    Name     string `json:"name" validate:"required,min=3,max=100"`
    Price    float64 `json:"price" validate:"required,min=0"`
}

func (r *CreateProductRequest) Validate(ctx *rest.EndpointContext) error {
    err:= ctx.ValidateStruct(r)
    if err != nil {
        return err
    }
    // Here you can add additional validation logic if needed
    return nil
}

var endpoint = &rest.Endpoint{
    Name:       "CreateProduct",
    Method:     rest.MethodPOST,
    Path:       "/products",
    Handler:    CreateProduct,
    BodyParams: func() rest.Validable { return &CreateProductRequest{} },
}
```

How to use `go-playground/validator` can be found in the [official documentation](https://pkg.go.dev/github.com/go-playground/validator/v10).

### Authentication

The framework allows defining a custom authenticator function that runs before each endpoint that is not public. This function must implement authentication and authorization logic, returning a `Principal` and an `AuthToken`.

```go
// Example custom authorization function
func MyAuthorizer(ctx *rest.EndpointContext) (rest.Principal, rest.AuthToken, error) {
    // Get token from Authorization header
    token := ctx.EchoCtx.Request().Header.Get("Authorization")
    if token == "" {
        return nil, nil, rest.NewErrorResponse(401, "Authorization header required")
    }

    // Validate token (implement specific logic)
    principal, authToken, err := ValidateToken(token)
    if err != nil {
        return nil, nil, rest.NewErrorResponse(401, "Invalid token")
    }

    return principal, authToken, nil
}

// Implement interfaces
type MyPrincipal struct {
    ID   string
    Role string
}

// Implement the Principal interface
func (p *MyPrincipal) GetPrincipalID() string   { return p.ID }
func (p *MyPrincipal) GetPrincipalRole() string { return p.Role }

type MyAuthToken struct {
    Token     string
    UserId    string
    UserType  string
    ExpiresAt int64
}

// Implement the AuthToken interface
func (t *MyAuthToken) IsValid() bool       { return time.Now().Unix() < t.ExpiresAt }
func (t *MyAuthToken) GetUserId() string   { return t.UserId }
func (t *MyAuthToken) GetUserType() string { return t.UserType }
func (t *MyAuthToken) GetToken() string    { return t.Token }
func (t *MyAuthToken) GetExpiresAt() int64 { return t.ExpiresAt }
```

### File Upload

The framework supports secure file uploads with validation, size limits, and type restrictions. You can define file upload endpoints with specific configurations.

```go
{
    Name:    "UploadUserAvatar",
    Method:  rest.MethodPOST,
    Path:    "/users/:id/avatar",
    Handler: UploadAvatar,
    FileUploadConfig: &rest.FileUploadConfig{
        MaxFileSize:        10 * 1024 * 1024, // 10MB
        TypeSizeLimits:     map[rest.FileExtension]int64{
            ".jpg": 5 * 1024 * 1024,  // 5MB for JPG
            ".png": 10 * 1024 * 1024, // 10MB for PNG
        },
        UploadPath:         "./uploads/avatars",
        KeepFilesAfterSend: true,
    },
}

func UploadAvatar(ctx *rest.EndpointContext) error {
    files := ctx.UploadedFiles["avatar"]
    if len(files) == 0 {
        return rest.NewErrorResponse(400, "No avatar file provided")
    }

    file := files[0]

    // Process file...

    return ctx.RespondAndLog(map[string]string{
        "message": "Avatar uploaded successfully",
        "path":    file.SavedPath,
    }, ctx.ParsedPath["id"], rest.ResponseTypeJSON)
}
```

### Rate Limiting

The framework includes a rate limiting system that allows controlling the number of requests a client can make in a given period. This is useful for preventing abuse and denial-of-service attacks. Requires a Redis connection.

```go
{
    Name:    "LoginUser",
    Method:  rest.MethodPOST,
    Path:    "/users/login",
    Handler: LoginUser,
    Public:  true,
    RateLimiter: func(ctx *rest.EndpointContext) rest.RateLimit {
        return rest.RateLimit{
            Max:    5,                    // 5 attempts
            Window: 15 * time.Minute,     // every 15 minutes
            Key:    ctx.IpAddress,        // by IP
        }
    },
}
```

### Timeouts

Endpoints can have a configured timeout to prevent long operations from blocking the server. This is especially useful for operations that can take a long time, such as file processing or complex queries.

```go
{
    Name:    "ProcessLongOperation",
    Method:  rest.MethodPOST,
    Path:    "/operations/long",
    Handler: ProcessLongOperation,
    Timeout: 30, // 30 seconds
}
```

## Static Files and SPA Support

The framework provides built-in support for serving static files with flexible header configuration and Single Page Application (SPA) mode. This is ideal for serving frontend applications built with React, Vue, Angular, or any other framework.

### Basic Usage

```go
// Serve static files from ./public directory
err := app.ServeStatic(rest.StaticConfig{
    Prefix:    "/",
    Directory: "./public",
})
```

### SPA Mode

Enable SPA mode to handle client-side routing. When enabled, all routes that don't match existing files will fallback to `index.html`:

```go
err := app.ServeStatic(rest.StaticConfig{
    Prefix:    "/",
    Directory: "./dist",
    EnableSPA: true,
})
```

### Header Configuration

Apply different HTTP headers based on file type:

```go
err := app.ServeStatic(rest.StaticConfig{
    Prefix:       "/",
    Directory:    "./dist",
    EnableSPA:    true,

    // Base security headers for all files
    Headers:      rest.SecureStaticHeaders(),

    // No-cache for index.html (enables SPA routing)
    IndexHeaders: rest.SPAIndexHeaders(),

    // Long-term cache for assets (.js, .css, images, fonts)
    AssetHeaders: rest.CachedAssetHeaders(),
})
```

### Predefined Header Functions

- **`SecureStaticHeaders()`** - Base security headers:

  - `X-Frame-Options: SAMEORIGIN`
  - `X-Content-Type-Options: nosniff`
  - `Referrer-Policy: strict-origin-when-cross-origin`

- **`SPAIndexHeaders()`** - No-cache headers for SPA:

  - `Cache-Control: no-cache, no-store, must-revalidate`
  - `Pragma: no-cache`
  - Plus all secure headers

- **`CachedAssetHeaders()`** - Long-term caching:
  - `Cache-Control: public, max-age=31536000, immutable`
  - Plus all secure headers

### Custom Header Matcher

For advanced scenarios, use a custom function to determine headers:

```go
err := app.ServeStatic(rest.StaticConfig{
    Prefix:    "/",
    Directory: "./dist",
    EnableSPA: true,
    Headers:   rest.SecureStaticHeaders(),
    HeaderMatcher: func(requestPath, filePath string) map[string]string {
        // Special handling for WebAssembly files
        if strings.HasSuffix(requestPath, ".wasm") {
            return map[string]string{
                "Content-Type":  "application/wasm",
                "Cache-Control": "public, max-age=31536000",
            }
        }

        // Return nil to use default behavior (IndexHeaders/AssetHeaders)
        return nil
    },
})
```

### Complete Example

```go
package main

import (
    rest "github.com/xompass/vsaas-rest"
)

func main() {
    app := rest.NewRestApp(rest.RestAppOptions{
        Name: "My SPA App",
        Port: 8080,
    })

    // Register API routes
    apiGroup := app.Group("/api")
    // ... register your API endpoints here

    // Serve frontend application
    err := app.ServeStatic(rest.StaticConfig{
        Prefix:       "/",
        Directory:    "./frontend/dist",
        EnableSPA:    true,
        Headers:      rest.SecureStaticHeaders(),
        IndexHeaders: rest.SPAIndexHeaders(),
        AssetHeaders: rest.CachedAssetHeaders(),
    })
    if err != nil {
        panic(err)
    }

    app.Start()
}
```

For a complete working example, see [examples/static-files](examples/static-files).

For full documentation on static file serving, see [STATIC_FILES_README.md](STATIC_FILES_README.md).

## Logging Configuration

The framework uses Go's structured logging system (`slog`) with different levels:

```go
app := rest.NewRestApp(rest.RestAppOptions{
    Name:     "My API",
    Port:     3000,
    LogLevel: rest.LogLevelDebug, // LogLevelDebug, LogLevelInfo, LogLevelWarn, LogLevelError
})
```

Available log levels are:

- `LogLevelDebug`: Detailed information for debugging
- `LogLevelInfo`: General operation information
- `LogLevelWarn`: Warnings that don't prevent operation
- `LogLevelError`: Errors that require attention

## Environment Variables

The framework uses the following environment variables:

### MongoDB

- `MONGO_URI`: MongoDB connection URI (default: `mongodb://localhost:27017`)
- `MONGO_DATABASE`: MongoDB database name. If not set, it will use the default database from the URI or "test".

### Redis (for Rate Limiting)

- `REDIS_HOST`: Redis server host (default: `localhost`)
- `REDIS_PORT`: Redis server port (default: `6379`)
- `REDIS_PASSWORD`: Redis password (optional)

### Application

- `APP_ENV`: Application environment (default: `development`)

```bash
# Example .env file
MONGO_URI=mongodb://localhost:27017
MONGO_DATABASE=my_application
REDIS_HOST=localhost
REDIS_PORT=6379
APP_ENV=production
```

## License

See the LICENSE file for more details.
