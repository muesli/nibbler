package main

import (
	"encoding/json"
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

func logIt(host, app, text string, level int) {
	logs := "logs"
	hostlogs := filepath.Join(logs, host)
	applogs := filepath.Join(hostlogs, app)

	logToFile(applogs+".log", text)
	logToFile(hostlogs+".log", text)

	switch level {
	case 4:
		logToFile(applogs+"_warning.log", text)
		logToFile(hostlogs+"_warning.log", text)
		logToFile(filepath.Join(logs, "warning.log"), text)
	case 3:
		logToFile(applogs+"_error.log", text)
		logToFile(hostlogs+"_error.log", text)
		logToFile(filepath.Join(logs, "error.log"), text)
	}
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

			text = strings.TrimSpace(text)
			logjson := make(map[string]interface{})
			err := json.Unmarshal([]byte(text), &logjson)
			if err != nil {
				fmt.Println("Could not decode log message as JSON:", text)
				continue
			}

			app = sanitize.BaseName(app)
			timestamp := logParts["timestamp"].(time.Time)
			hostname := sanitize.BaseName(logParts["hostname"].(string))

			logmsg := logjson["msg"].(string)
			for k, v := range logjson {
				if k == "time" || k == "level" || k == "msg" {
					continue
				}
				logmsg += fmt.Sprintf(" %s=%s", k, v)
			}

			logtext := fmt.Sprintf("%s %s\n", timestamp.Format("Mon Jan 2 2006 15:04:05"), logmsg)
			fmt.Print(logtext)
			logIt(hostname, app, logtext, logParts["severity"].(int))
		}
	}(channel)

	server.Wait()
}
