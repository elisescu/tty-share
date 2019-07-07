package main

import (
	"errors"
	"html/template"
	"mime"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"sync"

	"github.com/gorilla/mux"
	"github.com/gorilla/websocket"
)

const (
	errorInvalidSession = iota
	errorNotFound       = iota
	errorNotAllowed     = iota
)

var log = MainLogger

// SessionTemplateModel used for templating
type SessionTemplateModel struct {
	SessionID string
	Salt      string
	WSPath    string
}

// TTYServerConfig is used to configure the tty server before it is started
type TTYServerConfig struct {
	WebAddress       string
	TTYSenderAddress string
	ServerURL        string
	// The TLS Cert and Key can be null, if TLS should not be used
	TLSCertFile  string
	TLSKeyFile   string
	FrontendPath string
}

// TTYServer represents the instance of a tty server
type TTYServer struct {
	httpServer           *http.Server
	ttySendersListener   net.Listener
	config               TTYServerConfig
	activeSessions       map[string]*ttyShareSession
	activeSessionsRWLock sync.RWMutex
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
		Addr: config.WebAddress,
	}
	routesHandler := mux.NewRouter()

	routesHandler.PathPrefix("/static/").Handler(http.StripPrefix("/static/",
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			server.serveContent(w, r, r.URL.Path)
		})))

	routesHandler.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "https://github.com/elisescu/tty-share", http.StatusMovedPermanently)
	})
	routesHandler.HandleFunc("/s/{sessionID}", func(w http.ResponseWriter, r *http.Request) {
		server.handleSession(w, r)
	})
	routesHandler.HandleFunc("/ws/{sessionID}", func(w http.ResponseWriter, r *http.Request) {
		server.handleWebsocket(w, r)
	})
	routesHandler.NotFoundHandler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		server.serveContent(w, r, "404.html")
	})

	server.activeSessions = make(map[string]*ttyShareSession)
	server.httpServer.Handler = routesHandler
	return server
}

func getWSPath(sessionID string) string {
	return "/ws/" + sessionID
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

	session := getSession(server, sessionID)

	if session == nil {
		log.Error("WE connection for invalid sessionID: ", sessionID, ". Killing it.")
		w.WriteHeader(http.StatusForbidden)
		return
	}

	session.HandleReceiver(newWSConnection(conn))
}

func (server *TTYServer) handleSession(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	sessionID := vars["sessionID"]

	log.Debug("Handling web TTYReceiver session: ", sessionID)

	session := getSession(server, sessionID)

	// No valid session with this ID
	if session == nil {
		server.serveContent(w, r, "invalid-session.html")
		return
	}

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

func addNewSession(server *TTYServer, session *ttyShareSession) {
	server.activeSessionsRWLock.Lock()
	server.activeSessions[session.GetID()] = session
	server.activeSessionsRWLock.Unlock()
}

func removeSession(server *TTYServer, session *ttyShareSession) {
	server.activeSessionsRWLock.Lock()
	delete(server.activeSessions, session.GetID())
	server.activeSessionsRWLock.Unlock()
}

func getSession(server *TTYServer, sessionID string) (session *ttyShareSession) {
	// TODO: move this in a better place
	server.activeSessionsRWLock.RLock()
	session = server.activeSessions[sessionID]
	server.activeSessionsRWLock.RUnlock()
	return
}

func handleTTYSenderConnection(server *TTYServer, conn net.Conn) {
	defer conn.Close()

	session := newTTYShareSession(conn, server.config.ServerURL)

	if err := session.InitSender(); err != nil {
		log.Warnf("Cannot create session with %s. Error: %s", conn.RemoteAddr().String(), err.Error())
		return
	}

	addNewSession(server, session)

	session.HandleSenderConnection()

	removeSession(server, session)
	log.Debug("Finished session ", session.GetID(), ". Removing it.")
}

// Listen starts listening on connections
func (server *TTYServer) Listen() (err error) {
	var wg sync.WaitGroup
	runTLS := server.config.TLSCertFile != "" && server.config.TLSKeyFile != ""

	// Start listening on the frontend side
	wg.Add(1)
	go func() {
		if !runTLS {
			err = server.httpServer.ListenAndServe()
		} else {
			err = server.httpServer.ListenAndServeTLS(server.config.TLSCertFile, server.config.TLSKeyFile)
		}
		// Just in case we are existing because of an error, close the other listener too
		if server.ttySendersListener != nil {
			server.ttySendersListener.Close()
		}
		wg.Done()
	}()

	// TODO: Add support for listening for connections over TLS
	// Listen on connections on the tty sender side
	server.ttySendersListener, err = net.Listen("tcp", server.config.TTYSenderAddress)
	if err != nil {
		log.Error("Cannot create the front server. Error: ", err.Error())
		return
	}

	for {
		connection, err := server.ttySendersListener.Accept()
		if err == nil {
			go handleTTYSenderConnection(server, connection)
		} else {
			break
		}
	}
	// Close the http side too
	if server.httpServer != nil {
		server.httpServer.Close()
	}

	wg.Wait()
	log.Debug("Server finished")
	return
}

// Stop closes down the server
func (server *TTYServer) Stop() error {
	log.Debug("Stopping the server")
	err1 := server.httpServer.Close()
	err2 := server.ttySendersListener.Close()
	if err1 != nil || err2 != nil {
		//TODO: do this nicer
		return errors.New(err1.Error() + err2.Error())
	}
	return nil
}
