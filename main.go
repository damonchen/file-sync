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
	configFile string
	configData Config
	fileName   string // client端指定上传文件名
	filePath   string // client端指定服务器上的名称
)

func SendString(writer io.Writer, v string) error {
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
	logger.Debugf("length %d", length)

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
	_, err = io.Copy(f, conn)
	if err != nil {
		logger.Errorf("copy data error %s", err)
		return
	}

	go func(fileName string) {
		time.Sleep(time.Duration(1) * time.Second)
		calcMd5(fileName)
	}(fileName)

	logger.Infof("complete recv file %s", saveFile)
}

func server() {
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

	//	http.HandleFunc("/upload", func(w http.ResponseWriter, r *http.Request) {
	//		logger.Info("new upload coming")

	//		err := r.ParseMultipartForm(32 << 20)
	//		if err != nil {
	//			logger.Errorf("parse multipart form error %s", err)
	//			return
	//		}

	//		file, handler, err := r.FormFile("uploadfile")
	//		if err != nil {
	//			logger.Errorf("upload file error %s", err)
	//			return
	//		}
	//		defer file.Close()

	//		fmt.Fprintf(w, "%v", handler.Header)

	//		filePath := r.FormValue("filePath")

	//		ext := filepath.Ext(handler.Filename)
	//		name := time.Now().Format("2006-01-02") + ext

	//		saveFile := configData.SavePath + "/" + filePath + "/" + name
	//		saveFile = filepath.Clean(saveFile)

	//		logger.Infof("will save file %s", saveFile)
	//		f, err := os.OpenFile(saveFile, os.O_WRONLY|os.O_CREATE, 0666)
	//		if err != nil {
	//			logger.Errorf("upload file when save file error %s", err)
	//			return
	//		}

	//		defer f.Close()
	//		io.Copy(f, file)
	//	})

	//	logger.Infof("保存文件路径:%s", configData.SavePath)

	//	logger.Critical(http.ListenAndServe(configData.Port, nil))
}

func client() {

	address := configData.Server + configData.Port
	conn, err := net.Dial("tcp", address)
	if err != nil {
		logger.Errorf("client connection error %s", err)
		return
	}
	defer conn.Close()

	file := File{conn: conn}
	file.FileName = fileName
	file.FilePath = filePath
	err = file.send()
	if err != nil {
		logger.Errorf("file send error %s", err)
		return
	}
	return

	//	bodyBuff := &bytes.Buffer{}
	//	bodyWriter := multipart.NewWriter(bodyBuff)

	//	fileWriter, err := bodyWriter.CreateFormFile("uploadfile", fileName)
	//	if err != nil {
	//		logger.Errorf("create upload file error %s", err)
	//		return
	//	}

	//	fh, err := os.Open(fileName)
	//	if err != nil {
	//		logger.Errorf("could not open file %s", err)
	//		return
	//	}
	//	defer fh.Close()

	//	_, err = io.Copy(fileWriter, fh)
	//	if err != nil {
	//		logger.Errorf("copy data to file writer error %s", err)
	//		return
	//	}

	//	fieldWriter, err := bodyWriter.CreateFormField("filePath")
	//	if err != nil {
	//		logger.Errorf("create field writer error %s", err)
	//		return
	//	}

	//	fieldWriter.Write([]byte(filePath))

	//	bodyWriter.Close()

	//	contentType := bodyWriter.FormDataContentType()
	//	targetURL := "http://" + configData.Server + configData.Port + "/upload"
	//	resp, err := http.Post(targetURL, contentType, bodyBuff)
	//	if err != nil {
	//		logger.Errorf("post data error %s", err)
	//		return
	//	}
	//	defer resp.Body.Close()

	//	data, err := ioutil.ReadAll(resp.Body)
	//	if err != nil {
	//		logger.Errorf("read response data error %s", err)
	//		return
	//	}

	//	logger.Infof("respone status %s", resp.Status)
	//	logger.Info(string(data))
	//	return
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
