/*
Copyright © 2026 NAME HERE <EMAIL ADDRESS>
*/
package cmd

import (
	"embed"
	"encoding/json"
	"flag"
	"html/template"
	"io"
	"io/fs"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/gorilla/websocket"
	_ "github.com/lib/pq"
	"github.com/spf13/cobra"
)

//go:embed streamtext/*
var streamtextEmbed embed.FS

var streamTextCmd = &cobra.Command{
	Use:   "stream",
	Short: "stream text to your folder/file",
	Long: `A longer description that spans multiple lines and likely contains examples
and usage of using your command. `,
	Run: func(cmd *cobra.Command, args []string) {
		Logger.Debug("streamTextCmd called")
		webSocketServer()
	},
}

var filePath string

func init() {
	RootCmd.AddCommand(streamTextCmd)
	streamTextCmd.PersistentFlags().StringVarP(&filePath, "serve", "s", "", "serve the websocket")

	b, err := streamtextEmbed.ReadFile("streamtext/home.html")
	if err != nil {
		log.Fatal("streamtext template:", err)
	}
	homeTempl = template.Must(template.New("home").Parse(string(b)))
}

const (
	writeWait  = 10 * time.Second
	pongWait   = 60 * time.Second
	pingPeriod = (pongWait * 9) / 10
	pollPeriod = 500 * time.Millisecond
)

var (
	addr      = flag.String("addr", ":8080", "http service address")
	homeTempl *template.Template
	filename  string
	upgrader  = websocket.Upgrader{
		ReadBufferSize:  1024,
		WriteBufferSize: 1024,
	}
)

func readFileIfModified(lastMod time.Time) ([]byte, time.Time, error) {
	fi, err := os.Stat(filename)
	if err != nil {
		return nil, lastMod, err
	}
	if !fi.ModTime().After(lastMod) {
		return nil, lastMod, nil
	}
	p, err := os.ReadFile(filename)
	if err != nil {
		return nil, fi.ModTime(), err
	}
	return p, fi.ModTime(), nil
}

func reader(ws *websocket.Conn) {
	defer ws.Close()
	ws.SetReadLimit(512)
	ws.SetReadDeadline(time.Now().Add(pongWait))
	ws.SetPongHandler(func(string) error { ws.SetReadDeadline(time.Now().Add(pongWait)); return nil })
	for {
		_, _, err := ws.ReadMessage()
		if err != nil {
			break
		}
	}
}

func writerPollFallback(ws *websocket.Conn, lastMod *time.Time, pingTicker *time.Ticker) {
	fileTicker := time.NewTicker(pollPeriod)
	defer fileTicker.Stop()
	lastError := ""
	for {
		select {
		case <-fileTicker.C:
			p, newMod, err := readFileIfModified(*lastMod)
			if err != nil {
				if s := err.Error(); s != lastError {
					lastError = s
					ws.SetWriteDeadline(time.Now().Add(writeWait))
					if err := ws.WriteMessage(websocket.TextMessage, []byte(s)); err != nil {
						return
					}
				}
				continue
			}
			lastError = ""
			if p != nil {
				*lastMod = newMod
				ws.SetWriteDeadline(time.Now().Add(writeWait))
				if err := ws.WriteMessage(websocket.TextMessage, p); err != nil {
					return
				}
			}
		case <-pingTicker.C:
			ws.SetWriteDeadline(time.Now().Add(writeWait))
			if err := ws.WriteMessage(websocket.PingMessage, []byte{}); err != nil {
				return
			}
		}
	}
}

func writer(ws *websocket.Conn, lastMod time.Time) {
	pingTicker := time.NewTicker(pingPeriod)
	defer pingTicker.Stop()
	defer ws.Close()

	lastModLocal := lastMod
	lastError := ""

	push := func() bool {
		p, newMod, err := readFileIfModified(lastModLocal)
		if err != nil {
			if s := err.Error(); s != lastError {
				lastError = s
				ws.SetWriteDeadline(time.Now().Add(writeWait))
				if err := ws.WriteMessage(websocket.TextMessage, []byte(s)); err != nil {
					return false
				}
			}
			return true
		}
		lastError = ""
		if p == nil {
			return true
		}
		lastModLocal = newMod
		ws.SetWriteDeadline(time.Now().Add(writeWait))
		if err := ws.WriteMessage(websocket.TextMessage, p); err != nil {
			return false
		}
		return true
	}

	if !push() {
		return
	}

	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		log.Printf("stream: fsnotify unavailable (%v); using %v polling fallback", err, pollPeriod)
		writerPollFallback(ws, &lastModLocal, pingTicker)
		return
	}
	defer watcher.Close()
	if err := watcher.Add(filename); err != nil {
		log.Printf("stream: cannot watch file (%v); using %v polling fallback", err, pollPeriod)
		writerPollFallback(ws, &lastModLocal, pingTicker)
		return
	}

	for {
		select {
		case event, ok := <-watcher.Events:
			if !ok {
				return
			}
			if event.Op&(fsnotify.Write|fsnotify.Create|fsnotify.Rename) == 0 {
				continue
			}
			if !push() {
				return
			}
		case err, ok := <-watcher.Errors:
			if !ok {
				return
			}
			log.Println("stream: watcher error:", err)
		case <-pingTicker.C:
			ws.SetWriteDeadline(time.Now().Add(writeWait))
			if err := ws.WriteMessage(websocket.PingMessage, []byte{}); err != nil {
				return
			}
		}
	}
}

func serveWs(w http.ResponseWriter, r *http.Request) {
	ws, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		if _, ok := err.(websocket.HandshakeError); !ok {
			log.Println(err)
		}
		return
	}

	go writer(ws, time.Time{})
	reader(ws)
}

func serveHome(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.Error(w, "Not found", http.StatusNotFound)
		return
	}
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	p, _, err := readFileIfModified(time.Time{})
	if err != nil {
		p = []byte(err.Error())
	}
	dataJSON, jerr := json.Marshal(string(p))
	if jerr != nil {
		http.Error(w, "encode error", http.StatusInternalServerError)
		return
	}
	v := struct {
		Data template.JS
	}{
		Data: template.JS(dataJSON),
	}
	if err := homeTempl.Execute(w, &v); err != nil {
		log.Println("template:", err)
	}
}

func serveContent(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	p, err := os.ReadFile(filename)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	if _, err := w.Write(p); err != nil {
		log.Println("stream: write /content:", err)
	}
}

func saveStreamedFile(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "read error", http.StatusInternalServerError)
		return
	}
	if err := os.WriteFile(filename, body, 0644); err != nil {
		http.Error(w, "write error", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusOK)
}

func webSocketServer() {
	filename = filePath
	assetsFS, err := fs.Sub(streamtextEmbed, "streamtext")
	if err != nil {
		log.Fatal("streamtext assets:", err)
	}
	http.Handle("/assets/", http.StripPrefix("/assets/", http.FileServer(http.FS(assetsFS))))
	http.HandleFunc("/", serveHome)
	http.HandleFunc("/ws", serveWs)
	http.HandleFunc("/content", serveContent)
	http.HandleFunc("/save", saveStreamedFile)
	if err := http.ListenAndServe(*addr, nil); err != nil {
		log.Fatal(err)
	}
}
