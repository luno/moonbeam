package main

import (
	"encoding/json"
	"log"
	"net/http"

	"moonchan/models"
)

func parse(w http.ResponseWriter, r *http.Request, req interface{}) bool {
	if err := json.NewDecoder(r.Body).Decode(req); err != nil {
		http.Error(w, "json parse error", http.StatusBadRequest)
		return false
	}
	return true
}

func respond(w http.ResponseWriter, r *http.Request, resp interface{}, err error) {
	if err != nil {
		log.Printf("error: %v", err)
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
	log.Printf("req: %+v", req)
	resp, err := s.Receiver.Create(req)
	respond(w, r, resp, err)
}

func rpcOpenHandler(s *ServerState, w http.ResponseWriter, r *http.Request) {
	var req models.OpenRequest
	if !parse(w, r, &req) {
		return
	}
	log.Printf("req: %+v", req)
	resp, err := s.Receiver.Open(req)
	respond(w, r, resp, err)
}

func rpcSendHandler(s *ServerState, w http.ResponseWriter, r *http.Request) {
	var req models.SendRequest
	if !parse(w, r, &req) {
		return
	}
	log.Printf("req: %+v", req)
	resp, err := s.Receiver.Send(req)
	respond(w, r, resp, err)
}

func rpcCloseHandler(s *ServerState, w http.ResponseWriter, r *http.Request) {
	var req models.CloseRequest
	if !parse(w, r, &req) {
		return
	}
	log.Printf("req: %+v", req)
	resp, err := s.Receiver.Close(req)
	respond(w, r, resp, err)
}
