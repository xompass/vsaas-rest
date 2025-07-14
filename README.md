# VSAAS REST Framework

VSAAS REST Framework is a web framework built on Echo v4 for building REST APIs in Go. It includes features for authentication, authorization, auditing, request validation, structured logging, advanced filtering, file uploads, per-endpoint timeouts, and rate limiting.

## Main Features

- High-performance HTTP server (Echo v4)
- Extensible database connectors (MongoDB supported)
- Role-based authentication and authorization
- Automatic operation auditing
- Redis-based rate limiting
- Secure file uploads with validation
- Request validation with go-playground/validator
- LoopBack 3-compatible query filters
- Per-endpoint timeouts
- Structured logging (slog)

## Installation

```bash
go get github.com/xompass/vsaas-rest
```

## Quick Example

```go
package main

import (
    rest "github.com/xompass/vsaas-rest"
    "github.com/xompass/vsaas-rest/database"
)

func main() {
    // Create application
    app := rest.NewRestApp(rest.RestAppOptions{
        Name:              "My API",
        Port:              3000,
        LogLevel:          rest.LogLevelDebug
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
    // MONGO_DATABASE: Database name. Required
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

### Filters and Queries

The framework provides an advanced filter system based on **LoopBack 3** syntax (Node.js framework) that allows creating complex queries both programmatically and through query parameters. This syntax is familiar to developers who have worked with LoopBack and provides a consistent and powerful interface for filtering data.

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

#### Request Body Validation

The framework includes a request body validation system using the `Validable` interface and the `go-playground/validator` validator.

##### Validable Interface

All structs used as body parameters must implement the `Validable` interface:

```go
type Validable interface {
    Validate(ctx *EndpointContext) error
}
```

#### Basic Example

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
- `MONGO_DATABASE`: MongoDB database name (required)

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
