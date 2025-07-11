# VSAAS REST Framework

Un framework web robusto y completo construido sobre Echo v4 para crear APIs REST en Go, diseñado específicamente para aplicaciones empresariales con características avanzadas de autenticación, autorización, auditoría, limitación de velocidad y manejo de archivos.

## Características Principales

- 🔧 **Basado en Echo v4**: Aprovecha la velocidad y flexibilidad de Echo
- 🔐 **Autenticación y Autorización**: Sistema completo de roles y permisos
- 📝 **Auditoría Automática**: Logging detallado de todas las operaciones
- 🚦 **Rate Limiting**: Control de velocidad con Redis
- 📁 **Subida de Archivos**: Manejo streaming de archivos con configuración flexible
- 🎯 **Validación**: Validación automática de parámetros y cuerpo de requests
- 🔍 **Filtros Avanzados**: Sistema de consultas MongoDB-style
- 📊 **Logging Estructurado**: Logging con niveles configurables
- ⏱️ **Timeouts**: Control de timeouts por endpoint
- 🗃️ **Base de Datos**: Integración nativa con MongoDB

## Instalación

```bash
go get github.com/xompass/vsaas-rest
```

## Dependencias Críticas

```go
require (
    github.com/labstack/echo/v4 v4.13.4
    github.com/go-playground/validator/v10 v10.26.0
    github.com/redis/go-redis/v9 v9.11.0
    go.mongodb.org/mongo-driver/v2 v2.2.2
)
```

## Uso Básico

### 1. Crear una Aplicación

```go
package main

import (
    rest "github.com/xompass/vsaas-rest"
    "github.com/xompass/vsaas-rest/database"
)

func main() {
    // Configurar MongoDB
    mongoConnector, err := database.NewDefaultMongoConnector()
    if err != nil {
        panic(err)
    }

    datasource := database.Datasource{}
    datasource.AddConnector(mongoConnector)

    // Crear aplicación
    app := rest.NewRestApp(rest.RestAppOptions{
        Name:              "Mi API",
        Port:              3000,
        Datasource:        &datasource,
        Authorizer:        MyAuthorizer,
        LogLevel:          rest.LogLevelDebug,
        EnableRateLimiter: true,
        AuditLogConfig: &rest.AuditLogConfig{
            Enabled: true,
            Handler: MyAuditHandler,
        },
    })

    // Crear grupo de rutas
    api := app.Group("/api")

    // Registrar endpoints (definidos más abajo en el ejemplo)
    app.RegisterEndpoints(myEndpoints, api)

    // También puedes llamar a funciones de registro organizadas por módulo:
    // RegisterEndpoints(app)

    // Iniciar servidor
    err = app.Start()
    if err != nil {
        panic(err)
    }
}
```

### 2. Definir Endpoints

```go
package main

import (
    rest "github.com/xompass/vsaas-rest"
)

// Definir estructura de request
type CreateItemRequest struct {
    Name        string `json:"name" validate:"required,min=2,max=100"`
    Description string `json:"description" validate:"max=500"`
}

func (r *CreateItemRequest) Validate(ctx *rest.EndpointContext) error {
    return ctx.ValidateStruct(r)
}

// Definir endpoints
var myEndpoints = []*rest.Endpoint{
    {
        Name:       "GetAllItems",
        Method:     rest.MethodGET,
        Path:       "/items",
        Handler:    GetItems,
        Roles:      []rest.EndpointRole{PermListItems},
        ActionType: string(rest.ActionTypeRead),
        Model:      "MyModel",
        Accepts: []rest.Param{
            rest.NewQueryParam("filter", rest.QueryParamTypeFilter),
            rest.NewQueryParam("limit", rest.QueryParamTypeInt),
            rest.NewQueryParam("offset", rest.QueryParamTypeInt),
        },
    },
    {
        Name:       "GetItemByID",
        Method:     rest.MethodGET,
        Path:       "/items/:id",
        Handler:    GetItemByID,
        Roles:      []rest.EndpointRole{PermReadItem},
        ActionType: string(rest.ActionTypeRead),
        Model:      "MyModel",
        Accepts: []rest.Param{
            rest.NewPathParam("id", rest.PathParamTypeObjectID),
            rest.NewQueryParam("filter", rest.QueryParamTypeFilter),
        },
    },
    {
        Name:       "CreateItem",
        Method:     rest.MethodPOST,
        Path:       "/items",
        Handler:    CreateItem,
        BodyParams: func() rest.Validable { return &CreateItemRequest{} },
        Roles:      []rest.EndpointRole{PermCreateItem},
        ActionType: string(rest.ActionTypeCreate),
        Model:      "MyModel",
    },
    {
        Name:       "UpdateItem",
        Method:     rest.MethodPATCH,
        Path:       "/items/:id",
        Handler:    UpdateItem,
        BodyParams: func() rest.Validable { return &UpdateItemRequest{} },
        Roles:      []rest.EndpointRole{PermUpdateItem},
        ActionType: string(rest.ActionTypeUpdate),
        Model:      "MyModel",
        Accepts: []rest.Param{
            rest.NewPathParam("id", rest.PathParamTypeObjectID),
        },
    },
    {
        Name:       "DeleteItem",
        Method:     rest.MethodDELETE,
        Path:       "/items/:id",
        Handler:    DeleteItem,
        Roles:      []rest.EndpointRole{PermDeleteItem},
        ActionType: string(rest.ActionTypeDelete),
        Model:      "MyModel",
        Accepts: []rest.Param{
            rest.NewPathParam("id", rest.PathParamTypeObjectID),
        },
    },
    {
        Name:       "HealthCheck",
        Method:     rest.MethodGET,
        Path:       "/health",
        Handler:    HealthCheck,
        Public:     true, // Endpoint público
        ActionType: string(rest.ActionTypeRead),
    },
}

// Registrar endpoints en la aplicación
func RegisterEndpoints(app *rest.RestApp) {
    // Crear grupo de rutas
    api := app.Group("/api")

    // Registrar los endpoints
    app.RegisterEndpoints(myEndpoints, api)
}
```

