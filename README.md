# VSAAS REST Framework

Un framework web robusto y completo construido sobre Echo v4 para crear APIs REST en Go, diseñado específicamente para aplicaciones empresariales con características avanzadas de autenticación, autorización, auditoría, limitación de velocidad y manejo de archivos.

## Características Principales

- **Basado en Echo v4**: Framework web robusto y de alto rendimiento
- **Base de Datos**: Soporte para MongoDB con sistema de conectores extensible
- **Autenticación y Autorización**: Sistema de roles y permisos con autenticador personalizable
- **Auditoría**: Sistema de logging automático de operaciones con handler personalizable
- **Rate Limiting**: Control de velocidad de requests con soporte para Redis
- **Subida de Archivos**: Manejo avanzado de archivos con validación y configuración flexible
- **Validación**: Validación automática de requests usando go-playground/validator
- **Filtros Avanzados**: Sistema de filtros basado en sintaxis LoopBack 3 para consultas complejas
- **Timeouts**: Control de tiempo límite por endpoint para evitar operaciones largas

## Instalación

```bash
go get github.com/xompass/vsaas-rest
```

## Uso Básico

```go
package main

import (
    rest "github.com/xompass/vsaas-rest"
    "github.com/xompass/vsaas-rest/database"
)

func main() {
    // Crear aplicación
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

    // Crear grupo de rutas
    api := app.Group("/api")

    // Registrar endpoint
    app.RegisterEndpoint(endpoint, api)

    // Iniciar servidor
    err := app.Start()
    if err != nil {
        panic(err)
    }
}
```

## Base de datos, Modelos y Repositorios

### Datasource y Conectores

El `Datasource` es el componente central que centraliza las conexiones a la base de datos y el registro de modelos. Un Datasource puede tener múltiples conectores para diferentes bases de datos.

El `Connector` es responsable de establecer la conexión con la base de datos específica (actualmente solo MongoDB).

> **Nota**: Actualmente el framework solo soporta MongoDB como base de datos. El soporte para otros motores de base de datos **podría** implementarse en futuras versiones de ser necesario.

#### Configurar Datasource y Conectores

```go
import (
    "github.com/xompass/vsaas-rest/database"
)

func main() {
    // Opción 1: Usar configuración por defecto
    // Esto usa las variables de entorno:
    // MONGO_URI: URI de conexión a MongoDB, default: `mongodb://localhost:27017`
    // MONGO_DATABASE: Nombre de la base de datos. Requerida
    // El nombre del conector será "mongodb"
    mongoConnector, err := database.NewDefaultMongoConnector()
    if err != nil {
        panic(err)
    }

    // Opción 2: Configuración personalizada
    // opts := &database.MongoConnectorOpts{
    //     ClientOptions: *options.Client().ApplyURI("mongodb://localhost:27017"),
    //     Name:          "mongodb",
    //     Database:      "mi_base_datos",
    // }
    // mongoConnector, err := database.NewMongoConnector(opts)
    // La función `NewMongoConnector` crea el conector y realiza la conexión a la base de datos.

    // Crear datasource y agregar conector
    datasource := database.Datasource{}
    datasource.AddConnector(mongoConnector)

    // Usar en la aplicación
    app := rest.NewRestApp(rest.RestAppOptions{
        Datasource: &datasource,
        // ... otras opciones
    })
}
```

### Modelos

#### Interfaz IModel

La interfaz `IModel` define los métodos que deben implementar todos los modelos. Esto permite al framework manejar operaciones CRUD de manera genérica y flexible.

```go
type IModel interface {
    GetTableName() string     // Nombre de la tabla/colección en la base de datos
    GetModelName() string     // Nombre único del modelo para el registro
    GetConnectorName() string // Nombre del conector de base de datos a usar
    GetId() any              // ID del documento/registro
}
```

#### Crear un Modelo

Ejemplo completo de un modelo que implementa `IModel`:

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

    // Campos automáticos (opcionales)
    Created  *time.Time `json:"created,omitempty" bson:"created,omitempty"`
    Modified *time.Time `json:"modified,omitempty" bson:"modified,omitempty"`
    Deleted  *time.Time `json:"deleted,omitempty" bson:"deleted,omitempty"`
}

// Implementar la interfaz IModel
func (p Product) GetId() any {
    return p.ID
}

func (p Product) GetTableName() string {
    return "products" // Nombre de la colección en MongoDB
}

func (p Product) GetModelName() string {
    return "Product" // Nombre único para el registro
}

func (p Product) GetConnectorName() string {
    return "mongodb" // Debe coincidir con el nombre del conector registrado
}
```

