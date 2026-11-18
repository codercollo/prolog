package server

import (
	"encoding/json"
	"net/http"

	"github.com/gorilla/mux"
)

// httpServer wraps the log for HTTP handlers.
type httpServer struct {
	Log *Log
}

// ProduceRequest holds the record to append.
type ProduceRequest struct {
	Record Record `json:"record"`
}

// ProduceResponse returns the appended record's offset.
type ProduceResponse struct {
	Offset uint64 `json:"offset"`
}

// ConsumeResponse specifies the offset to record.
type ConsumeRequest struct {
	Offset uint64 `json:"offset"`
}

// ConsumeResponse returns the requestes record.
type ConsumeResponse struct {
	Record Record `json:"record"`
}

// NewHTTPServer configures routes and returns a new HTTP server
func NewHTTPServer(addr string) *http.Server {
	httpsrv := newHTTPServer()
	r := mux.NewRouter()

	r.HandleFunc("/", httpsrv.handleProduce).Methods("POST")
	r.HandleFunc("/", httpsrv.handleConsume).Methods("GET")
	return &http.Server{
		Addr:    addr,
		Handler: r,
	}
}

// newHTTPServer initializes a httpServer with a new log.
func newHTTPServer() *httpServer {
	return &httpServer{
		Log: NewLog(),
	}
}

// handleProduce implements unmarshalling the request to a struct,
// using that struct to produce to the log and getting the offset that
// the log stored the record under, and unmarshalling and writing the
// result to the response.
func (s *httpServer) handleProduce(w http.ResponseWriter, r *http.Request) {
	var req ProduceRequest
	err := json.NewDecoder(r.Body).Decode(&req)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	off, err := s.Log.Append(req.Record)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	res := ProduceResponse{Offset: off}
	err = json.NewEncoder(w).Encode(res)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

// handleConsume reads a record from the log by offset and writes it to the response.
func (s *httpServer) handleConsume(w http.ResponseWriter, r *http.Request) {
	var req ConsumeRequest
	err := json.NewDecoder(r.Body).Decode(&req)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	record, err := s.Log.Read(req.Offset)
	if err == ErrOffsetNotFound {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	res := ConsumeResponse{Record: record}
	err = json.NewEncoder(w).Encode(res)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}
