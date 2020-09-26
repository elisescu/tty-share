package server

import (
	"html/template"
	"io"
	"mime"

	"net/http"
	"os"
	"path/filepath"

	"github.com/gorilla/mux"
	"github.com/gorilla/websocket"
	log "github.com/sirupsen/logrus"
)

const (
	errorNotFound   = iota
	errorNotAllowed = iota
)

type NewClientConnectedCB func(string)

// SessionTemplateModel used for templating
type SessionTemplateModel struct {
	SessionID string
	Salt      string
	WSPath    string
}

// TTYServerConfig is used to configure the tty server before it is started
type TTYServerConfig struct {
	FrontListenAddress   string
	FrontendPath string
	TTYWriter io.Writer
}

// TTYServer represents the instance of a tty server
type TTYServer struct {
	httpServer *http.Server
	newClientCB NewClientConnectedCB
	config     TTYServerConfig
	session    *ttyShareSession
}

func (server *TTYServer) serveContent(w http.ResponseWriter, r *http.Request, name string) {
	// If a path to the frontend resources was passed, serve from there, otherwise, serve from the
	// builtin bundle
	if server.config.FrontendPath == "" {
		file, err := Asset(name)

		if err != nil {
			w.WriteHeader(http.StatusNotFound)
			return
		}

		ctype := mime.TypeByExtension(filepath.Ext(name))
		if ctype == "" {
			ctype = http.DetectContentType(file)
		}
		w.Header().Set("Content-Type", ctype)
		w.Write(file)
	} else {
		filePath := server.config.FrontendPath + string(os.PathSeparator) + name
		_, err := os.Open(filePath)

		if err != nil {
			log.Errorf("Couldn't find resource: %s at %s", name, filePath)
			w.WriteHeader(http.StatusNotFound)
			return
		}
		log.Debugf("Serving %s from %s", name, filePath)

		http.ServeFile(w, r, filePath)
	}
}

// NewTTYServer creates a new instance
func NewTTYServer(config TTYServerConfig) (server *TTYServer) {
	server = &TTYServer{
		config: config,
	}
	server.httpServer = &http.Server{
		Addr: config.FrontListenAddress,
	}
	routesHandler := mux.NewRouter()

	routesHandler.PathPrefix("/static/").Handler(http.StripPrefix("/static/",
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			server.serveContent(w, r, r.URL.Path)
		})))

	routesHandler.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		server.handleMainApp(w, r)
	})
	routesHandler.HandleFunc("/ws", func(w http.ResponseWriter, r *http.Request) {
		server.handleWebsocket(w, r)
	})
	routesHandler.NotFoundHandler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		server.serveContent(w, r, "404.html")
	})

	server.httpServer.Handler = routesHandler
	server.session = newTTYShareSession(config.TTYWriter)

	return server
}

func getWSPath(sessionID string) string {
	return "/ws" + sessionID
}

func (server *TTYServer) handleWebsocket(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	sessionID := vars["sessionID"]
	defer log.Debug("Finished WS connection for ", sessionID)

	// Validate incoming request.
	if r.Method != "GET" {
		w.WriteHeader(http.StatusForbidden)
		return
	}

	// Upgrade to Websocket mode.
	upgrader := websocket.Upgrader{
		ReadBufferSize:  1024,
		WriteBufferSize: 1024,
	}
	conn, err := upgrader.Upgrade(w, r, nil)

	if err != nil {
		log.Error("Cannot create the WS connection for session ", sessionID, ". Error: ", err.Error())
		return
	}

	server.newClientCB(conn.RemoteAddr().String())
	server.session.HandleReceiver(newWSConnection(conn))
}

func (server *TTYServer) handleMainApp(w http.ResponseWriter, r *http.Request) {
	var t *template.Template
	var err error
	if server.config.FrontendPath == "" {
		templateDta, err := Asset("tty-receiver.in.html")

		if err != nil {
			panic("Cannot find the tty-receiver html template")
		}

		t = template.New("tty-receiver.html")
		_, err = t.Parse(string(templateDta))
	} else {
		t, err = template.ParseFiles(server.config.FrontendPath + string(os.PathSeparator) + "tty-receiver.in.html")
	}

	if err != nil {
		panic("Cannot parse the tty-receiver html template")
	}

	sessionID := ""

	templateModel := SessionTemplateModel{
		SessionID: sessionID,
		Salt:      "salt&pepper",
		WSPath:    getWSPath(sessionID),
	}
	err = t.Execute(w, templateModel)

	if err != nil {
		panic("Cannot execute the tty-receiver html template")
	}
}

// Run starts listening on connections
func (server *TTYServer) Run(cb NewClientConnectedCB) (err error) {
	server.newClientCB = cb
	err = server.httpServer.ListenAndServe()
	log.Debug("Server finished")
	return
}

func (server *TTYServer) Write(buff []byte) (written int, err error) {
	return server.session.Write(buff)
}

func (server *TTYServer) WindowSize(cols, rows int) (err error) {
	return server.session.WindowSize(cols, rows)
}

// Stop closes down the server
func (server *TTYServer) Stop() error {
	log.Debug("Stopping the server")
	return server.httpServer.Close()
}
