package util

import (
	"github.com/tucats/ego/defs"
)

func MakeServerInfo(sessionID int32) defs.ServerInfo {
	hostName := Hostname()
	result := defs.ServerInfo{
		Hostname: hostName,
		ID:       defs.ServerInstanceID,
		Session:  int(sessionID),
		Version:  defs.APIVersion,
	}

	return result
}

func MakeBaseCollection(sessionID int32) defs.BaseCollection {
	result := defs.BaseCollection{
		ServerInfo: MakeServerInfo(sessionID),
	}

	return result
}
