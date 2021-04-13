package main

import (
	"fmt"
	"io"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"msw/moon"

	"github.com/BurntSushi/toml"
)

func Main(conf interface{},
	checkConfig func() string,
	run func()) {

	timestamp := time.Now()
	var logFile *os.File
	defer logFile.Close()
	defer logShutdown(timestamp)
	defer moon.MainRecover(1)
	confFileName := parseCmdLine()
	LogFileName := loadConfig(confFileName, conf, checkConfig)
	logFile = initLogFile(LogFileName)
	log.Printf("application startup [%s]", moon.SelfAppName())
	log.Printf("conf: %+v", conf)
	go runLogMemUsage()
	go func() {
		run()
	}()
	waitSignal()
}

func logShutdown(timestamp time.Time) {
	elapsedTime := time.Since(timestamp).Truncate(time.Second)
	log.Printf("uptime: %s", elapsedTime.String())
	log.Printf("application shutdown")
}

func parseCmdLine() string {
	if len(os.Args) > 1 && os.Args[1] != "" {
		return os.Args[1]
	}
	return moon.ReplaceFileExt(moon.SelfAppExecutable(), ".conf")
}

func loadConfig(confFile string, conf interface{}, checkConfig func() string) (logFileDir string) {
	fmt.Println("config-file:", confFile)
	_, err := toml.DecodeFile(confFile, conf)
	moon.Check(err, "load conf")
	logFileDir = checkConfig()
	fmt.Printf("config: %+v\n", conf)
	return
}

func initLogFile(logDir string) *os.File {
	subDir := time.Now().Format("2006-01-02")
	logDir = moon.PathJoin(logDir, subDir)
	moon.MakeDir(logDir)
	name := time.Now().Format("15-04-05")
	name += "@" + moon.SelfAppName() + ".log"
	logFileName := moon.PathJoin(logDir, name)
	if moon.PathExist(logFileName) {
		logFileName += "." + fmt.Sprint(os.Getpid())
	}
	logfile, err := moon.CreateFileX(logFileName)
	moon.Check(err, "error opening log file")
	//log.SetOutput(logfile)
	mw := io.MultiWriter(os.Stdout, logfile)
	log.SetOutput(mw)
	log.SetFlags(log.Ldate | log.Ltime | log.Lmicroseconds)
	return logfile
}

func runLogMemUsage() {
	for true {
		LogMemUsage()
		timer := time.NewTimer(time.Minute * 2)
		<-timer.C
	}
}

func waitSignal() {
	c := make(chan os.Signal, 1)
	signal.Notify(c,
		syscall.SIGHUP,
		syscall.SIGINT,
		syscall.SIGTERM,
		syscall.SIGQUIT,
	)
	s := <-c
	log.Printf("got signal: %s (%d)", s.String(), s)
}

func LogProcessFile(description string, name string, timestamp time.Time, fileSize int64, apiUrl string) {
	elapsedTime := time.Since(timestamp).Truncate(time.Second)
	if elapsedTime > time.Millisecond {
		speed := uint64(float64(fileSize) / elapsedTime.Seconds())
		log.Printf("%s: %s [%s] - %s - %s", description, name, moon.FormatByteSize(fileSize), elapsedTime.String(), moon.FormatByteSpeed(speed))
	} else {
		log.Printf("%s: %s [%s] - %s", description, name, moon.FormatByteSize(fileSize), elapsedTime.String())
	}
}

var lastMemUsage string

func LogMemUsage() {
	memUsage := moon.MemUsageStr()
	if memUsage != lastMemUsage {
		log.Printf("mem usage: %s", memUsage)
		lastMemUsage = memUsage
	}
}
