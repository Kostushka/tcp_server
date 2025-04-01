package log

import (
	"log"
	"os"
)

var infoLog *log.Logger
var errorLog *log.Logger

const permissions = 0644

// создаем логеры
func New(logFile string) error {
	// создаем логеры, пишущие в stdout
	if logFile == "" {
		infoLog = log.New(os.Stdout, "INFO: ", log.Ldate|log.Ltime)
		errorLog = log.New(os.Stderr, "ERROR: ", log.Ldate|log.Ltime)
		return nil
	}
	// создаем файл для записи лога
	f, err := os.OpenFile(logFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, permissions)
	if err != nil {
		return err
	}
	// создаем логеры, пишущие в файл
	infoLog = log.New(f, "INFO: ", log.Ldate|log.Ltime)
	errorLog = log.New(f, "ERROR: ", log.Ldate|log.Ltime)
	return nil
}

// пишет информационный лог
func Infof(v ...any) {
	// строка лога без аргументов
	if len(v) == 1 {
		infoLog.Println(v...)
		return
	}
	// строка лога с аргументами
	if r, ok := v[0].(string); ok {
		infoLog.Printf(r, v[1:]...)
		return
	}
	errorLog.Println("некорректный формат лога")
}

// пишет лог ошибки
func Errorf(v ...any) {
	// строка лога без аргументов
	if len(v) == 1 {
		errorLog.Println(v...)
		return
	}
	// строка лога с аргументами
	if r, ok := v[0].(string); ok {
		errorLog.Printf(r, v[1:]...)
		return
	}
	errorLog.Println("некорректный формат лога")
}
