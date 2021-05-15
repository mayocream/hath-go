package hath

const (
	ClientVersion = "1.6.1#go"
	// ClientBuild is among other things used by the server to determine the client's capabilities. any forks should use the build number as an indication of compatibility with mainline, rather than an internal build number.
	ClientBuild       = 154
	ClientKeyLength   = 20
	MaxKeyTimeDrift   = 300
	MaxConnectionBase = 20
	TCPPacketSize     = 1460

	ClientRPCProtocol   = "http"
	ClientRPCHost       = "rpc.hentaiathome.net"
	ClientRPCFile       = "15/rpc"
	ClientLoginFilename = "CLIENT_LOGIN_FILENAME"

	ContentTypeDefault = "text/html; charset=ISO-8859-1"
	ContentTypeOctet   = "application/octet-stream"
	ContentTypeJPG     = "image/jpeg"
	ContentTypePNG     = "image/png"
	ContentTypeGIF     = "image/gif"
	ContentTypeWEBM    = "video/webm"
)

type RPCStatus int

var (
	ResponseStatusNull RPCStatus = 0
	ResponseStatusOK   RPCStatus = 1
	ResponseStatusFail RPCStatus = -1
)

type Action string

var (
	ActionServerStat           Action = "server_stat"
	ActionGetBlacklist         Action = "get_blacklist"
	ActionGetCertificate       Action = "get_cert"
	ActionClientLogin          Action = "client_login"
	ActionClientSettings       Action = "client_settings"
	ActionClientStart          Action = "client_start"
	ActionClientSuspend        Action = "client_suspend"
	ActionClientResume         Action = "client_resume"
	ActionClientStop           Action = "client_stop"
	ActionStillAlive           Action = "still_alive"
	ActionStaticRangeFetch     Action = "srfetch"
	ActionDownloaderFetch      Action = "dlfetch"
	ActionDownloaderFailreport Action = "dlfails"
	ActionOverload             Action = "overload"
)
