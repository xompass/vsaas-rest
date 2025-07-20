# Sistema de Subida de Archivos para Echo Framework

Este sistema proporciona capacidades avanzadas de subida de archivos usando Echo framework, con validación de streaming para evitar cargar archivos grandes en memoria.

## Características

- **Streaming de archivos**: Valida el tamaño de archivos durante la subida, no después
- **Límites por tipo de archivo**: Configura límites de tamaño específicos por extensión
- **Límites por campo**: Configura límites diferentes para cada campo del formulario
- **Validación de tipos**: Restringe qué tipos de archivos pueden ser subidos por campo
- **Limpieza automática**: Los archivos temporales se eliminan automáticamente después de la respuesta
- **Múltiples archivos**: Soporte para múltiples archivos por campo

## Configuración

### Configuración Global de Archivos

```go
fileUploadConfig := &rest.FileUploadConfig{
    MaxFileSize: 10 * 1024 * 1024, // 10MB por defecto
    TypeSizeLimits: map[rest.FileExtension]int64{
        // Límites por tipo de archivo
        rest.FileExtensionJPEG: 5 * 1024 * 1024,  // 5MB para JPEG
        rest.FileExtensionPDF:  20 * 1024 * 1024, // 20MB para PDF
    },
    UploadPath:         "./uploads",  // Directorio para archivos permanentes
    TempPath:           "./temp",     // Directorio para archivos temporales
    KeepFilesAfterSend: false,        // false = eliminar después de respuesta
}
```

### Configuración por Campo

```go
fileUploadConfig.FileFields = map[string]*rest.FileFieldConfig{
    "avatar": {
        FieldName:    "avatar",
        Required:     true,                    // Campo obligatorio
        MaxFileSize:  3 * 1024 * 1024,       // 3MB límite específico
        AllowedTypes: []rest.FileExtension{   // Solo imágenes
            rest.FileExtensionJPEG,
            rest.FileExtensionJPG,
            rest.FileExtensionPNG,
        },
        MaxFiles: 1, // Solo un archivo
    },
    "documents": {
        FieldName:    "documents",
        Required:     false,
        MaxFiles:     5,                      // Hasta 5 documentos
        AllowedTypes: []rest.FileExtension{   // Solo documentos
            rest.FileExtensionPDF,
            rest.FileExtensionDOC,
            rest.FileExtensionDOCX,
        },
        // Límites específicos para este campo que anulan los globales
        TypeSizeLimits: map[rest.FileExtension]int64{
            rest.FileExtensionPDF: 25 * 1024 * 1024, // 25MB para PDFs en documentos
        },
    },
}
```

## Uso en Endpoints

### 1. Crear Endpoint con Soporte de Archivos

```go
endpoint := &rest.Endpoint{
    Name:             "Upload Files",
    Method:           rest.MethodPOST,
    Path:             "/upload",
    Handler:          UploadHandler,
    FileUploadConfig: fileUploadConfig, // Asociar configuración
    Public:           true,
}
```

### 2. Implementar Handler

```go
func UploadHandler(ctx *rest.EndpointContext) error {
    // Verificar si se subieron archivos
    if !ctx.HasUploadedFiles("avatar") {
        return ctx.JSON(map[string]string{
            "error": "Falta el archivo avatar",
        }, 400)
    }

    // Obtener el primer archivo del campo "avatar"
    avatarFile := ctx.GetFirstUploadedFile("avatar")

    // Obtener todos los archivos del campo "documents"
    documentFiles := ctx.GetUploadedFiles("documents")

    // Obtener todos los archivos subidos
    allFiles := ctx.GetAllUploadedFiles()

    // Procesar archivos...
    return ctx.JSON(map[string]any{
        "message": "Archivos subidos exitosamente",
        "avatar": map[string]any{
            "original_name": avatarFile.OriginalName,
            "filename":      avatarFile.Filename,
            "size":          avatarFile.Size,
            "extension":     avatarFile.Extension,
            "mime_type":     avatarFile.MimeType,
            "path":          avatarFile.Path,
        },
        "document_count": len(documentFiles),
    })
}
```

## Tipos de Archivo Soportados

