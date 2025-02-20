package log

import (
	"log"
	"os"
)

var infoLog *log.Logger = log.New(os.Stdout, "INFO: ", log.Ldate|log.Ltime)
var errorLog *log.Logger = log.New(os.Stderr, "ERROR: ", log.Ldate|log.Ltime)

func Infof(v ...interface{}) {
	if len(v) == 1 {
		infoLog.Println(v...)
		return
	}
	if r, ok := v[0].(string); ok {
		infoLog.Printf(r, v[1:]...)
		return
	}
	errorLog.Println("некорректный формат лога")
}

func Errorf(v ...interface{}) {
	if len(v) == 1 {
		errorLog.Println(v...)
		return
	}
	if r, ok := v[0].(string); ok {
		errorLog.Printf(r, v[1:]...)
		return
	}
	errorLog.Println("некорректный формат лога")
}
