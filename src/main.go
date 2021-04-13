package main

import (
	"log"
	"msw/moon"
	"msw/yandex/yandisk"
	"path"
	"time"
)

type Config struct {
	LogDir          string
	AppID           string
	Token           string
	LocalDir        string
	RemoteDir       string
	LimitDriveUsage int
	Imitation       bool
}

var conf Config

func main() {
	Main(&conf, func() string {
		conf.LogDir = moon.DecodePath(conf.LogDir)
		conf.LocalDir = moon.DecodePath(conf.LocalDir)
		return conf.LogDir
	}, run)
}

func run() {
	if conf.Token == "" {
		log.Printf("Go to the following link in your browser for get token:\n%s\nput retrieved Token to config and restart app\n",
			yandisk.TokenUrl(conf.AppID))
		return
	}
	client := yandisk.NewClient(conf.Token)
	waitCheckDrive(client)
	client.CreateFolderIfNotExist(conf.RemoteDir)
	for {
		name, filename := waitFile()
		waitCheckDrive(client)
		uploadFile(client, name, filename)
	}
	log.Print("error: upload files completed")
}

func nextFileForUpload() (string, string) {
	files := moon.ReadDir(conf.LocalDir, moon.ReadDirCtx{Files: true, RelativePath: true})
	if len(files) == 0 {
		return "", ""
	}
	name := files[0]
	return name, moon.PathJoin(conf.LocalDir, name)
}

func waitFile() (string, string) {
	name, filename := nextFileForUpload()
	if filename == "" {
		timestamp := time.Now()
		log.Print("wait file...")
		for {
			time.Sleep(10 * time.Second)
			name, filename = nextFileForUpload()
			if filename != "" {
				log.Printf("waited: %v", time.Since(timestamp).Truncate(time.Second).String())
				break
			}
		}
	}
	return name, filename
}

func uploadFile(client *yandisk.Client, name string, filename string) {
	fileSize := moon.FileSize(filename)
	log.Printf("upload: %s [%s]", name, moon.FormatByteSize(fileSize))
	defer func() {
		if r := recover(); r != nil {
			log.Print(r)
			log.Print("upload file fail:", filename)
			time.Sleep(10 * time.Second)
		}
	}()
	if conf.Imitation {
		log.Print("simulate upload...")
		time.Sleep(5 * time.Second)
	} else {
		if client != nil {
			client.UploadFileMust(filename, path.Join(conf.RemoteDir, name))
			log.Printf("upload completed")
		}
	}
	moon.PathRemove(filename)
}

func checkDrive(client *yandisk.Client) bool {
	log.Print("check drive...")
	disk := client.GetDiskMust()
	percentageUsage := moon.PercentageRelation(disk.UsedSpace, disk.TotalSpace)
	log.Printf("drive usage: %s / %s [%d %%]\n", moon.FormatByteSize(disk.UsedSpace), moon.FormatByteSize(disk.TotalSpace), percentageUsage)
	if percentageUsage > conf.LimitDriveUsage {
		log.Printf("drive usage limit exceeded: %d %% (limit: %d %%). Please free up disk space...", percentageUsage, conf.LimitDriveUsage)
		time.Sleep(30 * time.Second)
		return false
	}
	return true
}

func waitCheckDrive(client *yandisk.Client) {
	for !checkDrive(client) {
	}
}
