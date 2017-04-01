package main

import (
	"encoding/json"
	"flag"
	"io/ioutil"
	"log"
	"net/http"
	"strings"

	"moonchan/models"
)

var debugServerRPC = flag.Bool(
	"debug_server_rpc", false, "Log server RPC requests and responses")

func parse(w http.ResponseWriter, r *http.Request, req interface{}) bool {
	buf, err := ioutil.ReadAll(r.Body)
	if err != nil {
		return false
	}

	if *debugServerRPC {
		log.Printf("Request: %s", string(buf))
	}

	if err := json.Unmarshal(buf, &req); err != nil {
		http.Error(w, "json parse error", http.StatusBadRequest)
		return false
	}
	return true
}

func checkID(w http.ResponseWriter, a, b string) bool {
	if a != b {
		http.Error(w, "URL doesn't match channel ID", http.StatusBadRequest)
		return false
	}
	return true
}

func respond(w http.ResponseWriter, r *http.Request, resp interface{}, err error) {
	if err != nil {
		if *debugServerRPC {
			log.Printf("error: %v", err)
		}
		http.Error(w, "error", http.StatusInternalServerError)
	} else {
		err := json.NewEncoder(w).Encode(resp)
		if err != nil {
			log.Printf("json encode error: %v", err)
		}
	}
}

func rpcCreateHandler(s *ServerState, w http.ResponseWriter, r *http.Request) {
	var req models.CreateRequest
	if !parse(w, r, &req) {
		return
	}
	resp, err := s.Receiver.Create(req)
	respond(w, r, resp, err)
}

func rpcOpenHandler(s *ServerState, w http.ResponseWriter, r *http.Request, id string) {
	var req models.OpenRequest
	if !parse(w, r, &req) {
		return
	}
	if !checkID(w, req.ID, id) {
		return
	}
	resp, err := s.Receiver.Open(req)
	respond(w, r, resp, err)
}

func rpcSendHandler(s *ServerState, w http.ResponseWriter, r *http.Request, id string) {
	var req models.SendRequest
	if !parse(w, r, &req) {
		return
	}
	if !checkID(w, req.ID, id) {
		return
	}
	resp, err := s.Receiver.Send(req)
	respond(w, r, resp, err)
}

func rpcCloseHandler(s *ServerState, w http.ResponseWriter, r *http.Request, id string) {
	var req models.CloseRequest
	if !parse(w, r, &req) {
		return
	}
	if !checkID(w, req.ID, id) {
		return
	}
	resp, err := s.Receiver.Close(req)
	respond(w, r, resp, err)
}

const rpcPath = "/moonbeamrpc"

func rpcHandler(s *ServerState, w http.ResponseWriter, r *http.Request) {
	path := r.URL.Path

	if *debugServerRPC {
		log.Printf("%s %s", r.Method, path)
	}

	if path == rpcPath {
		if r.Method == "POST" {
			rpcCreateHandler(s, w, r)
			return
		}
		http.Error(w, "", http.StatusMethodNotAllowed)
		return
	}

	id := strings.TrimPrefix(path, rpcPath+"/")

	if !models.ValidateChannelID(id) {
		http.Error(w, "Invalid channel ID", http.StatusNotFound)
		return
	}

	switch r.Method {
	case "PATCH":
		rpcOpenHandler(s, w, r, id)
	case "POST":
		rpcSendHandler(s, w, r, id)
	case "DELETE":
		rpcCloseHandler(s, w, r, id)
	default:
		http.Error(w, "", http.StatusMethodNotAllowed)
	}
}
