package rest

type ResponseType string

const (
	ResponseTypeJSON      ResponseType = "json"
	ResponseTypeXML       ResponseType = "xml"
	ResponseTypeText      ResponseType = "text"
	ResponseTypeHTML      ResponseType = "html"
	ResponseTypeNoContent ResponseType = "no_content"
	ResponseTypeFile      ResponseType = "file"
)

type EndpointMethod string

const (
	MethodHEAD   EndpointMethod = "Head"
	MethodGET    EndpointMethod = "Get"
	MethodPOST   EndpointMethod = "Post"
	MethodPUT    EndpointMethod = "Put"
	MethodPATCH  EndpointMethod = "Patch"
	MethodDELETE EndpointMethod = "Delete"
)

type ParamLocation string

const (
	InQuery  ParamLocation = "query"
	InPath   ParamLocation = "path"
	InHeader ParamLocation = "header"
)

type PathParamType string

const (
	PathParamTypeString   PathParamType = "string"
	PathParamTypeInt      PathParamType = "int"
	PathParamTypeFloat    PathParamType = "float"
	PathParamTypeBool     PathParamType = "bool"
	PathParamTypeDate     PathParamType = "date"
	PathParamTypeDateTime PathParamType = "datetime"
	PathParamTypeObjectID PathParamType = "objectid"
)

type QueryParamType string

const (
	QueryParamTypeString   QueryParamType = "string"
	QueryParamTypeInt      QueryParamType = "int"
	QueryParamTypeFloat    QueryParamType = "float"
	QueryParamTypeBool     QueryParamType = "bool"
	QueryParamTypeDate     QueryParamType = "date"
	QueryParamTypeDateTime QueryParamType = "datetime"
	QueryParamTypeObjectID QueryParamType = "objectid"
	QueryParamTypeFilter   QueryParamType = "filter"
	QueryParamTypeWhere    QueryParamType = "where"
)

type HeaderParamType string

const (
	HeaderParamTypeString   HeaderParamType = "string"
	HeaderParamTypeInt      HeaderParamType = "int"
	HeaderParamTypeFloat    HeaderParamType = "float"
	HeaderParamTypeBool     HeaderParamType = "bool"
	HeaderParamTypeDate     HeaderParamType = "date"
	HeaderParamTypeDateTime HeaderParamType = "datetime"
	HeaderParamTypeObjectID HeaderParamType = "objectid"
	HeaderParamTypeFilter   HeaderParamType = "filter"
	HeaderParamTypeWhere    HeaderParamType = "where"
)

type ActionType string

const (
	ActionTypeRead           ActionType = "read"
	ActionTypeCreate         ActionType = "create"
	ActionTypeUpdate         ActionType = "update"
	ActionTypeDelete         ActionType = "delete"
	ActionTypeLogin          ActionType = "login"
	ActionTypeLogout         ActionType = "logout"
	ActionTypeResetPassword  ActionType = "reset_password"
	ActionTypeChangePassword ActionType = "change_password"
	ActionTypeFileUpload     ActionType = "file_upload"
)

type LogLevel uint8

const (
	LogLevelDebug LogLevel = iota
	LogLevelInfo
	LogLevelWarn
	LogLevelError
)

var LogLevelLabels = map[LogLevel]string{
	LogLevelDebug: "DEBUG",
	LogLevelInfo:  "INFO",
	LogLevelWarn:  "WARN",
	LogLevelError: "ERROR",
}

// FileUpload types and configurations
type FileExtension string

