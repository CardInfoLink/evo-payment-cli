package output

// Exit code constants for the CLI process.
const (
	ExitSuccess       = 0 // S prefix — success (including S0003 "User paying" etc.)
	ExitBusinessError = 1 // B, C prefix — Evo Payment business/resource error
	ExitValidation    = 2 // V prefix (except V0010) — parameter validation failure
	ExitAuthError     = 3 // V0010, signature verification failure — auth/signature error
	ExitNetworkError  = 4 // E prefix, HTTP non-200, network timeout — network/system error
	ExitCLIError      = 5 // CLI internal bug
	ExitPSPError      = 6 // P, I prefix — PSP/issuer error
)
