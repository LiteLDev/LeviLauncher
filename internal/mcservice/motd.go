package mcservice

import (
	"encoding/binary"
	"net"
	"strconv"
	"strings"
	"time"
)

// MotdBEInfo represents the server status information
type MotdBEInfo struct {
	Status         string `json:"status"`           // online/offline
	Host           string `json:"host"`             // Server Host
	Motd           string `json:"motd"`             // Motd message
	Agreement      int    `json:"agreement"`        // Protocol version
	Version        string `json:"version"`          // Game version
	Online         int    `json:"online"`           // Online players
	Max            int    `json:"max"`              // Max players
	LevelName      string `json:"level_name"`       // Level name
	GameMode       string `json:"gamemode"`         // Game mode
	ServerUniqueID string `json:"server_unique_id"` // Server Unique ID
	Delay          int64  `json:"delay"`            // Latency in ms
}

func MotdBE(Host string) (*MotdBEInfo, error) {
	errorReturn := &MotdBEInfo{
		Status: "offline",
		Host:   Host,
	}

	if Host == "" {
		return errorReturn, nil
	}

	Socket, err := net.DialTimeout("udp", Host, 3*time.Second)
	if err != nil {
		return errorReturn, err
	}
	defer Socket.Close()

	PacketID := []byte{0x01}
	ClientSendTime := make([]byte, 8)
	binary.BigEndian.PutUint64(ClientSendTime, uint64(time.Now().UnixMilli()))
	Magic := []byte{0x00, 0xFF, 0xFF, 0x00, 0xFE, 0xFE, 0xFE, 0xFE, 0xFD, 0xFD, 0xFD, 0xFD}
	ClientID := []byte{0x12, 0x34, 0x56, 0x78, 0x00}
	ClientGUID := make([]byte, 8)
	binary.BigEndian.PutUint64(ClientGUID, 0)

	SendData := append(PacketID, ClientSendTime...)
	SendData = append(SendData, Magic...)
	SendData = append(SendData, ClientID...)
	SendData = append(SendData, ClientGUID...)

	StartTime := time.Now().UnixNano() / 1e6
	_, err = Socket.Write(SendData)
	if err != nil {
		return errorReturn, err
	}

	UDPdata := make([]byte, 2048)
	Socket.SetReadDeadline(time.Now().Add(3 * time.Second))

	n, err := Socket.Read(UDPdata)
	if err != nil {
		return errorReturn, err
	}
	EndTime := time.Now().UnixNano() / 1e6

	UDPdata = UDPdata[:n]

	if len(UDPdata) < 33 {
		return errorReturn, nil
	}

	ServerInfo := UDPdata[33:]

	MotdData := strings.Split(string(ServerInfo), ";")

	if len(MotdData) < 9 {
		return errorReturn, nil
	}

	MOTD1 := MotdData[1]           // Server MOTD line 1
	ProtocolVersion := MotdData[2] // Protocol version
	VersionName := MotdData[3]     // Game version
	PlayerCount := MotdData[4]     // Online players
	MaxPlayerCount := MotdData[5]  // Max players
	ServerUniqueID := MotdData[6]  // Server Unique ID
	MOTD2 := MotdData[7]           // Server MOTD line 2
	GameMode := MotdData[8]        // Game mode

	ProtocolVersionInt, _ := strconv.Atoi(ProtocolVersion)
	PlayerCountInt, _ := strconv.Atoi(PlayerCount)
	MaxPlayerCountInt, _ := strconv.Atoi(MaxPlayerCount)

	MotdInfo := &MotdBEInfo{
		Status:         "online",
		Host:           Host,
		Motd:           MOTD1,
		Agreement:      ProtocolVersionInt,
		Version:        VersionName,
		Online:         PlayerCountInt,
		Max:            MaxPlayerCountInt,
		LevelName:      MOTD2,
		GameMode:       GameMode,
		ServerUniqueID: ServerUniqueID,
		Delay:          EndTime - StartTime,
	}
	return MotdInfo, nil
}