### 3. Implementar Handlers

```go
// Definir un modelo genérico para el ejemplo
type MyModel struct {
    ID   string `json:"id" bson:"_id,omitempty"`
    Name string `json:"name" bson:"name"`
    // ... otros campos
}

// Implementar la interfaz IModel
func (m MyModel) GetTableName() string {
    return "my_collection"
}

func (m MyModel) GetModelName() string {
    return "MyModel"
}

func (m MyModel) GetConnectorName() string {
    return "mongodb"
}

func (m MyModel) GetId() any {
    return m.ID
}

func GetItems(ctx *rest.EndpointContext) error {
    filter, err := ctx.GetFilterParam()
    if err != nil {
        return err
    }

    // Obtener repositorio
    repo, err := database.GetDatasourceModelRepository(ctx.App.Datasource, MyModel{})
    if err != nil {
        return rest.NewErrorResponse(500, "Database error")
    }

    // Consultar datos
    items, err := repo.Find(ctx.Context(), filter)
    if err != nil {
        return rest.NewErrorResponse(500, "Failed to fetch items")
    }

    // Operación de lectura con auditoría para trazabilidad
    return ctx.RespondAndLog(items, nil, rest.ResponseTypeJSON)
}

func GetItemByID(ctx *rest.EndpointContext) error {
    id := ctx.ParsedPath["id"]

    repo, err := database.GetDatasourceModelRepository(ctx.App.Datasource, MyModel{})
    if err != nil {
        return rest.NewErrorResponse(500, "Database error")
    }

    item, err := repo.FindById(ctx.Context(), id)
    if err != nil {
        return rest.NewErrorResponse(404, "Item not found")
    }

    // Operación sensible con auditoría
    return ctx.RespondAndLog(item, id, rest.ResponseTypeJSON)
}

func CreateItem(ctx *rest.EndpointContext) error {
    req := ctx.ParsedBody.(*CreateItemRequest)

    item := MyModel{
        Name: req.Name,
        // ... otros campos
    }

    repo, err := database.GetDatasourceModelRepository(ctx.App.Datasource, MyModel{})
    if err != nil {
        return rest.NewErrorResponse(500, "Database error")
    }

    result, err := repo.Insert(ctx.Context(), item)
    if err != nil {
        return rest.NewErrorResponse(500, "Failed to create item")
    }

    // Operación de escritura DEBE usar auditoría
    return ctx.RespondAndLog(result, result.InsertedID, rest.ResponseTypeJSON, 201)
}

// Ejemplo de endpoint que NO requiere auditoría
func HealthCheck(ctx *rest.EndpointContext) error {
    // Operación simple sin auditoría
    return ctx.JSON(map[string]string{
        "status":    "ok",
        "timestamp": time.Now().Format(time.RFC3339),
    })
}
```

### 4. Configurar Autenticación