#### Hooks del Modelo (Opcional)

Puedes implementar hooks para ejecutar lógica antes de operaciones:

```go
// Hook antes de crear
type BeforeCreateHook interface {
    BeforeCreate() error
}

// Hook antes de actualizar
type BeforeUpdateHook interface {
    BeforeUpdate() error
}

// Hook antes de eliminar
type BeforeDeleteHook interface {
    BeforeDelete() error
}

// Ejemplo de implementación
func (p *Product) BeforeCreate() error {
    // Lógica antes de crear, como establecer valores predeterminados
    return nil
}
```

### Repositorios

#### Crear un Repositorio

```go
package main

import (
    "github.com/xompass/vsaas-rest/database"
)

type ProductRepository database.Repository[Product]

func NewProductRepository(ds *database.Datasource) (ProductRepository, error) {
    // Crear repositorio con opciones
    repository, err := database.NewMongoRepository[Product](ds, database.RepositoryOptions{
        Created:  true,  // Maneja automáticamente el campo "created"
        Modified: true,  // Maneja automáticamente el campo "modified"
        Deleted:  true,  // Habilita soft delete con campo "deleted"
    })

    if err != nil {
        return nil, err
    }

    return repository, nil
}
```

#### Registrar Repositorios

```go
package repositories

import (
    rest "github.com/xompass/vsaas-rest"
    "github.com/xompass/vsaas-rest/database"
)

func main(){
    // Crear datasource y conectores
    datasource := database.Datasource{}
    mongoConnector, err := database.NewDefaultMongoConnector()
    if err != nil {
        panic(err)
    }
    datasource.AddConnector(mongoConnector)

    app := rest.NewRestApp(rest.RestAppOptions{
        Datasource: &datasource,
        // ... otras opciones
    })

    _, err= NewProductRepository(&datasource)

    if err != nil {
        panic(err)
    }

    // Registrar endpoints e iniciar servidor
}
```

#### Operaciones de Repositorio

Los repositorios proporcionan una interfaz para realizar operaciones CRUD y consultas sobre los modelos. Aquí hay algunos ejemplos de cómo usar un repositorio:

```go
// Buscar todos los registros
products, err := repo.Find(ctx, nil)

// Buscar con filtro
filter := database.NewFilter().WithWhere(database.NewWhere().Gte("price", 100))
products, err := repo.Find(ctx, filter)

// Buscar uno
filter = database.NewFilter().WithWhere(database.NewWhere().Eq("name", "Product A"))
product, err := repo.FindOne(ctx, filter)

// Buscar por ID
product, err := repo.FindById(ctx, productID, nil)

// Crear
newProduct := Product{Name: "Product B", Price: 99.99}
insertID, err := repo.Insert(ctx, newProduct)

// Crear y retornar el documento creado
createdItem, err := repo.Create(ctx, newProduct)

type UpdateItem struct {
    Name  string   `bson:"name"`
    Price *float64 `bson:"price,omitempty"` // omitempty hace que este campo no se actualice si está vacío
}

// Actualizar por ID
err = repo.UpdateById(ctx, itemID, UpdateItem{
    Name:  "Updated Name",
})

// Actualizar uno basico
// UpdateOne actualiza el primer documento que coincida con el filtro y actualiza solo los campos especificados
err = repo.UpdateOne(ctx, filter, UpdateItem{
    Name:  "Updated Product",
})

// Buscar y actualizar
// FindOneAndUpdate busca un documento y lo actualiza, retornando el documento actualizado
updatedItem, err := repo.FindOneAndUpdate(ctx, filter, UpdateItem{
    Name:  "Updated Product",
    Price: 19.99,
})

// Actualizar avanzado
// UpdateOne, UpdateById y FindOneAndUpdate permiten realizar actualizaciones más complejas con operadores de MongoDB
err = repo.UpdateOne(ctx, filter, bson.M{"$set": bson.M{"name": "Updated Product"}, "$inc": bson.M{"price": 10}})

// Contar documentos
count, err := repo.Count(ctx, filter)

// Verificar existencia
exists, err := repo.Exists(ctx, itemID)

// Eliminar (soft delete si está habilitado)
err = repo.DeleteById(ctx, itemID)

// Eliminar múltiples
deletedCount, err := repo.DeleteMany(ctx, filter)
```

