package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/kennygrant/sanitize"
	"gopkg.in/mcuadros/go-syslog.v2"
)

var (
	logFiles = make(map[string]*os.File) // filename => os.File
)

func logToFile(filename, text string) {
	logf, ok := logFiles[filename]
	if !ok {
		os.MkdirAll(filepath.Dir(filename), 0700)
		f, err := os.OpenFile(filename, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0700)
		if err != nil {
			fmt.Printf("ERROR: Could not create logfile %s: %s\n", filename, err)
			return
		}

		logFiles[filename] = f
		logf = f
	}

	logf.WriteString(text)
	logf.Sync()
}

func main() {
	channel := make(syslog.LogPartsChannel)
	handler := syslog.NewChannelHandler(channel)

	server := syslog.NewServer()
	server.SetFormat(syslog.RFC3164)
	server.SetHandler(handler)
	server.ListenUDP("0.0.0.0:5514")
	server.ListenTCP("0.0.0.0:5514")
	server.Boot()

	go func(channel syslog.LogPartsChannel) {
		for logParts := range channel {
			text := logParts["content"].(string)
			var app string
			if strings.Count(text, " ") >= 3 {
				app = strings.Split(text, " ")[2]
				if strings.Contains(app, "[") {
					app = app[:strings.Index(app, "[")]
				}
				text = strings.Join(strings.Split(text, " ")[3:], " ")
			}

			app = sanitize.BaseName(app)
			timestamp := logParts["timestamp"].(time.Time)
			hostname := sanitize.BaseName(logParts["hostname"].(string))
			logs := "logs"
			hostlogs := filepath.Join(logs, hostname)
			filename := filepath.Join(hostlogs, app)

			logtext := fmt.Sprintf("%s %s\n", timestamp.Format("Mon Jan 2 2006 15:04:05"), text)
			fmt.Println(logtext)
			logToFile(filename+".log", logtext)
			logToFile(hostlogs+".log", logtext)

			switch logParts["severity"].(int) {
			case 4:
				logToFile(filename+"_warning.log", logtext)
				logToFile(hostlogs+"_warning.log", logtext)
				logToFile(filepath.Join(logs, "warning.log"), logtext)
			case 3:
				logToFile(filename+"_error.log", logtext)
				logToFile(hostlogs+"_error.log", logtext)
				logToFile(filepath.Join(logs, "error.log"), logtext)
			}
		}
	}(channel)

	server.Wait()
}