```go
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

func (p *MyPrincipal) GetPrincipalID() string   { return p.ID }
func (p *MyPrincipal) GetPrincipalRole() string { return p.Role }

type MyAuthToken struct {
    Token     string
    UserId    string
    UserType  string
    ExpiresAt int64
}

func (t *MyAuthToken) IsValid() bool       { return time.Now().Unix() < t.ExpiresAt }
func (t *MyAuthToken) GetUserId() string   { return t.UserId }
func (t *MyAuthToken) GetUserType() string { return t.UserType }
func (t *MyAuthToken) GetToken() string    { return t.Token }
func (t *MyAuthToken) GetExpiresAt() int64 { return t.ExpiresAt }
```

### 5. Configurar Auditoría (Opcional)

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

#### Handler de Auditoría Personalizado

El `Handler` es una función que tú defines para procesar la información de auditoría como consideres conveniente:

```go
// Función de auditoría que decide qué hacer con los logs
func MyAuditHandler(ctx *rest.EndpointContext, response any, affectedModelId any) error {
    principal := ctx.Principal
    if principal == nil {
        // No hay usuario autenticado, omitir auditoría
        return nil
    }

    // Puedes implementar cualquier lógica aquí:

    // Opción 1: Guardar en base de datos
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

    // Guardar en MongoDB, PostgreSQL, archivo, etc.
    return saveAuditLog(auditLog)

    // Opción 2: Enviar a un sistema externo
    // return sendToAuditService(auditLog)

    // Opción 3: Escribir a un archivo de log
    // return writeToLogFile(auditLog)

    // Opción 4: Enviar a métricas/monitoring
    // return sendToMetrics(auditLog)
}
```

#### RespondAndLog vs Respuestas Directas

El framework proporciona dos formas de responder:

##### 1. `RespondAndLog` - Con Auditoría Automática

```go
func CreateUser(ctx *rest.EndpointContext) error {
    // ... lógica de creación

    // Respuesta CON auditoría automática
    return ctx.RespondAndLog(user, user.ID, rest.ResponseTypeJSON, 201)
}
```

##### 2. Respuestas Directas - Sin Auditoría

```go
func GetHealthCheck(ctx *rest.EndpointContext) error {
    // Operación simple que no requiere auditoría
    return ctx.JSON(map[string]string{"status": "ok"})
}

func InternalOperation(ctx *rest.EndpointContext) error {
    // ... alguna operación interna

    // Respuesta directa sin auditoría
    return ctx.JSON("Operation completed", 200)
}
```

#### Cuándo Usar Cada Método

**Usar `RespondAndLog`** para:

- Operaciones de escritura (Create, Update, Delete)
- Acceso a datos sensibles
- Operaciones que requieren trazabilidad
- APIs que necesitan cumplir con regulaciones

**Usar respuestas directas** para:

- Health checks
- Operaciones de solo lectura simples
- Endpoints internos
- Respuestas que no requieren auditoría

#### Deshabilitar Auditoría por Endpoint

También puedes deshabilitar la auditoría para endpoints específicos:

```go
{
    Name:          "HealthCheck",
    Method:        rest.MethodGET,
    Path:          "/health",
    Handler:       HealthCheck,
    Public:        true,
    AuditDisabled: true, // No auditar este endpoint
}
```

## Características Avanzadas

### Subida de Archivos

```go
{
    Name:    "UploadUserAvatar",
    Method:  rest.MethodPOST,
    Path:    "/users/:id/avatar",
    Handler: UploadAvatar,
    FileUploadConfig: &rest.FileUploadConfig{
        MaxFileSize:         10 * 1024 * 1024, // 10MB
        AllowedExtensions:   []string{".jpg", ".png", ".gif"},
        AllowedMimeTypes:    []string{"image/jpeg", "image/png", "image/gif"},
        UploadPath:          "./uploads/avatars",
        KeepFilesAfterSend:  true,
        GenerateUniqueNames: true,
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

### Filtros y Consultas

El framework proporciona un sistema avanzado de filtros basado en MongoDB que permite crear consultas complejas tanto programáticamente como a través de query parameters.

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

#### Acceso a Parámetros Parseados

Los parámetros definidos en `Accepts` se parsean automáticamente y están disponibles en el contexto:

```go
{
    Name: "GetUserPosts",
    Path: "/users/:userId/posts",
    Accepts: []rest.Param{
        rest.NewPathParam("userId", rest.PathParamTypeObjectID),
        rest.NewQueryParam("status", rest.QueryParamTypeString),
        rest.NewQueryParam("limit", rest.QueryParamTypeInt),
        rest.NewQueryParam("published", rest.QueryParamTypeBool),
        rest.NewQueryParam("filter", rest.QueryParamTypeFilter),
    },
}