### Filtros y Consultas

El framework proporciona un sistema avanzado de filtros basado en la sintaxis de **LoopBack 3** (Node.js framework) que permite crear consultas complejas tanto programáticamente como a través de query parameters. Esta sintaxis es familiar para desarrolladores que han trabajado con LoopBack y proporciona una interfaz consistente y poderosa para filtrar datos.

#### Compatibilidad con LoopBack 3

El sistema de filtros de `vsaas-rest` está basado en LoopBack 3, proporcionando una sintaxis familiar y potente:

**Características compatibles:**

- **where**: Filtros de condición con operadores como `gt`, `lt`, `gte`, `lte`, `eq`, `neq`, `in`, `nin`, `like`, `nlike`
- **order**: Ordenamiento ascendente/descendente por múltiples campos
- **limit/skip**: Paginación estándar
- **fields**: Proyección de campos (incluir/excluir)

> Nota: La opción `include` aún no ha sido implementada

#### Usar FilterBuilder Programáticamente

```go
import "github.com/xompass/vsaas-rest/database"

func GetUsers(ctx *rest.EndpointContext) error {
    // Crear filtro programáticamente
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

#### Usar Filtros desde Query Parameters

Cuando defines un parámetro de tipo `QueryParamTypeFilter`, el framework automáticamente parsea el JSON y te da un `FilterBuilder`:

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
    // El framework ya parseó el filtro del query parameter
    filter, err := ctx.GetFilterParam()
    if err != nil {
        return err
    }

    // Si no hay filtro, crear uno nuevo
    if filter == nil {
        filter = database.NewFilter()
    }

    // Obtener otros parámetros parseados
    if search, ok := ctx.ParsedQuery["search"].(string); ok && search != "" {
        // Agregar búsqueda por texto al filtro existente
        filter = filter.WithWhere(database.NewWhere().
            Like("name", search, "i"), // "i" para case-insensitive
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

#### Ejemplos de Requests con Filtros

```bash
# Filtro básico
GET /api/items?filter={"where":{"isActive":true,"priority":{"gt":1}},"limit":10,"order":"name ASC"}

# Filtro con búsqueda adicional
GET /api/items?filter={"where":{"isActive":true}}&search=test&limit=5

# Filtro complejo con múltiples condiciones
GET /api/items?filter={"where":{"and":[{"priority":{"gte":1}},{"status":"active"}]},"order":"name ASC","limit":20,"skip":40}

# Solo proyección de campos específicos
GET /api/items?filter={"fields":{"name":true,"description":true},"limit":10}
```

#### API de FilterBuilder

**Métodos de construcción:**

```go
filter := database.NewFilter()

// Condiciones WHERE
filter.WithWhere(database.NewWhere().Eq("status", "active"))
filter.WithWhere(database.NewWhere().Gt("age", 18))

// Ordenamiento
filter.OrderByAsc("name")
filter.OrderByDesc("createdAt")

// Paginación
filter.Limit(10)
filter.Skip(20)
filter.Page(2, 10) // página 2, 10 elementos por página

