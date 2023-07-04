package sysPrint

import (
	"errors"
	"log"
	"os"
)

const (
	SYSTEM = "[SYSTEM]:"
	ERROR  = "[ERROR]:"
	FATAL  = "[FATAL]"
)

var (
	ErrUnknownLoadBalancer        = ErrorMsg("Unknown load balancer type.")
	ErrNoServer                   = ErrorMsg("No available servers.")
	ErrServerExists               = ErrorMsg("Server already exists.")
	ErrServerNotExists            = ErrorMsg("Server does not exists.")
	ErrServerWeightNegative       = ErrorMsg("Server weight cannot be negative.")
	ErrServerWeightGreaterThanMax = ErrorMsg("Server weight cannot greater than max limit 1000000.")
	ErrServerAddrInvalid          = ErrorMsg("Server address invalid.")
	ErrServerProbeInvalid         = ErrorMsg("Server probe invalid, probe must have an HTTP scheme, for example: http://127.0.0.1:8081/check/")
)

var (
	logFile *os.File
)

func init() {
	var err error
	logFile, err = os.OpenFile("log.txt", os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	if err != nil {
		log.Fatal(err)
	}
}

func ErrorMsg(msg string) error {
	return errors.New(ERROR + msg)
}

func PrintlnErrorMsg(msg string) {
	log.SetOutput(os.Stderr)
	log.Println(ERROR + msg)
}

func PrintlnAndLogWriteErrorMsg(msg string) {
	log.SetOutput(os.Stderr)
	log.Println(SYSTEM + msg)
	log.SetOutput(logFile)
	log.Println(SYSTEM + msg)
}

func LogWriteErrorMsg(msg string) {
	log.SetOutput(logFile)
	log.Println(ERROR + msg)
}

func SystemMsg(msg string) string {
	return SYSTEM + msg
}

func PrintlnSystemMsg(msg string) {
	log.SetOutput(os.Stderr)
	log.Println(SYSTEM + msg)
}

func PrintlnAndLogWriteSystemMsg(msg string) {
	log.SetOutput(os.Stderr)
	log.Println(SYSTEM + msg)
	log.SetOutput(logFile)
	log.Println(SYSTEM + msg)
}

func LogWriteSystemMsg(msg string) {
	log.SetOutput(logFile)
	log.Println(SYSTEM + msg)
}

func FatalMsg(msg string) string {
	return FATAL + msg
}

func PrintlnFatalMsg(msg string) string {
	return FATAL + msg
}

func PrintlnAndLogWriteFatalMsg(msg string) {
	log.SetOutput(os.Stderr)
	log.Println(FATAL + msg)
	log.SetOutput(logFile)
	log.Println(FATAL + msg)
}

func LogWriteFatalMsg(msg string) {
	log.SetOutput(logFile)
	log.Println(FATAL + msg)
}

func LogClose() {
	LogWriteSystemMsg("log close...")
	logFile.Close()
}