func GetUserPosts(ctx *rest.EndpointContext) error {
    // Parámetros de path (siempre disponibles si son requeridos)
    userId := ctx.ParsedPath["userId"].(bson.ObjectID)

    // Parámetros de query (verificar existencia)
    var status string
    if s, ok := ctx.ParsedQuery["status"].(string); ok {
        status = s
    }

    var limit int = 10 // valor por defecto
    if l, ok := ctx.ParsedQuery["limit"].(int); ok && l > 0 {
        limit = l
    }

    var published *bool
    if p, ok := ctx.ParsedQuery["published"].(bool); ok {
        published = &p
    }

    // Construir filtro combinando query param filter con otros parámetros
    filter, err := ctx.GetFilterParam()
    if err != nil {
        return err
    }

    if filter == nil {
        filter = database.NewFilter()
    }

    // Agregar condiciones basadas en otros parámetros
    whereBuilder := database.NewWhere().Eq("authorId", userId)

    if status != "" {
        whereBuilder = whereBuilder.Eq("status", status)
    }

    if published != nil {
        whereBuilder = whereBuilder.Eq("published", *published)
    }

    filter = filter.WithWhere(whereBuilder).Limit(uint(limit))

    // Usar el filtro final
    repo, err := database.GetDatasourceModelRepository(ctx.App.Datasource, Post{})
    if err != nil {
        return rest.NewErrorResponse(500, "Database error")
    }

    posts, err := repo.Find(ctx.Context(), filter)
    if err != nil {
        return rest.NewErrorResponse(500, "Failed to fetch posts")
    }

    return ctx.RespondAndLog(posts, nil, rest.ResponseTypeJSON)
}
```

#### Tipos de Query Parameters Disponibles

```go
// Filtros y consultas avanzadas
rest.NewQueryParam("filter", rest.QueryParamTypeFilter)   // FilterBuilder completo
rest.NewQueryParam("where", rest.QueryParamTypeWhere)     // Solo condiciones WHERE

// Tipos básicos
rest.NewQueryParam("search", rest.QueryParamTypeString)   // string
rest.NewQueryParam("limit", rest.QueryParamTypeInt)       // int
rest.NewQueryParam("active", rest.QueryParamTypeBool)     // bool
rest.NewQueryParam("score", rest.QueryParamTypeFloat)     // float64
rest.NewQueryParam("date", rest.QueryParamTypeDate)       // time.Time (YYYY-MM-DD)
rest.NewQueryParam("created", rest.QueryParamTypeDateTime) // time.Time (RFC3339)
rest.NewQueryParam("id", rest.QueryParamTypeObjectID)     // bson.ObjectID
```

### Timeouts

```go
{
    Name:    "ProcessLongOperation",
    Method:  rest.MethodPOST,
    Path:    "/operations/long",
    Handler: ProcessLongOperation,
    Timeout: 30, // 30 segundos
}
```

### Validación del Body de Request

El framework incluye un sistema robusto de validación del body de las peticiones HTTP utilizando la interfaz `Validable` y el validador `go-playground/validator`.

#### Interfaz Validable

Todos los structs que se usan como body parameters deben implementar la interfaz `Validable`:

```go
type Validable interface {
    Validate(ctx *EndpointContext) error
}
```

#### Flujo de Validación

1. **Binding**: El framework usa Echo's binding para convertir el JSON del request al struct
2. **Validación**: Se llama al método `Validate()` del struct
3. **Manejo de Errores**: Los errores de validación se convierten a mensajes amigables

#### Ejemplo Básico

```go
type CreateUserRequest struct {
    Name     string `json:"name" validate:"required,min=3,max=100"`
    Email    string `json:"email" validate:"required,email"`
    Password string `json:"password" validate:"required,min=6,max=30"`
    Age      int    `json:"age" validate:"gte=18,lte=120"`
}