// Proyección de campos
filter.Fields(map[string]bool{
    "name":  true,
    "email": true,
    "_id":   false, // excluir _id
})
```

#### API de WhereBuilder

**Operadores de comparación:**

```go
where := database.NewWhere()

// Igualdad
where.Eq("status", "active")
where.Neq("status", "inactive")

// Comparación numérica
where.Gt("age", 18)
where.Gte("age", 18)
where.Lt("age", 65)
where.Lte("age", 65)

// Arrays
where.In("category", []string{"tech", "science"})
where.Nin("status", []string{"deleted", "banned"})

// Rangos
where.Between("age", 18, 65, false) // inclusive
where.Between("score", 80, 100, true) // exclusive

// Búsqueda de texto (regex)
where.Like("name", "john", "i") // case-insensitive

// Valores nulos
where.IsNull("deletedAt")
where.IsNotNull("email")
```

**Operadores lógicos:**

```go
// AND (se combinan automáticamente)
where := database.NewWhere().
    Eq("isActive", true).
    Gt("age", 18)

// OR explícito
where := database.NewWhere().Or(
    database.NewWhere().Eq("role", "admin"),
    database.NewWhere().Eq("role", "moderator"),
)

// Combinaciones complejas
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

Para definir endpoints en tu API, puedes usar la estructura `Endpoint` del framework. Cada endpoint define un método HTTP, una ruta, un handler y otros parámetros como roles, tipo de acción y validación.

```go
package main

import (
    rest "github.com/xompass/vsaas-rest"
)

// Definir estructura de request
type CreateProductRequest struct {
    Name        string  `json:"name" validate:"required,min=2,max=100"`
    Price       float64 `json:"price" validate:"required,gte=0"`
}

func (r *CreateProductRequest) Validate(ctx *rest.EndpointContext) error {
    return ctx.ValidateStruct(r)
}

// Definir endpoints
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
    filter, err := ctx.GetFilterParam() // Obtener filtro de query params
    if err != nil {
        return err
    }

    // Obtener repositorio de productos
    repo, err := database.GetDatasourceModelRepository(ctx.App.Datasource, Product{})
    if err != nil {
        return rest.NewErrorResponse(500, "Database error")
    }

    // Consultar datos
    products, err := repo.Find(ctx.Context(), filter)
    if err != nil {
        return rest.NewErrorResponse(500, "Failed to fetch products")
    }

    // Operación de lectura con auditoría para trazabilidad
    return ctx.JSON(products)
}

func CreateProduct(ctx *rest.EndpointContext) error {
    req := ctx.ParsedBody.(*CreateProductRequest)

    item := Product{
        Name:  req.Name,
        Price: req.Price,
    }
    }

    repo, err := database.GetDatasourceModelRepository(ctx.App.Datasource, Product{})
    if err != nil {
        return rest.NewErrorResponse(500, "Database error")
    }

    result, err := repo.Insert(ctx.Context(), item)
    if err != nil {
        return rest.NewErrorResponse(500, "Failed to create item")
    }

    // Antes de responder, puedes agregar lógica de auditoría u otra lógica adicional.
    // Ver Configurar Auditoría
    return ctx.RespondAndLog(result, result.InsertedID, rest.ResponseTypeJSON, 201)
}
```

### Configurar Auditoría

El framework incluye un sistema de auditoría configurable que te permite registrar automáticamente las operaciones realizadas en tu API.

#### AuditLogConfig

La auditoría se configura mediante `AuditLogConfig`:

```go
app := rest.NewRestApp(rest.RestAppOptions{
    // ... otras opciones
    AuditLogConfig: &rest.AuditLogConfig{
        Enabled: true,          // Habilitar auditoría
        Handler: MyAuditHandler, // Función personalizada para manejar auditoría
    },
})
```

El `Handler` es una función que tú defines para procesar la información de auditoría como consideres conveniente:

