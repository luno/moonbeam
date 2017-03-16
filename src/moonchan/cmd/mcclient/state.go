package main

import (
	"encoding/json"
	"os"
	"strconv"
	"time"

	"moonchan/channels"
)

type State struct {
	Channels map[string]channels.SimpleSharedState
}

func newState() *State {
	return &State{
		Channels: make(map[string]channels.SimpleSharedState),
	}
}

const name = "client-state.json"

func save(s *State) error {
	suffix := strconv.FormatInt(time.Now().Unix(), 10)

	tmpName := name + ".tmp." + suffix

	f, err := os.Create(tmpName)
	if err != nil {
		return err
	}
	defer f.Close()

	if err := json.NewEncoder(f).Encode(s); err != nil {
		return err
	}

	if err := f.Close(); err != nil {
		return err
	}

	return os.Rename(tmpName, name)
}

func load() (*State, error) {
	f, err := os.Open(name)
	if os.IsNotExist(err) {
		return newState(), nil
	} else if err != nil {
		return nil, err
	}
	defer f.Close()

	var s State
	if err := json.NewDecoder(f).Decode(&s); err != nil {
		return nil, err
	}

	return &s, nil
}

var globalState *State