func (r *CreateUserRequest) Validate(ctx *rest.EndpointContext) error {
    return ctx.ValidateStruct(r)
}
```

#### Configuración del Endpoint

```go
{
    Name:       "CreateUser",
    Method:     rest.MethodPOST,
    Path:       "/users",
    Handler:    CreateUser,
    BodyParams: func() rest.Validable { return &CreateUserRequest{} },
}
```

#### Uso en el Handler

```go
func CreateItem(ctx *rest.EndpointContext) error {
    // El body ya está validado y parseado
    body := ctx.ParsedBody.(*CreateItemRequest)

    // Usar los datos validados
    item := MyModel{
        Name: body.Name,
        // ... otros campos
    }

    // ... lógica de creación
    return ctx.JSON(item, 201)
}
```

#### Tags de Validación Soportados

El framework usa `go-playground/validator/v10` que soporta una amplia gama de tags de validación. Para ver la lista completa de tags disponibles y su documentación, consulta:

- **Documentación oficial**: https://pkg.go.dev/github.com/go-playground/validator/v10
- **Lista de tags**: https://github.com/go-playground/validator#baked-in-validations

Algunos tags comunes incluyen: `required`, `email`, `min`, `max`, `len`, `gte`, `lte`, `oneof`, `omitempty`, entre muchos otros.

#### Validación Personalizada y Ejemplos Avanzados

Para validaciones más complejas que van más allá de los tags básicos, puedes agregar lógica personalizada en el método `Validate()`:

```go
// Ejemplo 1: Validación condicional
type LoginRequest struct {
    Username string `json:"username" validate:"omitempty,min=3,max=50"`
    Email    string `json:"email" validate:"omitempty,email"`
    Password string `json:"password" validate:"required,min=6,max=30"`
}

func (r *LoginRequest) Validate(ctx *rest.EndpointContext) error {
    // Validar que al menos username o email esté presente
    if r.Username == "" && r.Email == "" {
        return errors.New("field:username,required; field:email,required;")
    }

    return ctx.ValidateStruct(r)
}

// Ejemplo 2: Validación de campos opcionales para updates
type UpdateUserRequest struct {
    Name        *string `json:"name,omitempty" validate:"omitempty,min=3,max=100"`
    Email       *string `json:"email,omitempty" validate:"omitempty,email"`
    PhoneNumber *string `json:"phoneNumber,omitempty" validate:"omitempty,min=10"`
    IsActive    *bool   `json:"isActive,omitempty"`
}

func (r *UpdateUserRequest) Validate(ctx *rest.EndpointContext) error {
    // Validación estructural básica
    err := ctx.ValidateStruct(r)
    if err != nil {
        return err
    }

    // Validación de negocio: al menos un campo debe estar presente
    if r.Name == nil && r.Email == nil && r.PhoneNumber == nil && r.IsActive == nil {
        return rest.NewErrorResponse(422, "Unprocessable Entity",
            "At least one field must be provided for update")
    }

    return nil
}

// Ejemplo 3: Validación de tipos específicos como ObjectID
type ResetPasswordRequest struct {
    Password string `json:"password" validate:"required,min=6,max=30"`
    UserId   string `json:"userId" validate:"required"`
    Token    string `json:"token" validate:"required"`
}

func (r *ResetPasswordRequest) Validate(ctx *rest.EndpointContext) error {
    err := ctx.ValidateStruct(r)
    if err != nil {
        return err
    }

    // Validar formato de ObjectID
    if _, err := bson.ObjectIDFromHex(r.UserId); err != nil {
        return errors.Errorf("field:userId,objectId,Invalid user ID format: %s", r.UserId)
    }

    return nil
}
```

#### Manejo de Errores de Validación

El framework convierte automáticamente los errores de validación a mensajes amigables:

```json
{
  "error": "Validation failed",
  "details": {
    "name": "This field must have a minimum length of 3",
    "email": "This field must be a valid email",
    "age": "This field must be greater than or equal to 18"
  }
}
```

#### Campos Opcionales vs Requeridos

```go
type UserRequest struct {
    // Campo requerido
    Name string `json:"name" validate:"required,min=3"`

    // Campo opcional que se valida solo si está presente
    Email *string `json:"email,omitempty" validate:"omitempty,email"`

    // Campo opcional sin validación adicional
    Description *string `json:"description,omitempty"`
}
```

**Nota**: La validación del body solo se aplica a métodos HTTP que pueden tener body (POST, PUT, PATCH). Para métodos GET, HEAD, DELETE, el campo `BodyParams` es ignorado.

### Roles y Permisos

```go
// Definir permisos
type Permission struct {
    name string
}

func (p Permission) RoleName() string {
    return p.name
}

var (
    PermListItems   = Permission{"list_items"}
    PermCreateItem  = Permission{"create_item"}
    PermUpdateItem  = Permission{"update_item"}
    PermDeleteItem  = Permission{"delete_item"}
    PermReadItem    = Permission{"read_item"}
)