El sistema incluye constantes predefinidas para los tipos de archivo más comunes:

### Imágenes

- `FileExtensionJPEG` (.jpeg)
- `FileExtensionJPG` (.jpg)
- `FileExtensionPNG` (.png)
- `FileExtensionGIF` (.gif)
- `FileExtensionWEBP` (.webp)
- `FileExtensionSVG` (.svg)
- `FileExtensionBMP` (.bmp)
- `FileExtensionTIFF` (.tiff)

### Documentos

- `FileExtensionPDF` (.pdf)
- `FileExtensionDOC` (.doc)
- `FileExtensionDOCX` (.docx)
- `FileExtensionXLS` (.xls)
- `FileExtensionXLSX` (.xlsx)
- `FileExtensionPPT` (.ppt)
- `FileExtensionPPTX` (.pptx)

### Archivos de Texto

- `FileExtensionTXT` (.txt)
- `FileExtensionCSV` (.csv)
- `FileExtensionRTF` (.rtf)

### Audio y Video

- `FileExtensionMP3` (.mp3)
- `FileExtensionMP4` (.mp4)
- `FileExtensionAVI` (.avi)
- `FileExtensionMOV` (.mov)

## Estructura UploadedFile

Cada archivo subido se representa con la estructura `UploadedFile`:

```go
type UploadedFile struct {
    FieldName    string `json:"field_name"`    // Nombre del campo del formulario
    OriginalName string `json:"original_name"` // Nombre original del archivo
    Filename     string `json:"filename"`      // Nombre único generado
    Size         int64  `json:"size"`          // Tamaño en bytes
    Extension    string `json:"extension"`     // Extensión del archivo (.jpg, .pdf, etc.)
    MimeType     string `json:"mime_type"`     // Tipo MIME
    Path         string `json:"path"`          // Ruta donde se guardó el archivo
    TempPath     string `json:"temp_path"`     // Ruta temporal (si se usa)
}
```

## Validaciones

### Validación de Tamaño

El sistema valida el tamaño de archivo **durante la subida** usando streaming. Si un archivo excede el límite, la transferencia se termina inmediatamente sin cargar el archivo completo en memoria.

### Jerarquía de Límites

1. **Límite específico por tipo y campo** (TypeSizeLimits en FileFieldConfig)
2. **Límite global por tipo** (TypeSizeLimits en FileUploadConfig)
3. **Límite específico por campo** (MaxFileSize en FileFieldConfig)
4. **Límite global** (MaxFileSize en FileUploadConfig)

### Validación de Tipos

- Si `AllowedTypes` está vacío, se permiten todos los tipos
- Si `AllowedTypes` está definido, solo se permiten esos tipos específicos

## Manejo de Errores

El sistema devuelve errores HTTP apropiados:

- **400 Bad Request**: Archivo sin extensión, tipo no permitido, campo requerido faltante
- **413 Request Entity Too Large**: Archivo excede límite de tamaño
- **415 Unsupported Media Type**: Tipo de archivo no permitido
- **500 Internal Server Error**: Error al crear o escribir archivo

## Ejemplo de Request

```bash
curl -X POST http://localhost:8080/api/v1/upload \
  -F "avatar=@/path/to/profile.jpg" \
  -F "documents=@/path/to/document1.pdf" \
  -F "documents=@/path/to/document2.docx"
```

## Limpieza de Archivos

- Si `KeepFilesAfterSend` es `false`: Los archivos se guardan en `TempPath` y se eliminan después de enviar la respuesta
- Si `KeepFilesAfterSend` es `true`: Los archivos se guardan en `UploadPath` y persisten

## Consideraciones de Rendimiento

- El sistema usa buffers de 32KB para lectura streaming
- Los archivos se validan por tamaño antes de escribirse al disco
- Los archivos temporales se limpian en goroutines separadas para no bloquear la respuesta

## Seguridad

- **Validación de extensiones**: Previene subida de archivos ejecutables
- **Límites de tamaño**: Previene ataques de denegación de servicio por archivos grandes
- **Nombres únicos**: Usa UUIDs para evitar conflictos y ataques de path traversal
- **Cleanup automático**: Evita acumulación de archivos temporales