```go
// Función de auditoría que decide qué hacer con los logs
func MyAuditHandler(ctx *rest.EndpointContext, response any, affectedModelId any) error {
    principal := ctx.Principal
    // Puedes implementar cualquier lógica que necesites que se ejecute antes de responder

    // Ejemplo de lógica de auditoría:
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

    // Guardar en base datos
    // Enviar a un sistema externo
    // Escribir a un archivo de log
    // Enviar a métricas/monitoring
    return nil
}
```

#### Parámetros de Endpoints

En el endpoint se pueden definir el campo `Accepts` para especificar los parámetros que acepta, incluyendo parámetros de ruta, query y headers. Los parámetros definidos en `Accepts` se parsean automáticamente y están disponibles en el contexto:

##### Path Parameters

```go
rest.NewPathParam("id", rest.PathParamTypeObjectID)
rest.NewPathParam("count", rest.PathParamTypeInt)
```

#### Query Parameters

```go
rest.NewQueryParam("filter", rest.QueryParamTypeFilter)          // Filtro LoopBack-style
rest.NewQueryParam("limit", rest.QueryParamTypeInt)              // Entero
rest.NewQueryParam("search", rest.QueryParamTypeString)          // String
rest.NewQueryParam("active", rest.QueryParamTypeBool)            // Boolean
rest.NewQueryParam("date", rest.QueryParamTypeDate)              // Fecha
```

#### Header Parameters

```go
rest.NewHeaderParam("X-Custom-Header", rest.HeaderParamTypeString, true) // Requerido
rest.NewHeaderParam("X-Request-ID", rest.HeaderParamTypeString, false)
```

#### Acceso a Parámetros Parseados

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
    // id es un parámetro de ruta, ya parseado automáticamente
    id := ctx.ParsedPath["id"].(bson.ObjectID)

    // Construir filtro combinando query param filter con otros parámetros
    filter, err := ctx.GetFilterParam() // o ctx.ParsedQuery["filter"]
    if err != nil {
        return err
    }

    if filter == nil {
        filter = database.NewFilter()
    }

    // Hacer algo con id y filter
    return ctx.JSON(map[string]any{
        "id": id.Hex(),
        "filter": filter,
    })
}
```

#### Validación del Body de Request

El framework incluye un sistema de validación del body de las peticiones HTTP utilizando la interfaz `Validable` y el validador `go-playground/validator`.

##### Interfaz Validable

Todos los structs que se usan como body parameters deben implementar la interfaz `Validable`:

```go
type Validable interface {
    Validate(ctx *EndpointContext) error
}
```

#### Ejemplo Básico

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
    // Aquí puedes agregar lógica adicional de validación si es necesario
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

Como utilizar `go-playground/validator` puede ser encontrado en la [documentación oficial](https://pkg.go.dev/github.com/go-playground/validator/v10).

### Autenticación

El framework permite definir una función autenticadora personalizada que se ejecuta antes de cada endpoint que no sea público. Esta función debe implementar la lógica de autenticación y autorización, devolviendo un `Principal` y un `AuthToken`.

```go
// Ejemplo de función de autorización personalizada
func MyAuthorizer(ctx *rest.EndpointContext) (rest.Principal, rest.AuthToken, error) {
    // Obtener token del header Authorization
    token := ctx.EchoCtx.Request().Header.Get("Authorization")
    if token == "" {
        return nil, nil, rest.NewErrorResponse(401, "Authorization header required")
    }

    // Validar token (implementar lógica específica)
    principal, authToken, err := ValidateToken(token)
    if err != nil {
        return nil, nil, rest.NewErrorResponse(401, "Invalid token")
    }

    return principal, authToken, nil
}

// Implementar interfaces
type MyPrincipal struct {
    ID   string
    Role string
}

// Implementar la interfaz Principal
func (p *MyPrincipal) GetPrincipalID() string   { return p.ID }
func (p *MyPrincipal) GetPrincipalRole() string { return p.Role }

type MyAuthToken struct {
    Token     string
    UserId    string
    UserType  string
    ExpiresAt int64
}