// Usar en endpoints
{
    Name:    "GetItems",
    Method:  rest.MethodGET,
    Path:    "/items",
    Handler: GetItems,
    Roles:   []rest.EndpointRole{PermListItems},
}
```

## Tipos de Parámetros

### Path Parameters

```go
rest.NewPathParam("id", rest.PathParamTypeObjectID)
rest.NewPathParam("count", rest.PathParamTypeInt)
```

### Query Parameters

```go
rest.NewQueryParam("filter", rest.QueryParamTypeFilter)          // Filtro MongoDB
rest.NewQueryParam("limit", rest.QueryParamTypeInt)              // Entero
rest.NewQueryParam("search", rest.QueryParamTypeString)          // String
rest.NewQueryParam("active", rest.QueryParamTypeBool)            // Boolean
rest.NewQueryParam("date", rest.QueryParamTypeDate)              // Fecha
```

### Header Parameters

```go
rest.NewHeaderParam("X-Custom-Header", rest.HeaderParamTypeString, true)
rest.NewHeaderParam("X-Request-ID", rest.HeaderParamTypeString, false)
```

## Tipos de Respuesta

El framework ofrece dos enfoques para enviar respuestas: con auditoría automática (`RespondAndLog`) y sin auditoría (respuestas directas).

### Respuestas con Auditoría (`RespondAndLog`)

Usar cuando necesites trazabilidad automática de la operación:

```go
// JSON (por defecto)
return ctx.RespondAndLog(data, modelId, rest.ResponseTypeJSON)

// XML
return ctx.RespondAndLog(data, modelId, rest.ResponseTypeXML)

// Texto plano
return ctx.RespondAndLog("Success", modelId, rest.ResponseTypeText)

// HTML
return ctx.RespondAndLog("<h1>Success</h1>", modelId, rest.ResponseTypeHTML)

// Sin contenido
return ctx.RespondAndLog(nil, modelId, rest.ResponseTypeNoContent, 204)

// Con código de estado personalizado
return ctx.RespondAndLog(data, modelId, rest.ResponseTypeJSON, 201)
```

### Respuestas Directas (Sin Auditoría)

Usar para operaciones simples que no requieren trazabilidad:

```go
// JSON directo
return ctx.JSON(data)
return ctx.JSON(data, 201) // Con código de estado

// XML directo
return ctx.XML(data)
return ctx.XML(data, 200)

// Texto plano
return ctx.Text("Success")
return ctx.Text("Success", 201)

// HTML
return ctx.HTML("<h1>Success</h1>")

// Sin contenido
return ctx.NoContent(204)

// Usando Echo directamente (máximo control)
return ctx.EchoCtx.JSON(200, data)
```

### Cuándo Usar Cada Tipo

| Método               | Cuándo Usar                                               | Auditoría |
| -------------------- | --------------------------------------------------------- | --------- |
| `RespondAndLog`      | Operaciones críticas, CRUD, acceso a datos sensibles      | ✅ Sí     |
| `JSON/XML/Text/HTML` | Health checks, endpoints simples, respuestas informativas | ❌ No     |
| `EchoCtx` directo    | Control total sobre la respuesta, casos especiales        | ❌ No     |

## Middleware Personalizado

```go
func MyMiddleware(next rest.HandlerFunc) rest.HandlerFunc {
    return func(ctx *rest.EndpointContext) error {
        // Lógica antes del handler
        ctx.App.Infof("Processing request to %s", ctx.Endpoint.Path)

        err := next(ctx)

        // Lógica después del handler
        ctx.App.Infof("Completed request to %s", ctx.Endpoint.Path)

        return err
    }
}

// Usar middleware en grupos
api := app.Group("/api", MyMiddleware)
```

## Logging

```go
// En handlers
ctx.App.Debugf("Debug message: %v", data)
ctx.App.Infof("Info message: %s", message)
ctx.App.Warnf("Warning: %s", warning)
ctx.App.Errorf("Error occurred: %v", err)
```

## Sistema de Modelos y Repositorios

### Datasource y Conectores

El `Datasource` es el componente central que gestiona las conexiones a la base de datos y el registro de modelos.

> **Nota**: Actualmente el framework solo soporta MongoDB como base de datos. El soporte para otros motores de base de datos **podría** implementarse en futuras versiones de ser necesario.

#### Configurar MongoDB con Variables de Entorno

```bash
# Variables de entorno requeridas
MONGO_URI=mongodb://localhost:27017
MONGO_DATABASE=mi_aplicacion
```

#### Configurar Datasource

```go
import (
    "github.com/xompass/vsaas-rest/database"
)

