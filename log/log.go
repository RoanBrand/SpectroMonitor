package log

import (
	"log"

	"gopkg.in/natefinch/lumberjack.v2"
)

// Print out to console if serviceMode = false
func Setup(logFilePath string, serviceMode bool) {
	if serviceMode {
		log.SetOutput(&lumberjack.Logger{
			Filename:   logFilePath,
			MaxBackups: 3,
			MaxAge:     28, //days
		})
	}
}

func Println(v ...interface{}) {
	log.Println(v...)
}

func Printf(format string, v ...interface{}) {
	log.Printf(format, v...)
}

func Fatal(v ...interface{}) {
	log.Fatal(v...)
}
