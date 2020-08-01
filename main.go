package main

import (
	"encoding/json"
	"flag"
	"log"
	"net/http"
	"os"
	"regexp"

	"github.com/gorilla/websocket"
)

// Command is the websocket command string
type Command struct {
	Cmd  int              `json:"cmd"`
	Data FileTransferData `json:"data"`
}

// FileTransferData is specific data for a file transfer
type FileTransferData struct {
	Size      int    `json:"size"`
	ChunkSize int    `json:"chunkSize"`
	Filename  string `json:"filename"`
}

var addr = flag.String("addr", "0.0.0.0:8080", "http service address")

var upgrader = websocket.Upgrader{} // use default options

func upload(writer http.ResponseWriter, reader *http.Request) {
	connection, err := upgrader.Upgrade(writer, reader, nil)
	if err != nil {
		log.Print("upgrade:", err)
		return
	}
	defer connection.Close()
	var command Command // Websocket json command

	fileTransferInProgress := false
	chunksToReceive := 0
	var outputFile *os.File
	filenameFilter := regexp.MustCompile(`[^ a-zA-Z0-9_\.-]`)

	for {
		mt, message, err := connection.ReadMessage()
		if err != nil {
			log.Println("read:", err)
			break
		}
		if (!fileTransferInProgress) && err == nil {
			err = json.Unmarshal(message, &command)
			// Cmd 1 means file transfer start. Switch to file transfer in progress mode
			// Cmd 2 means file transfer is done. Switch to file transfer in progress mode
			if command.Cmd == 1 {
				// Remove illegal characters in filename
				command.Data.Filename = filenameFilter.ReplaceAllString(command.Data.Filename, "")
				fileTransferInProgress = true
				chunksToReceive = command.Data.Size / command.Data.ChunkSize
				outputFile, err = os.Create("./upload/" + command.Data.Filename)
				log.Printf("Receiving file %s, %d bytes", command.Data.Filename, command.Data.Size)
			}
			if command.Cmd == 2 {
				// Remove illegal characters in filename
				command.Data.Filename = filenameFilter.ReplaceAllString(command.Data.Filename, "")
				log.Printf("Finished receiving file %s, %d bytes", command.Data.Filename, command.Data.Size)
			}
		} else if fileTransferInProgress {
			outputFile.Write(message)

			chunksToReceive--
			if chunksToReceive < 0 {
				fileTransferInProgress = false
				outputFile.Close()
			}
		}

		err = connection.WriteMessage(mt, message)
		if err != nil {
			log.Println("write:", err)
			break
		}
	}
}

func main() {
	flag.Parse()
	log.SetFlags(0)
	http.HandleFunc("/upload", upload)
	http.Handle("/", http.FileServer(http.Dir("./")))
	log.Fatal(http.ListenAndServe(*addr, nil))
}