func main() {
    // Opción 1: Usar configuración por defecto (recomendado)
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

**Variables de entorno para `NewDefaultMongoConnector()`:**

- `MONGO_URI`: URI de conexión a MongoDB (default: `mongodb://localhost:27017`)
- `MONGO_DATABASE`: Nombre de la base de datos (default: `vsaas_dispatch`)

### Interfaz IModel

Todos los modelos deben implementar la interfaz `IModel`:

```go
type IModel interface {
    GetTableName() string     // Nombre de la tabla/colección en la base de datos
    GetModelName() string     // Nombre único del modelo para el registro
    GetConnectorName() string // Nombre del conector de base de datos a usar
    GetId() any              // ID del documento/registro
}
```

### Crear un Modelo

Ejemplo completo de un modelo que implementa `IModel`:

```go
package models

import (
    "time"
    "go.mongodb.org/mongo-driver/v2/bson"
    "github.com/xompass/vsaas-rest/database"
)

type User struct {
    ID       bson.ObjectID `json:"id" bson:"_id,omitempty"`
    Name     string        `json:"name" bson:"name"`
    Email    string        `json:"email" bson:"email"`
    Password string        `json:"-" bson:"password"` // "-" oculta el campo en JSON
    IsActive bool          `json:"isActive" bson:"isActive"`

    // Campos automáticos (opcionales)
    Created  *time.Time `json:"created,omitempty" bson:"created,omitempty"`
    Modified *time.Time `json:"modified,omitempty" bson:"modified,omitempty"`
    Deleted  *time.Time `json:"deleted,omitempty" bson:"deleted,omitempty"`
}

// Implementar la interfaz IModel
func (u User) GetId() any {
    return u.ID
}

func (u User) GetTableName() string {
    return "users" // Nombre de la colección en MongoDB
}

func (u User) GetModelName() string {
    return "User" // Nombre único para el registro
}

func (u User) GetConnectorName() string {
    return "mongodb" // Debe coincidir con el nombre del conector registrado
}
```

### Hooks del Modelo (Opcional)

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
func (u *User) BeforeCreate() error {
    // Hash password antes de guardar
    hashedPassword, err := bcrypt.GenerateFromPassword([]byte(u.Password), bcrypt.DefaultCost)
    if err != nil {
        return err
    }
    u.Password = string(hashedPassword)
    return nil
}

func (u *User) BeforeUpdate() error {
    // Validaciones antes de actualizar
    if u.Email == "" {
        return errors.New("email is required")
    }
    return nil
}
```

### Crear Repositorio

#### Repositorio Básico

```go
package repositories

import (
    "github.com/xompass/vsaas-rest/database"
    // Importar tus modelos según la estructura de tu proyecto
)

type MyModelRepository database.Repository[MyModel]

func NewMyModelRepository(ds *database.Datasource) (MyModelRepository, error) {
    // Crear repositorio con opciones
    repository, err := database.NewMongoRepository[MyModel](ds, database.RepositoryOptions{
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

#### Repositorio con Patrón Singleton (Ejemplo)

Si quieres evitar crear múltiples instancias del mismo repositorio:

```go
package repositories

import (
    "sync"
    "github.com/xompass/vsaas-rest/database"
)

type MyModelRepository database.Repository[MyModel]

var myModelRepository MyModelRepository
var myModelRepositoryLock = &sync.Mutex{}

func NewMyModelRepository(ds *database.Datasource) (MyModelRepository, error) {
    myModelRepositoryLock.Lock()
    defer myModelRepositoryLock.Unlock()

    // Singleton: reutilizar si ya existe
    if myModelRepository != nil {
        return myModelRepository, nil
    }

    _repository, err := database.NewMongoRepository[MyModel](ds, database.RepositoryOptions{
        Created:  true,
        Modified: true,
        Deleted:  true,
    })

    if err != nil {
        return nil, err
    }

    myModelRepository = _repository
    return myModelRepository, nil
}
```

### Inicializar Repositorios

#### 1. Crear función de inicialización

```go
package repositories

import (
    rest "github.com/xompass/vsaas-rest"
    "github.com/xompass/vsaas-rest/database"
)

func Init(app *rest.RestApp, datasource *database.Datasource) error {
    // Crear y registrar todos los repositorios al inicio de la aplicación
    _, err := NewMyModelRepository(datasource)
    if err != nil {
        return err
    }

    // Agregar más repositorios aquí según tus modelos...
    // _, err = NewOtherModelRepository(datasource)
    // if err != nil {
    //     return err
    // }

    return nil
}
```

#### 2. Llamar desde main.go

```go
import (
    "your-project/repositories" // Importar según la estructura de tu proyecto
)

func main() {
    // ... configurar datasource y app ...

    // Inicializar todos los repositorios
    err = repositories.Init(app, &datasource)
    if err != nil {
        panic(err)
    }

    // ... registrar endpoints y iniciar servidor ...
}
```

### Usar Repositorios en Handlers

```go
func GetItems(ctx *rest.EndpointContext) error {
    // Obtener repositorio registrado desde el datasource
    repo, err := database.GetDatasourceModelRepository(ctx.App.Datasource, MyModel{})
    if err != nil {
        return rest.NewErrorResponse(500, "Database error")
    }

    filter, err := ctx.GetFilterParam()
    if err != nil {
        return err
    }

    items, err := repo.Find(ctx.Context(), filter)
    if err != nil {
        return rest.NewErrorResponse(500, "Failed to fetch items")
    }

    return ctx.RespondAndLog(items, nil, rest.ResponseTypeJSON)
}
```

### Opciones de Repositorio

```go
type RepositoryOptions struct {
    Created  bool // Maneja automáticamente timestamps de creación
    Modified bool // Maneja automáticamente timestamps de modificación
    Deleted  bool // Habilita soft delete (no elimina físicamente)
}

// Ejemplo con todas las opciones
repo, err := database.NewMongoRepository[MyModel](ds, database.RepositoryOptions{
    Created:  true,  // Establece "created" automáticamente al insertar
    Modified: true,  // Actualiza "modified" automáticamente al modificar
    Deleted:  true,  // Soft delete: marca "deleted" en lugar de eliminar
})
```

### Operaciones de Repositorio

```go
// Buscar todos los registros
items, err := repo.Find(ctx, nil)

// Buscar con filtro programático
filter := database.NewFilter().WithWhere(database.NewWhere().Eq("isActive", true))
items, err := repo.Find(ctx, filter)

// Buscar uno
item, err := repo.FindOne(ctx, filter)

// Buscar por ID
item, err := repo.FindById(ctx, itemID, nil)

// Crear
newItem := MyModel{Name: "My Item"}
insertID, err := repo.Insert(ctx, newItem)

// Crear y retornar el documento creado
createdItem, err := repo.Create(ctx, newItem)

// Actualizar uno
err = repo.UpdateOne(ctx, filter, bson.M{"$set": bson.M{"name": "Updated Name"}})

// Actualizar por ID
err = repo.UpdateById(ctx, itemID, bson.M{"$set": bson.M{"name": "Updated Name"}})

// Buscar y actualizar
updatedItem, err := repo.FindOneAndUpdate(ctx, filter, bson.M{"$set": bson.M{"name": "Updated Name"}})

// Contar documentos
count, err := repo.Count(ctx, filter)

// Verificar existencia
exists, err := repo.Exists(ctx, itemID)

// Eliminar (soft delete si está habilitado)
err = repo.DeleteById(ctx, itemID)

// Eliminar múltiples
deletedCount, err := repo.DeleteMany(ctx, filter)
```

## Dependencias para Ejemplos

Para usar los ejemplos completos, necesitarás estas dependencias adicionales:

```go
require (
    golang.org/x/crypto v0.38.0  // Para bcrypt
    github.com/golang-jwt/jwt/v5 v5.0.0  // Para JWT tokens
)
```

## Estructura de Proyecto Recomendada

```
my-api/
├── main.go
├── controllers/
│   ├── user/
│   │   ├── register.go
│   │   ├── user_controller.go
│   │   └── helpers.go
│   └── product/
│       ├── register.go
│       └── product_controller.go
├── models/
│   ├── user.go
│   ├── product.go
│   └── audit_log.go
├── repositories/
│   └── init.go
└── services/
    └── auth/
        └── auth.go
```

## Contribución

Este framework está diseñado para ser extensible y mantenible. Para contribuir:

1. Mantén la compatibilidad con las interfaces existentes
2. Agrega tests para nuevas funcionalidades
3. Documenta cambios en la API
4. Sigue las convenciones de Go

## Licencia

Consulta el archivo LICENSE para más detalles.