const (
	// Image formats
	FileExtensionJPEG FileExtension = ".jpeg"
	FileExtensionJPG  FileExtension = ".jpg"
	FileExtensionPNG  FileExtension = ".png"
	FileExtensionGIF  FileExtension = ".gif"
	FileExtensionWEBP FileExtension = ".webp"
	FileExtensionSVG  FileExtension = ".svg"
	FileExtensionBMP  FileExtension = ".bmp"
	FileExtensionTIFF FileExtension = ".tiff"

	// Document formats
	FileExtensionPDF  FileExtension = ".pdf"
	FileExtensionDOC  FileExtension = ".doc"
	FileExtensionDOCX FileExtension = ".docx"
	FileExtensionXLS  FileExtension = ".xls"
	FileExtensionXLSX FileExtension = ".xlsx"
	FileExtensionPPT  FileExtension = ".ppt"
	FileExtensionPPTX FileExtension = ".pptx"
	FileExtensionTXT  FileExtension = ".txt"
	FileExtensionRTF  FileExtension = ".rtf"
	FileExtensionODT  FileExtension = ".odt"
	FileExtensionODS  FileExtension = ".ods"
	FileExtensionODP  FileExtension = ".odp"

	// Archive formats
	FileExtensionZIP FileExtension = ".zip"
	FileExtensionRAR FileExtension = ".rar"
	FileExtension7Z  FileExtension = ".7z"
	FileExtensionTAR FileExtension = ".tar"
	FileExtensionGZ  FileExtension = ".gz"

	// Video formats
	FileExtensionMP4  FileExtension = ".mp4"
	FileExtensionAVI  FileExtension = ".avi"
	FileExtensionMOV  FileExtension = ".mov"
	FileExtensionWMV  FileExtension = ".wmv"
	FileExtensionFLV  FileExtension = ".flv"
	FileExtensionMKV  FileExtension = ".mkv"
	FileExtensionWEBM FileExtension = ".webm"

	// Audio formats
	FileExtensionMP3  FileExtension = ".mp3"
	FileExtensionWAV  FileExtension = ".wav"
	FileExtensionFLAC FileExtension = ".flac"
	FileExtensionAAC  FileExtension = ".aac"
	FileExtensionOGG  FileExtension = ".ogg"
	FileExtensionWMA  FileExtension = ".wma"

	// Code formats
	FileExtensionJS   FileExtension = ".js"
	FileExtensionTS   FileExtension = ".ts"
	FileExtensionPY   FileExtension = ".py"
	FileExtensionGO   FileExtension = ".go"
	FileExtensionJAVA FileExtension = ".java"
	FileExtensionC    FileExtension = ".c"
	FileExtensionCPP  FileExtension = ".cpp"
	FileExtensionCSS  FileExtension = ".css"
	FileExtensionHTML FileExtension = ".html"
	FileExtensionXML  FileExtension = ".xml"
	FileExtensionJSON FileExtension = ".json"
	FileExtensionYAML FileExtension = ".yaml"
	FileExtensionYML  FileExtension = ".yml"

	// Other formats
	FileExtensionCSV FileExtension = ".csv"
)

// Predefined file type groups
var (
	ImageExtensions = []FileExtension{
		FileExtensionJPEG, FileExtensionJPG, FileExtensionPNG, FileExtensionGIF,
		FileExtensionWEBP, FileExtensionSVG, FileExtensionBMP, FileExtensionTIFF,
	}

	DocumentExtensions = []FileExtension{
		FileExtensionPDF, FileExtensionDOC, FileExtensionDOCX, FileExtensionXLS,
		FileExtensionXLSX, FileExtensionPPT, FileExtensionPPTX, FileExtensionTXT,
		FileExtensionRTF, FileExtensionODT, FileExtensionODS, FileExtensionODP,
	}

	ArchiveExtensions = []FileExtension{
		FileExtensionZIP, FileExtensionRAR, FileExtension7Z,
		FileExtensionTAR, FileExtensionGZ,
	}

	VideoExtensions = []FileExtension{
		FileExtensionMP4, FileExtensionAVI, FileExtensionMOV, FileExtensionWMV,
		FileExtensionFLV, FileExtensionMKV, FileExtensionWEBM,
	}

	AudioExtensions = []FileExtension{
		FileExtensionMP3, FileExtensionWAV, FileExtensionFLAC,
		FileExtensionAAC, FileExtensionOGG, FileExtensionWMA,
	}

	CodeExtensions = []FileExtension{
		FileExtensionJS, FileExtensionTS, FileExtensionPY, FileExtensionGO,
		FileExtensionJAVA, FileExtensionC, FileExtensionCPP, FileExtensionCSS,
		FileExtensionHTML, FileExtensionXML, FileExtensionJSON, FileExtensionYAML,
		FileExtensionYML,
	}
)
