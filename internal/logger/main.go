/*
* -----------------------------------------------------------
* In this package - Logging
* - Output strategy (stdout/file/external)
* - prefix logging
* - Informational logging
* -----------------------------------------------------------
* @see more
*  - https://pkg.go.dev/log
 */
//

package logger

import (
	"log"
	"os"

	"github.com/fatih/color"
)

var (
	LOG_LEVEL string
)

// // @todo use the custom file path
func toFile() *os.File {

	file, err := os.OpenFile("logger.out", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0666)

	if err != nil {
		log.Fatal(err)
	}

	defer file.Close()

	log.SetOutput(file)
	return file
}

// Custom
type Logger interface {
	// add the rest of the methods if necessary
	Debug(format string, args ...interface{})
	Debugf(format string, args ...interface{})
}

type BuiltinLogger struct {
	logger *log.Logger
}

func Create(level string) *BuiltinLogger {
	LOG_LEVEL = level
	// make it better
	// var flagNoColor = flag.Bool("no-color", false, "Disable color output")
	// flag.Parse()
	noColorEnvVal, _ := os.LookupEnv("NO_COLOR")
	if noColorEnvVal == "true" {
		color.NoColor = true // disables colorized output
	}

	out := os.Stdout                                   // or toFile()
	return &BuiltinLogger{logger: log.New(out, "", 0)} //  log.Ldate|log.Ltime|log.Lshortfile
}

func (l *BuiltinLogger) Warn(args ...interface{}) {
	l.logger.SetPrefix(color.YellowString("WARN: "))
	l.logger.Println(args...)
}

func (l *BuiltinLogger) Info(args ...interface{}) {
	l.logger.SetPrefix(color.CyanString("INFO: "))
	l.logger.Println(args...)
}

func (l *BuiltinLogger) Infof(format string, args ...interface{}) {
	l.logger.SetPrefix(color.CyanString("INFO: "))
	l.logger.Printf(format, args...)
}

func (l *BuiltinLogger) Fatal(args ...interface{}) {
	l.logger.SetPrefix(color.RedString("FATAL: "))
	l.logger.Fatalln(args...)
}
func (l *BuiltinLogger) Fatalf(format string, args ...interface{}) {
	l.logger.SetPrefix(color.RedString("FATAL: "))
	l.logger.Fatalf(format, args...)
}

func (l *BuiltinLogger) Success(args ...interface{}) {
	l.logger.SetPrefix(color.GreenString("DONE: "))
	l.logger.Println(args...)
}

func (l *BuiltinLogger) Error(args ...interface{}) {
	l.logger.SetPrefix(color.RedString("ERROR: "))
	l.logger.Println(args...)
}

func (l *BuiltinLogger) Debug(args ...interface{}) {
	if LOG_LEVEL == "debug" {
		l.logger.SetPrefix(color.MagentaString("DEBUG:  "))
		l.logger.Println(args...)
	}
}

func (l *BuiltinLogger) Debugf(format string, args ...interface{}) {
	if LOG_LEVEL == "debug" {
		l.logger.SetPrefix(color.MagentaString("DEBUG: "))
		l.logger.Printf(format, args...)
	}
}

// move to the ui package
func (l *BuiltinLogger) Bot(args ...interface{}) {
	l.logger.SetPrefix(color.BlueString("\\[._.]/ : "))
	l.logger.Println(args...)
}
