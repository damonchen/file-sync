package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/op/go-logging"
)

// Config  for client or server
type Config struct {
	Server   string `json:"server,omitempty"`
	Port     string `json:"port"`
	SavePath string `json:"savePath,omitempty"` // 服务器上总的路径
}

var (
	logger     = logging.MustGetLogger("file-sync")
	configFile string
	configData Config
	fileName   string // client端指定上传文件名
	filePath   string // client端指定服务器上的名称
)

func initConfig() error {
	data, err := ioutil.ReadFile(configFile)
	if err != nil {
		logger.Errorf("open %s file error %s", configFile, err)
		return err
	}

	err = json.Unmarshal(data, &configData)
	if err != nil {
		logger.Errorf("unmarshal json file error %s", err)
		return err
	}

	return nil
}

func server() {
	http.HandleFunc("/upload", func(w http.ResponseWriter, r *http.Request) {
		logger.Info("new upload coming")

		err := r.ParseMultipartForm(32 << 20)
		if err != nil {
			logger.Errorf("parse multipart form error %s", err)
			return
		}

		file, handler, err := r.FormFile("uploadfile")
		if err != nil {
			logger.Errorf("upload file error %s", err)
			return
		}
		defer file.Close()

		fmt.Fprintf(w, "%v", handler.Header)

		filePath := r.FormValue("filePath")

		ext := filepath.Ext(handler.Filename)
		name := time.Now().Format("2006-01-02") + ext

		saveFile := configData.SavePath + "/" + filePath + "/" + name
		saveFile = filepath.Clean(saveFile)

		logger.Infof("will save file %s", saveFile)
		f, err := os.OpenFile(saveFile, os.O_WRONLY|os.O_CREATE, 0666)
		if err != nil {
			logger.Errorf("upload file when save file error %s", err)
			return
		}

		defer f.Close()
		io.Copy(f, file)
	})

	logger.Critical(http.ListenAndServe(configData.Port, nil))
}

func client() {
	bodyBuff := &bytes.Buffer{}
	bodyWriter := multipart.NewWriter(bodyBuff)

	fileWriter, err := bodyWriter.CreateFormFile("uploadfile", fileName)
	if err != nil {
		logger.Errorf("create upload file error %s", err)
		return
	}

	fh, err := os.Open(fileName)
	if err != nil {
		logger.Errorf("could not open file %s", err)
		return
	}
	defer fh.Close()

	_, err = io.Copy(fileWriter, fh)
	if err != nil {
		logger.Errorf("copy data to file writer error %s", err)
		return
	}

	fieldWriter, err := bodyWriter.CreateFormField("filePath")
	if err != nil {
		logger.Errorf("create field writer error %s", err)
		return
	}

	fieldWriter.Write([]byte(filePath))

	bodyWriter.Close()

	contentType := bodyWriter.FormDataContentType()
	targetURL := "http://" + configData.Server + configData.Port + "/upload"
	resp, err := http.Post(targetURL, contentType, bodyBuff)
	if err != nil {
		logger.Errorf("post data error %s", err)
		return
	}
	defer resp.Body.Close()

	data, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		logger.Errorf("read response data error %s", err)
		return
	}

	logger.Infof("respone status %s", resp.Status)
	logger.Info(string(data))
	return
}

func main() {
	flag.StringVar(&configFile, "configFile", "", "config file")
	flag.StringVar(&fileName, "fileName", "", "upload filename")
	flag.StringVar(&filePath, "filePath", "", "filepath")
	flag.Parse()

	err := initConfig()
	if err != nil {
		return
	}

	if configData.Server == "" {
		server()
	} else {

		client()
	}
}
