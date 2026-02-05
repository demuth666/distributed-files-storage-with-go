package server

import (
	"encoding/json"
	"io"
	"net/http"
)

type API struct {
	server   *FileServer
	httpAddr string
}

func NewAPI(server *FileServer, httpAddr string) *API {
	return &API{
		server:   server,
		httpAddr: httpAddr,
	}
}

func (a *API) Start() error {
	mux := http.NewServeMux()

	mux.HandleFunc("POST /store", a.handleStore)
	mux.HandleFunc("GET /get/{key}", a.handleGet)
	mux.HandleFunc("DELETE /delete/{key}", a.handleDelete)
	mux.HandleFunc("GET /peers", a.handlePeers)
	mux.HandleFunc("GET /status", a.handleStatus)

	return http.ListenAndServe(a.httpAddr, mux)
}

func (a *API) handleStore(w http.ResponseWriter, r *http.Request) {
	file, _, err := r.FormFile("file")
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	defer file.Close()

	key, err := a.server.StoreFile("", file)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	json.NewEncoder(w).Encode(map[string]string{"key": key})
}

func (a *API) handleGet(w http.ResponseWriter, r *http.Request) {
	key := r.PathValue("key")

	reader, err := a.server.GetFile(key)
	if err != nil {
		http.Error(w, "file not found", http.StatusNotFound)
		return
	}
	defer reader.Close()

	io.Copy(w, reader)
}

func (a *API) handlePeers(w http.ResponseWriter, r *http.Request) {
	peers := a.server.ListPeers()

	addrs := make([]string, len(peers))
	for i, p := range peers {
		addrs[i] = p.String()
	}

	json.NewEncoder(w).Encode(map[string][]string{"peers": addrs})
}

func (a *API) handleDelete(w http.ResponseWriter, r *http.Request) {
	key := r.PathValue("key")

	if err := a.server.DeleteFile(key); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

func (a *API) handleStatus(w http.ResponseWriter, r *http.Request) {
	json.NewEncoder(w).Encode(map[string]any{
		"listen_addr":  a.server.opts.ListenAddr,
		"storage_root": a.server.opts.StorageRoot,
		"replication":  a.server.opts.ReplicationFactor,
		"peer_count":   len(a.server.ListPeers()),
	})
}
