package log

import (
	"log"
	"os"
)

var InfoLog *log.Logger = log.New(os.Stdout, "INFO: ", log.Ldate|log.Ltime)
var ErrorLog *log.Logger = log.New(os.Stderr, "ERROR: ", log.Ldate|log.Ltime|log.Lshortfile)
