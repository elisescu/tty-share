package main

import (
	"errors"
	"fmt"
	"html/template"
	"net"
	"net/http"
	"sync"

	"github.com/gorilla/mux"
	"github.com/gorilla/websocket"
)

var log = MainLogger

// SessionTemplateModel used for templating
type SessionTemplateModel struct {
	SessionID string
	Salt      string
	WSPath    string
}

// TTYProxyServerConfig is used to configure the proxy server before it is started
type TTYProxyServerConfig struct {
	WebAddress       string
	TTYSenderAddress string
	ServerURL        string
	// The TLS Cert and Key can be null, if TLS should not be used
	TLSCertFile  string
	TLSKeyFile   string
	FrontendPath string
}

// TTYProxyServer represents the instance of a proxy server
type TTYProxyServer struct {
	httpServer           *http.Server
	ttySendersListener   net.Listener
	config               TTYProxyServerConfig
	activeSessions       map[string]*ttyShareSession
	activeSessionsRWLock sync.RWMutex
}

// NewTTYProxyServer creates a new instance
func NewTTYProxyServer(config TTYProxyServerConfig) (server *TTYProxyServer) {
	server = &TTYProxyServer{
		config: config,
	}
	server.httpServer = &http.Server{
		Addr: config.WebAddress,
	}
	routesHandler := mux.NewRouter()

	if config.FrontendPath != "" {
		routesHandler.PathPrefix("/static/").Handler(http.StripPrefix("/static/",
			http.FileServer(http.Dir(config.FrontendPath))))
	} else {
		// Serve the bundled assets
		routesHandler.PathPrefix("/static/").Handler(http.StripPrefix("/static/",
			http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				data, err := Asset(r.URL.Path)
				if err != nil {
					w.WriteHeader(http.StatusNotFound)
					return
				}
				w.Write(data)
				log.Infof("Delivered %s from the bundle", r.URL.Path)
			})))
	}

	routesHandler.HandleFunc("/", defaultHandler)
	routesHandler.HandleFunc("/s/{sessionID}", func(w http.ResponseWriter, r *http.Request) {
		sessionsHandler(server, w, r)
	})
	routesHandler.HandleFunc("/ws/{sessionID}", func(w http.ResponseWriter, r *http.Request) {
		websocketHandler(server, w, r)
	})

	server.activeSessions = make(map[string]*ttyShareSession)
	server.httpServer.Handler = routesHandler
	return server
}

func getWSPath(sessionID string) string {
	return "/ws/" + sessionID
}

func websocketHandler(server *TTYProxyServer, w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	sessionID := vars["sessionID"]
	defer log.Debug("Finished WS connection for ", sessionID)

	// Validate incoming request.
	if r.Method != "GET" {
		w.WriteHeader(http.StatusMethodNotAllowed)
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
		// TODO: Create a proper way to communicate with the remote WS end, so that the server can send
		// control messages or data messages to go directly to the terminal.
		conn.WriteMessage(websocket.TextMessage, []byte("$ access denied."))
		return
	}

	session.HandleReceiver(newWSConnection(conn))
}

func defaultHandler(http.ResponseWriter, *http.Request) {
	log.Debug("Default handler ")
}

func sessionsHandler(server *TTYProxyServer, w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	sessionID := vars["sessionID"]

	log.Debug("Handling web TTYReceiver session: ", sessionID)

	session := getSession(server, sessionID)

	if session == nil {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	templateDta, err := Asset("templates/index.html")

	if err != nil {
		fmt.Fprintf(w, err.Error())
		return
	}

	t := template.New("index.html")
	_, err = t.Parse(string(templateDta))

	if err != nil {
		fmt.Fprintf(w, err.Error())
		return
	}
	templateModel := SessionTemplateModel{
		SessionID: sessionID,
		Salt:      "salt&pepper",
		WSPath:    getWSPath(sessionID),
	}
	t.Execute(w, templateModel)
}

func addNewSession(server *TTYProxyServer, session *ttyShareSession) {
	server.activeSessionsRWLock.Lock()
	server.activeSessions[session.GetID()] = session
	server.activeSessionsRWLock.Unlock()
}

func removeSession(server *TTYProxyServer, session *ttyShareSession) {
	server.activeSessionsRWLock.Lock()
	delete(server.activeSessions, session.GetID())
	server.activeSessionsRWLock.Unlock()
}

func getSession(server *TTYProxyServer, sessionID string) (session *ttyShareSession) {
	// TODO: move this in a better place
	server.activeSessionsRWLock.RLock()
	session = server.activeSessions[sessionID]
	server.activeSessionsRWLock.RUnlock()
	return
}

func handleTTYSenderConnection(server *TTYProxyServer, conn net.Conn) {
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
func (server *TTYProxyServer) Listen() (err error) {
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
func (server *TTYProxyServer) Stop() error {
	log.Debug("Stopping the server")
	err1 := server.httpServer.Close()
	err2 := server.ttySendersListener.Close()
	if err1 != nil || err2 != nil {
		//TODO: do this nicer
		return errors.New(err1.Error() + err2.Error())
	}
	return nil
}
