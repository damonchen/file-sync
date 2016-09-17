package main

import (
	"crypto/md5"
	"encoding/binary"
	"encoding/json"
	"flag"
	"io"
	"io/ioutil"
	"net"
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
	verbose    bool
	configFile string
	configData Config
	fileName   string // client端指定上传文件名
	filePath   string // client端指定服务器上的名称
)

func verboseInfo(format string, msg ...interface{}) {
	if verbose {
		logger.Debugf(format, msg...)
	}
}

func SendString(writer io.Writer, v string) error {
	verboseInfo("will send string %s", v)
	var length int64 = int64(len(v))
	err := binary.Write(writer, binary.BigEndian, &length)
	if err != nil {
		return err
	}

	_, err = writer.Write([]byte(v))
	return err
}

func RecvString(reader io.Reader) (string, error) {
	var length int64

	err := binary.Read(reader, binary.BigEndian, &length)
	if err != nil {
		return "", err
	}

	verboseInfo("recv remote string length %d", length)

	data := make([]byte, length)
	_, err = reader.Read(data)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// 文件结构
type File struct {
	conn     net.Conn // 连接
	FileName string   // 文件名
	FilePath string   // 文件路径
}

// md5码
func calcMd5(fileName string) error {
	fh, err := os.Open(fileName)
	if err != nil {
		logger.Errorf("could not open file %s", err)
		return err
	}
	defer fh.Close()

	h := md5.New()
	io.Copy(h, fh)

	logger.Infof("file %s md5 %x", fileName, h.Sum(nil))
	return nil
}

func (f *File) send() error {
	err := SendString(f.conn, f.FileName)
	if err != nil {
		return err
	}

	err = SendString(f.conn, f.FilePath)
	if err != nil {
		return err
	}

	fh, err := os.Open(fileName)
	if err != nil {
		logger.Errorf("could not open file %s", err)
		return err
	}
	defer fh.Close()

	_, err = io.Copy(f.conn, fh)
	return err
}

func initConfig() error {

	verboseInfo("start read config file %s", configFile)
	data, err := ioutil.ReadFile(configFile)
	if err != nil {
		logger.Errorf("open %s file error %s", configFile, err)
		return err
	}

	verboseInfo("unmarshal data to config json")
	err = json.Unmarshal(data, &configData)
	if err != nil {
		logger.Errorf("unmarshal json file error %s", err)
		return err
	}

	return nil
}

func newClient(conn net.Conn) {
	defer conn.Close()

	fileName, err := RecvString(conn)
	if err != nil {
		logger.Errorf("get file name %s", err)
		return
	}
	logger.Infof("get file name %s", fileName)

	filePath, err := RecvString(conn)
	if err != nil {
		logger.Errorf("get file path %s", err)
		return
	}
	logger.Infof("get file path %s", filePath)

	ext := filepath.Ext(fileName)
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

	verboseInfo("will copy file from network to %s", saveFile)
	_, err = io.Copy(f, conn)
	if err != nil {
		logger.Errorf("copy data error %s", err)
		return
	}

	go func(fileName string) {
		verboseInfo("will calc file %s md5 after 1 second later", fileName)
		time.Sleep(time.Duration(1) * time.Second)
		calcMd5(fileName)
	}(fileName)

	logger.Infof("complete recv file %s", saveFile)
}

func server() {
	logger.Infof("will listen %s", configData.Port)
	ln, err := net.Listen("tcp", configData.Port)
	if err != nil {
		logger.Errorf("server net dial error %s", err)
		return
	}

	for {
		conn, err := ln.Accept()
		if err != nil {
			logger.Errorf("connection error %s", err)
			return
		}

		go newClient(conn)
	}

}

func client() {

	address := configData.Server + configData.Port
	verboseInfo("connect to remote address %s", address)
	conn, err := net.Dial("tcp", address)
	if err != nil {
		logger.Errorf("client connection error %s", err)
		return
	}
	defer conn.Close()

	file := File{conn: conn}
	file.FileName = fileName
	file.FilePath = filePath

	verboseInfo("client fileName %s, filePath %s", fileName, filePath)

	err = file.send()
	if err != nil {
		logger.Errorf("file send error %s", err)
		return
	}
	logger.Info("file send complete")
	return
}

func main() {
	flag.StringVar(&configFile, "configFile", "", "config file")
	flag.StringVar(&fileName, "fileName", "", "upload filename")
	flag.StringVar(&filePath, "filePath", "", "filepath")
	flag.BoolVar(&verbose, "v", false, "verbose")
	flag.Parse()

	verboseInfo("start parse config file %s", configFile)
	err := initConfig()
	if err != nil {
		return
	}

	if configData.Server == "" {
		verboseInfo("running at server model")
		server()
	} else {
		verboseInfo("running at client model")
		client()
	}
}
