package main

import (
	"encoding/hex"
	"encoding/json"
	"flag"
	"io/ioutil"
	"log"
	"net/http"
	"strconv"
	"strings"

	"bitbucket.org/bitx/moonchan/models"
	"bitbucket.org/bitx/moonchan/receiver"
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

func splitTxIDVout(txidvout string) (string, uint32, bool) {
	i := strings.Index(txidvout, "-")
	if i < 0 {
		return "", 0, false
	}

	txid := txidvout[:i]
	vouts := txidvout[i+1:]

	if len(txid) != 64 {
		return "", 0, false
	}
	if txid != strings.ToLower(txid) {
		return "", 0, false
	}
	if _, err := hex.DecodeString(txid); err != nil {
		return "", 0, false
	}

	if len(vouts) == 0 {
		return "", 0, false
	}
	vout, err := strconv.Atoi(vouts)
	if err != nil {
		return "", 0, false
	}
	if vout < 0 {
		return "", 0, false
	}

	return txid, uint32(vout), true
}

func checkID(w http.ResponseWriter, atxid string, avout uint32, btxid string, bvout uint32) bool {
	if !(atxid == btxid && avout == bvout) {
		http.Error(w, "URL doesn't match channel ID", http.StatusBadRequest)
		return false
	}
	return true
}

func checkAuthToken(s *ServerState, r *http.Request, txid string, vout uint32) bool {
	h := r.Header.Get("Authorization")
	const prefix = "Bearer "
	if !strings.HasPrefix(h, prefix) {
		return false
	}
	h = h[len(prefix):]
	return s.Receiver.ValidateToken(txid, vout, h)
}

func respond(w http.ResponseWriter, r *http.Request, resp interface{}, err error) {
	if err != nil {
		if *debugServerRPC {
			log.Printf("error: %v", err)
		}

		if ee, ok := err.(receiver.ExposableError); ok {
			http.Error(w, ee.Error(), http.StatusBadRequest)
		} else {
			http.Error(w, "error", http.StatusInternalServerError)
		}
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

func rpcOpenHandler(s *ServerState, w http.ResponseWriter, r *http.Request, txid string, vout uint32) {
	var req models.OpenRequest
	if !parse(w, r, &req) {
		return
	}
	if !checkID(w, txid, vout, req.TxID, req.Vout) {
		return
	}
	resp, err := s.Receiver.Open(req)
	respond(w, r, resp, err)
}

func rpcValidateHandler(s *ServerState, w http.ResponseWriter, r *http.Request, txid string, vout uint32) {
	var req models.ValidateRequest
	if !parse(w, r, &req) {
		return
	}
	if !checkID(w, txid, vout, req.TxID, req.Vout) {
		return
	}
	resp, err := s.Receiver.Validate(req)
	respond(w, r, resp, err)
}

func rpcSendHandler(s *ServerState, w http.ResponseWriter, r *http.Request, txid string, vout uint32) {
	var req models.SendRequest
	if !parse(w, r, &req) {
		return
	}
	if !checkID(w, txid, vout, req.TxID, req.Vout) {
		return
	}
	resp, err := s.Receiver.Send(req)
	respond(w, r, resp, err)
}

func rpcCloseHandler(s *ServerState, w http.ResponseWriter, r *http.Request, txid string, vout uint32) {
	var req models.CloseRequest
	if !parse(w, r, &req) {
		return
	}
	if !checkID(w, txid, vout, req.TxID, req.Vout) {
		return
	}
	resp, err := s.Receiver.Close(req)
	respond(w, r, resp, err)
}

func rpcStatusHandler(s *ServerState, w http.ResponseWriter, r *http.Request, txid string, vout uint32) {
	var req models.StatusRequest
	if !parse(w, r, &req) {
		return
	}
	if !checkID(w, txid, vout, req.TxID, req.Vout) {
		return
	}
	resp, err := s.Receiver.Status(req)
	respond(w, r, resp, err)
}

const rpcPath = "/moonbeamrpc"

func rpcHandler(s *ServerState, w http.ResponseWriter, r *http.Request) {
	if *debugServerRPC {
		log.Printf("%s %s", r.Method, r.URL.Path)
	}

	if r.URL.Path == rpcPath+"/create" {
		if r.Method == http.MethodPost {
			rpcCreateHandler(s, w, r)
			return
		}
		http.Error(w, "", http.StatusMethodNotAllowed)
		return
	}

	path := strings.TrimPrefix(r.URL.Path, rpcPath+"/")

	i := strings.Index(path, "/")
	if i < 0 {
		http.NotFound(w, r)
		return
	}
	call := path[:i]
	txid, vout, ok := splitTxIDVout(path[i+1:])
	if !ok {
		http.Error(w, "Invalid channel ID", http.StatusNotFound)
		return
	}

	if call == "open" {
		rpcOpenHandler(s, w, r, txid, vout)
		return
	}

	if !checkAuthToken(s, r, txid, vout) {
		http.Error(w, "invalid auth token", http.StatusUnauthorized)
		return
	}

	switch call {
	case "validate":
		rpcValidateHandler(s, w, r, txid, vout)
	case "send":
		rpcSendHandler(s, w, r, txid, vout)
	case "close":
		rpcCloseHandler(s, w, r, txid, vout)
	case "status":
		rpcStatusHandler(s, w, r, txid, vout)
	default:
		http.Error(w, "", http.StatusMethodNotAllowed)
	}
}