// Implementar la interfaz AuthToken
func (t *MyAuthToken) IsValid() bool       { return time.Now().Unix() < t.ExpiresAt }
func (t *MyAuthToken) GetUserId() string   { return t.UserId }
func (t *MyAuthToken) GetUserType() string { return t.UserType }
func (t *MyAuthToken) GetToken() string    { return t.Token }
func (t *MyAuthToken) GetExpiresAt() int64 { return t.ExpiresAt }
```

### Subida de Archivos

```go
{
    Name:    "UploadUserAvatar",
    Method:  rest.MethodPOST,
    Path:    "/users/:id/avatar",
    Handler: UploadAvatar,
    FileUploadConfig: &rest.FileUploadConfig{
        MaxFileSize:        10 * 1024 * 1024, // 10MB
        TypeSizeLimits:     map[rest.FileExtension]int64{
            ".jpg": 5 * 1024 * 1024,  // 5MB para JPG
            ".png": 10 * 1024 * 1024, // 10MB para PNG
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

    // Procesar archivo...

    return ctx.RespondAndLog(map[string]string{
        "message": "Avatar uploaded successfully",
        "path":    file.SavedPath,
    }, ctx.ParsedPath["id"], rest.ResponseTypeJSON)
}
```

### Rate Limiting

El framework incluye un sistema de limitación de velocidad (rate limiting) que permite controlar la cantidad de solicitudes que un cliente puede hacer en un período determinado. Esto es útil para prevenir abusos y ataques de denegación de servicio. Requiere una conexión a Redis.

```go
{
    Name:    "LoginUser",
    Method:  rest.MethodPOST,
    Path:    "/users/login",
    Handler: LoginUser,
    Public:  true,
    RateLimiter: func(ctx *rest.EndpointContext) rest.RateLimit {
        return rest.RateLimit{
            Max:    5,                    // 5 intentos
            Window: 15 * time.Minute,     // cada 15 minutos
            Key:    ctx.IpAddress,        // por IP
        }
    },
}
```

### Timeouts

Los endpoints pueden tener un timeout configurado para evitar que operaciones largas bloqueen el servidor. Esto es especialmente útil para operaciones que pueden tardar mucho tiempo, como procesamiento de archivos o consultas complejas.

```go
{
    Name:    "ProcessLongOperation",
    Method:  rest.MethodPOST,
    Path:    "/operations/long",
    Handler: ProcessLongOperation,
    Timeout: 30, // 30 segundos
}
```

## Licencia

Consulta el archivo LICENSE para más detalles.

## Configuración de Logging

El framework utiliza el sistema de logging estructurado de Go (`slog`) con diferentes niveles:

```go
app := rest.NewRestApp(rest.RestAppOptions{
    Name:     "Mi API",
    Port:     3000,
    LogLevel: rest.LogLevelDebug, // LogLevelDebug, LogLevelInfo, LogLevelWarn, LogLevelError
})
```

Los niveles de log disponibles son:

- `LogLevelDebug`: Información detallada para debugging
- `LogLevelInfo`: Información general del funcionamiento
- `LogLevelWarn`: Advertencias que no impiden el funcionamiento
- `LogLevelError`: Errores que requieren atención

## Variables de Entorno

El framework utiliza las siguientes variables de entorno:

### MongoDB

- `MONGO_URI`: URI de conexión a MongoDB (default: `mongodb://localhost:27017`)
- `MONGO_DATABASE`: Nombre de la base de datos MongoDB (requerida)

### Redis (para Rate Limiting)

- `REDIS_HOST`: Host del servidor Redis (default: `localhost`)
- `REDIS_PORT`: Puerto del servidor Redis (default: `6379`)
- `REDIS_PASSWORD`: Contraseña de Redis (opcional)

### Aplicación

- `APP_ENV`: Entorno de la aplicación (default: `development`)

```bash
# Ejemplo de archivo .env
MONGO_URI=mongodb://localhost:27017
MONGO_DATABASE=mi_aplicacion
REDIS_HOST=localhost
REDIS_PORT=6379
APP_ENV=production
```
