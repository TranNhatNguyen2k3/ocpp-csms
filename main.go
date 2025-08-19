package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

// OCPP Message Types
const (
	MessageTypeCall       = 2
	MessageTypeCallResult = 3
	MessageTypeCallError  = 4
)

// OCPP Actions - Charge Point to Central System
const (
	ActionBootNotification              = "BootNotification"
	ActionHeartbeat                     = "Heartbeat"
	ActionStatusNotification            = "StatusNotification"
	ActionAuthorize                     = "Authorize"
	ActionStartTransaction              = "StartTransaction"
	ActionStopTransaction               = "StopTransaction"
	ActionMeterValues                   = "MeterValues"
	ActionDataTransfer                  = "DataTransfer"
	ActionDiagnosticsStatusNotification = "DiagnosticsStatusNotification"
	ActionFirmwareStatusNotification    = "FirmwareStatusNotification"
	ActionSecurityEventNotification     = "SecurityEventNotification"
)

// OCPP Actions - Central System to Charge Point
const (
	ActionCancelReservation      = "CancelReservation"
	ActionChangeAvailability     = "ChangeAvailability"
	ActionChangeConfiguration    = "ChangeConfiguration"
	ActionClearCache             = "ClearCache"
	ActionClearChargingProfile   = "ClearChargingProfile"
	ActionGetCompositeSchedule   = "GetCompositeSchedule"
	ActionGetConfiguration       = "GetConfiguration"
	ActionGetDiagnostics         = "GetDiagnostics"
	ActionGetLocalListVersion    = "GetLocalListVersion"
	ActionRemoteStartTransaction = "RemoteStartTransaction"
	ActionRemoteStopTransaction  = "RemoteStopTransaction"
	ActionReserveNow             = "ReserveNow"
	ActionReset                  = "Reset"
	ActionSendLocalList          = "SendLocalList"
	ActionSetChargingProfile     = "SetChargingProfile"
	ActionTriggerMessage         = "TriggerMessage"
	ActionUnlockConnector        = "UnlockConnector"
	ActionUpdateFirmware         = "UpdateFirmware"
)

// Registration Status
const (
	RegistrationStatusAccepted = "Accepted"
	RegistrationStatusRejected = "Rejected"
	RegistrationStatusPending  = "Pending"
)

// Authorization Status
const (
	AuthorizationStatusAccepted     = "Accepted"
	AuthorizationStatusBlocked      = "Blocked"
	AuthorizationStatusExpired      = "Expired"
	AuthorizationStatusInvalid      = "Invalid"
	AuthorizationStatusConcurrentTx = "ConcurrentTx"
)

// Availability Status
const (
	AvailabilityStatusAccepted  = "Accepted"
	AvailabilityStatusRejected  = "Rejected"
	AvailabilityStatusScheduled = "Scheduled"
)

// Availability Type
const (
	AvailabilityTypeOperative   = "Operative"
	AvailabilityTypeInoperative = "Inoperative"
)

// Reset Type
const (
	ResetTypeHard = "Hard"
	ResetTypeSoft = "Soft"
)

// Message represents an OCPP message
type Message struct {
	MessageTypeID int             `json:"messageTypeId"`
	MessageID     string          `json:"messageId"`
	Action        string          `json:"action,omitempty"`
	Payload       json.RawMessage `json:"payload,omitempty"`
}

// ChargePoint represents a connected charge point
type ChargePoint struct {
	ID              string
	Conn            *websocket.Conn
	Send            chan []byte
	Done            chan bool
	Status          string
	Vendor          string
	Model           string
	SerialNumber    string
	FirmwareVersion string
	LastHeartbeat   time.Time
	Connectors      map[int]*Connector
	IsAuthenticated bool
	mu              sync.RWMutex
}

// Connector represents a charging connector
type Connector struct {
	ID           int
	Status       string
	Availability string
	Transaction  *Transaction
}

// Transaction represents a charging transaction
type Transaction struct {
	ID          int
	ConnectorID int
	IdTag       string
	StartTime   time.Time
	StopTime    *time.Time
	MeterStart  int
	MeterStop   *int
	Reason      string
	Status      string
}

// Server represents the OCPP CSMS server
type Server struct {
	upgrader websocket.Upgrader
	clients  map[string]*ChargePoint
	mu       sync.RWMutex
	// Authentication system
	authorizedCPs  map[string]*AuthorizedCP
	authorizedTags map[string]*AuthorizedTag
}

// AuthorizedCP represents an authorized charge point
type AuthorizedCP struct {
	ID              string
	Vendor          string
	Model           string
	SerialNumber    string
	FirmwareVersion string
	IsActive        bool
	CreatedAt       time.Time
}

// AuthorizedTag represents an authorized RFID tag/card
type AuthorizedTag struct {
	ID         string
	Status     string // "Active", "Blocked", "Expired"
	ExpiryDate *time.Time
	CreatedAt  time.Time
}

// Request/Response Structures

// BootNotificationRequest
type BootNotificationRequest struct {
	ChargePointVendor       string `json:"chargePointVendor"`
	ChargePointModel        string `json:"chargePointModel"`
	ChargePointSerialNumber string `json:"chargePointSerialNumber,omitempty"`
	ChargeBoxSerialNumber   string `json:"chargeBoxSerialNumber,omitempty"`
	FirmwareVersion         string `json:"firmwareVersion,omitempty"`
	Iccid                   string `json:"iccid,omitempty"`
	Imsi                    string `json:"imsi,omitempty"`
	MeterType               string `json:"meterType,omitempty"`
	MeterSerialNumber       string `json:"meterSerialNumber,omitempty"`
}

// BootNotificationResponse
type BootNotificationResponse struct {
	Status      string `json:"status"`
	CurrentTime string `json:"currentTime"`
	Interval    int    `json:"interval"`
}

// HeartbeatResponse
type HeartbeatResponse struct {
	CurrentTime string `json:"currentTime"`
}

// StatusNotificationRequest
type StatusNotificationRequest struct {
	ConnectorId     int    `json:"connectorId"`
	ErrorCode       string `json:"errorCode"`
	Status          string `json:"status"`
	Info            string `json:"info,omitempty"`
	Timestamp       string `json:"timestamp,omitempty"`
	VendorId        string `json:"vendorId,omitempty"`
	VendorErrorCode string `json:"vendorErrorCode,omitempty"`
}

// AuthorizeRequest
type AuthorizeRequest struct {
	IdTag string `json:"idTag"`
}

// AuthorizeResponse
type AuthorizeResponse struct {
	IdTagInfo IdTagInfo `json:"idTagInfo"`
}

// IdTagInfo
type IdTagInfo struct {
	Status      string `json:"status"`
	ExpiryDate  string `json:"expiryDate,omitempty"`
	ParentIdTag string `json:"parentIdTag,omitempty"`
}

// StartTransactionRequest
type StartTransactionRequest struct {
	ConnectorId   int    `json:"connectorId"`
	IdTag         string `json:"idTag"`
	MeterStart    int    `json:"meterStart"`
	ReservationId int    `json:"reservationId,omitempty"`
	Timestamp     string `json:"timestamp,omitempty"`
}

// StartTransactionResponse
type StartTransactionResponse struct {
	TransactionId int       `json:"transactionId"`
	IdTagInfo     IdTagInfo `json:"idTagInfo"`
}

// StopTransactionRequest
type StopTransactionRequest struct {
	TransactionId   int          `json:"transactionId"`
	IdTag           string       `json:"idTag,omitempty"`
	MeterStop       int          `json:"meterStop"`
	Timestamp       string       `json:"timestamp,omitempty"`
	Reason          string       `json:"reason,omitempty"`
	TransactionData []MeterValue `json:"transactionData,omitempty"`
}

// StopTransactionResponse
type StopTransactionResponse struct {
	IdTagInfo IdTagInfo `json:"idTagInfo"`
}

// MeterValuesRequest
type MeterValuesRequest struct {
	ConnectorId   int          `json:"connectorId"`
	TransactionId int          `json:"transactionId,omitempty"`
	MeterValue    []MeterValue `json:"meterValue"`
}

// MeterValue
type MeterValue struct {
	Timestamp    string         `json:"timestamp"`
	SampledValue []SampledValue `json:"sampledValue"`
}

// SampledValue
type SampledValue struct {
	Value     string `json:"value"`
	Context   string `json:"context,omitempty"`
	Format    string `json:"format,omitempty"`
	Measurand string `json:"measurand,omitempty"`
	Phase     string `json:"phase,omitempty"`
	Location  string `json:"location,omitempty"`
	Unit      string `json:"unit,omitempty"`
}

// Measurand constants - Common OCPP 1.6J measurands
const (
	// Energy measurements
	MeasurandEnergyActiveImportRegister   = "Energy.Active.Import.Register"
	MeasurandEnergyActiveExportRegister   = "Energy.Active.Export.Register"
	MeasurandEnergyReactiveImportRegister = "Energy.Reactive.Import.Register"
	MeasurandEnergyReactiveExportRegister = "Energy.Reactive.Export.Register"
	MeasurandEnergyApparentImportRegister = "Energy.Apparent.Import.Register"
	MeasurandEnergyApparentExportRegister = "Energy.Apparent.Export.Register"

	// Power measurements
	MeasurandPowerActiveImport   = "Power.Active.Import"
	MeasurandPowerActiveExport   = "Power.Active.Export"
	MeasurandPowerReactiveImport = "Power.Reactive.Import"
	MeasurandPowerReactiveExport = "Power.Reactive.Export"
	MeasurandPowerApparentImport = "Power.Apparent.Import"
	MeasurandPowerApparentExport = "Power.Apparent.Export"
	MeasurandPowerFactor         = "Power.Factor"

	// Current measurements
	MeasurandCurrentImport  = "Current.Import"
	MeasurandCurrentExport  = "Current.Export"
	MeasurandCurrentOffered = "Current.Offered"

	// Voltage measurements
	MeasurandVoltage = "Voltage"

	// Frequency
	MeasurandFrequency = "Frequency"

	// Temperature
	MeasurandTemperature = "Temperature"

	// SoC (State of Charge)
	MeasurandSoC = "SoC"

	// RPM (for DC charging)
	MeasurandRPM = "RPM"
)

// Context constants
const (
	ContextInterruptionBegin = "Interruption.Begin"
	ContextInterruptionEnd   = "Interruption.End"
	ContextOther             = "Other"
	ContextSampleClock       = "Sample.Clock"
	ContextSamplePeriodic    = "Sample.Periodic"
	ContextTransactionBegin  = "Transaction.Begin"
	ContextTransactionEnd    = "Transaction.End"
	ContextTrigger           = "Trigger"
)

// Format constants
const (
	FormatRaw        = "Raw"
	FormatSignedData = "SignedData"
)

// Location constants
const (
	LocationBody   = "Body"
	LocationCable  = "Cable"
	LocationCharge = "Charge"
	LocationEV     = "EV"
	LocationInlet  = "Inlet"
	LocationOutlet = "Outlet"
)

// Phase constants
const (
	PhaseL1   = "L1"
	PhaseL2   = "L2"
	PhaseL3   = "L3"
	PhaseN    = "N"
	PhaseL1N  = "L1-N"
	PhaseL2N  = "L2-N"
	PhaseL3N  = "L3-N"
	PhaseL1L2 = "L1-L2"
	PhaseL2L3 = "L2-L3"
	PhaseL3L1 = "L3-L1"
)

// DataTransferRequest
type DataTransferRequest struct {
	VendorId  string `json:"vendorId"`
	MessageId string `json:"messageId,omitempty"`
	Data      string `json:"data,omitempty"`
}

// DataTransferResponse
type DataTransferResponse struct {
	Status string `json:"status"`
	Data   string `json:"data,omitempty"`
}

// DiagnosticsStatusNotificationRequest
type DiagnosticsStatusNotificationRequest struct {
	Status string `json:"status"`
}

// FirmwareStatusNotificationRequest
type FirmwareStatusNotificationRequest struct {
	Status string `json:"status"`
}

// SecurityEventNotificationRequest
type SecurityEventNotificationRequest struct {
	Type      string `json:"type"`
	Timestamp string `json:"timestamp"`
	TechCode  string `json:"techCode,omitempty"`
	TechInfo  string `json:"techInfo,omitempty"`
}

// Central System to Charge Point Request/Response Structures

// ChangeAvailabilityRequest
type ChangeAvailabilityRequest struct {
	ConnectorId int    `json:"connectorId"`
	Type        string `json:"type"`
}

// ChangeAvailabilityResponse
type ChangeAvailabilityResponse struct {
	Status string `json:"status"`
}

// GetConfigurationRequest
type GetConfigurationRequest struct {
	Key []string `json:"key,omitempty"`
}

// GetConfigurationResponse
type GetConfigurationResponse struct {
	ConfigurationKey []ConfigurationKey `json:"configurationKey"`
	UnknownKey       []string           `json:"unknownKey,omitempty"`
}

// ConfigurationKey
type ConfigurationKey struct {
	Key      string `json:"key"`
	Readonly bool   `json:"readonly"`
	Value    string `json:"value,omitempty"`
}

// ChangeConfigurationRequest
type ChangeConfigurationRequest struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

// ChangeConfigurationResponse
type ChangeConfigurationResponse struct {
	Status string `json:"status"`
}

// RemoteStartTransactionRequest
type RemoteStartTransactionRequest struct {
	ConnectorId     int              `json:"connectorId"`
	IdTag           string           `json:"idTag"`
	ChargingProfile *ChargingProfile `json:"chargingProfile,omitempty"`
}

// RemoteStartTransactionResponse
type RemoteStartTransactionResponse struct {
	Status string `json:"status"`
}

// RemoteStopTransactionRequest
type RemoteStopTransactionRequest struct {
	TransactionId int `json:"transactionId"`
}

// RemoteStopTransactionResponse
type RemoteStopTransactionResponse struct {
	Status string `json:"status"`
}

// ResetRequest
type ResetRequest struct {
	Type string `json:"type"`
}

// ResetResponse
type ResetResponse struct {
	Status string `json:"status"`
}

// UnlockConnectorRequest
type UnlockConnectorRequest struct {
	ConnectorId int `json:"connectorId"`
}

// UnlockConnectorResponse
type UnlockConnectorResponse struct {
	Status string `json:"status"`
}

// ChargingProfile
type ChargingProfile struct {
	ChargingProfileId      int              `json:"chargingProfileId"`
	TransactionId          int              `json:"transactionId,omitempty"`
	StackLevel             int              `json:"stackLevel"`
	ChargingProfilePurpose string           `json:"chargingProfilePurpose"`
	ChargingProfileKind    string           `json:"chargingProfileKind"`
	RecurrencyKind         string           `json:"recurrencyKind,omitempty"`
	ValidFrom              string           `json:"validFrom,omitempty"`
	ValidTo                string           `json:"validTo,omitempty"`
	ChargingSchedule       ChargingSchedule `json:"chargingSchedule"`
}

// ChargingSchedule
type ChargingSchedule struct {
	Duration               int                      `json:"duration,omitempty"`
	StartSchedule          string                   `json:"startSchedule,omitempty"`
	ChargingRateUnit       string                   `json:"chargingRateUnit"`
	ChargingSchedulePeriod []ChargingSchedulePeriod `json:"chargingSchedulePeriod"`
	MinChargingRate        float64                  `json:"minChargingRate,omitempty"`
}

// ChargingSchedulePeriod
type ChargingSchedulePeriod struct {
	StartPeriod  int     `json:"startPeriod"`
	Limit        float64 `json:"limit"`
	NumberPhases int     `json:"numberPhases,omitempty"`
}

// NewServer creates a new OCPP server
func NewServer() *Server {
	server := &Server{
		upgrader: websocket.Upgrader{
			CheckOrigin: func(r *http.Request) bool {
				return true // Allow all origins for development
			},
			Subprotocols: []string{"ocpp1.6"},
		},
		clients:        make(map[string]*ChargePoint),
		authorizedCPs:  make(map[string]*AuthorizedCP),
		authorizedTags: make(map[string]*AuthorizedTag),
	}

	// Initialize with some test data
	server.initializeTestData()

	return server
}

// initializeTestData adds some test charge points and tags
func (s *Server) initializeTestData() {
	// Add authorized charge points
	s.authorizedCPs["ocpp/d0a5a05c-aa94-4754-96f1-a6316dc86ce0"] = &AuthorizedCP{
		ID:              "ocpp/d0a5a05c-aa94-4754-96f1-a6316dc86ce0",
		Vendor:          "LINCHR",
		Model:           "AC_22KW_001",
		SerialNumber:    "912307000220F101050100A01",
		FirmwareVersion: "V940B00D02",
		IsActive:        true,
		CreatedAt:       time.Now(),
	}

	// Add authorized RFID tags
	s.authorizedTags["TEST_CARD_001"] = &AuthorizedTag{
		ID:         "TEST_CARD_001",
		Status:     "Active",
		ExpiryDate: nil, // No expiry
		CreatedAt:  time.Now(),
	}

	s.authorizedTags["CARD_001"] = &AuthorizedTag{
		ID:         "CARD_001",
		Status:     "Active",
		ExpiryDate: nil,
		CreatedAt:  time.Now(),
	}

	s.authorizedTags["BLOCKED_CARD"] = &AuthorizedTag{
		ID:         "BLOCKED_CARD",
		Status:     "Blocked",
		ExpiryDate: nil,
		CreatedAt:  time.Now(),
	}

	// Add expired card
	expiredTime := time.Now().Add(-24 * time.Hour)
	s.authorizedTags["EXPIRED_CARD"] = &AuthorizedTag{
		ID:         "EXPIRED_CARD",
		Status:     "Expired",
		ExpiryDate: &expiredTime,
		CreatedAt:  time.Now().Add(-30 * 24 * time.Hour),
	}

	log.Printf("Initialized with %d authorized charge points and %d authorized tags",
		len(s.authorizedCPs), len(s.authorizedTags))
}

// NewChargePoint creates a new charge point instance
func NewChargePoint(id string, conn *websocket.Conn) *ChargePoint {
	return &ChargePoint{
		ID:         id,
		Conn:       conn,
		Send:       make(chan []byte, 256),
		Done:       make(chan bool),
		Status:     "Connected",
		Connectors: make(map[int]*Connector),
	}
}

// Start starts the charge point message handling
func (cp *ChargePoint) Start() {
	go cp.writePump()
	go cp.readPump()
}

// readPump pumps messages from the websocket connection
func (cp *ChargePoint) readPump() {
	defer func() {
		cp.Conn.WriteControl(websocket.CloseMessage,
			websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""),
			time.Now().Add(time.Second))
		cp.Done <- true
		cp.Conn.Close()
	}()

	cp.Conn.SetReadDeadline(time.Now().Add(60 * time.Second))
	cp.Conn.SetPongHandler(func(string) error {
		cp.Conn.SetReadDeadline(time.Now().Add(60 * time.Second))
		return nil
	})

	for {
		_, message, err := cp.Conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("WebSocket read error for %s: %v", cp.ID, err)
			} else {
				log.Printf("WebSocket connection closed for %s: %v", cp.ID, err)
			}
			break
		}

		log.Printf("[%s] Received: %s", cp.ID, string(message))
		cp.handleMessage(message)
	}
}

// writePump pumps messages to the websocket connection
func (cp *ChargePoint) writePump() {
	ticker := time.NewTicker(30 * time.Second)
	defer func() {
		ticker.Stop()
		cp.Conn.Close()
	}()

	for {
		select {
		case message, ok := <-cp.Send:
			if !ok {
				cp.Conn.WriteControl(websocket.CloseMessage,
					websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""),
					time.Now().Add(time.Second))
				return
			}

			cp.Conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			w, err := cp.Conn.NextWriter(websocket.TextMessage)
			if err != nil {
				log.Printf("WebSocket write error for %s: %v", cp.ID, err)
				return
			}
			w.Write(message)

			if err := w.Close(); err != nil {
				log.Printf("WebSocket writer close error for %s: %v", cp.ID, err)
				return
			}
		case <-ticker.C:
			cp.Conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if err := cp.Conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				log.Printf("WebSocket ping error for %s: %v", cp.ID, err)
				return
			}
		case <-cp.Done:
			return
		}
	}
}

// handleMessage processes incoming OCPP messages
func (cp *ChargePoint) handleMessage(data []byte) {
	var arr []json.RawMessage
	if err := json.Unmarshal(data, &arr); err != nil {
		log.Printf("Error unmarshaling OCPP array: %v", err)
		return
	}
	if len(arr) < 3 {
		log.Printf("OCPP message insufficient fields: %s", string(data))
		return
	}

	var msgType int
	var msgID, action string
	var payload json.RawMessage

	if err := json.Unmarshal(arr[0], &msgType); err != nil {
		log.Printf("Error parsing message type: %v", err)
		return
	}
	if err := json.Unmarshal(arr[1], &msgID); err != nil {
		log.Printf("Error parsing message ID: %v", err)
		return
	}
	if err := json.Unmarshal(arr[2], &action); err != nil {
		action = ""
	}
	if len(arr) > 3 {
		payload = arr[3]
	} else {
		payload = json.RawMessage([]byte("{}"))
	}

	log.Printf("[%s] OCPP: type=%d, id=%s, action=%s, payload=%s",
		cp.ID, msgType, msgID, action, string(payload))

	switch msgType {
	case MessageTypeCall:
		cp.handleCall(msgID, action, payload)
	case MessageTypeCallResult:
		cp.handleCallResult(msgID, payload)
	case MessageTypeCallError:
		cp.handleCallError(msgID, payload)
	default:
		log.Printf("Unknown OCPP message type: %d", msgType)
	}
}

// handleCall processes OCPP call messages
func (cp *ChargePoint) handleCall(messageID, action string, payload json.RawMessage) {
	switch action {
	case ActionBootNotification:
		cp.handleBootNotification(messageID, payload)
	case ActionHeartbeat:
		cp.handleHeartbeat(messageID)
	case ActionStatusNotification:
		cp.handleStatusNotification(messageID, payload)
	case ActionAuthorize:
		cp.handleAuthorize(messageID, payload)
	case ActionStartTransaction:
		cp.handleStartTransaction(messageID, payload)
	case ActionStopTransaction:
		cp.handleStopTransaction(messageID, payload)
	case ActionMeterValues:
		cp.handleMeterValues(messageID, payload)
	case ActionDataTransfer:
		cp.handleDataTransfer(messageID, payload)
	case ActionDiagnosticsStatusNotification:
		cp.handleDiagnosticsStatusNotification(messageID, payload)
	case ActionFirmwareStatusNotification:
		cp.handleFirmwareStatusNotification(messageID, payload)
	case ActionSecurityEventNotification:
		cp.handleSecurityEventNotification(messageID, payload)
	default:
		log.Printf("Unsupported OCPP action: %s", action)
		cp.sendCallResult(messageID, struct{}{})
	}
}

// handleBootNotification processes boot notification requests
func (cp *ChargePoint) handleBootNotification(messageID string, payload json.RawMessage) {
	var request BootNotificationRequest
	if err := json.Unmarshal(payload, &request); err != nil {
		log.Printf("Error unmarshaling boot notification request: %v", err)
		return
	}

	cp.mu.Lock()
	cp.Vendor = request.ChargePointVendor
	cp.Model = request.ChargePointModel
	cp.SerialNumber = request.ChargePointSerialNumber
	cp.FirmwareVersion = request.FirmwareVersion
	cp.mu.Unlock()

	log.Printf("[%s] Boot notification: Vendor=%s, Model=%s, Serial=%s, Firmware=%s",
		cp.ID, request.ChargePointVendor, request.ChargePointModel,
		request.ChargePointSerialNumber, request.FirmwareVersion)

	// Check if this charge point is authorized
	server := cp.getServer()
	if server != nil {
		authorizedCP := server.authorizedCPs[cp.ID]
		if authorizedCP != nil && authorizedCP.IsActive {
			cp.mu.Lock()
			cp.IsAuthenticated = true
			cp.mu.Unlock()
			log.Printf("[%s] Charge point authenticated successfully", cp.ID)
		} else {
			log.Printf("[%s] Charge point NOT authorized - ID: %s", cp.ID, cp.ID)
		}
	}

	response := BootNotificationResponse{
		Status:      RegistrationStatusAccepted,
		CurrentTime: time.Now().UTC().Format(time.RFC3339),
		Interval:    10,
	}

	cp.sendCallResult(messageID, response)
}

// getServer returns the server instance (this is a workaround for accessing server from charge point)
func (cp *ChargePoint) getServer() *Server {
	// This is a simplified approach - in a real implementation, you'd pass server reference
	// For now, we'll use a global variable or singleton pattern
	return globalServer
}

// Global server instance for access from charge points
var globalServer *Server

// handleHeartbeat processes heartbeat requests
func (cp *ChargePoint) handleHeartbeat(messageID string) {
	cp.mu.Lock()
	cp.LastHeartbeat = time.Now()
	cp.mu.Unlock()

	response := HeartbeatResponse{
		CurrentTime: time.Now().UTC().Format(time.RFC3339),
	}

	cp.sendCallResult(messageID, response)
}

// handleStatusNotification processes status notification requests
func (cp *ChargePoint) handleStatusNotification(messageID string, payload json.RawMessage) {
	var request StatusNotificationRequest
	if err := json.Unmarshal(payload, &request); err != nil {
		log.Printf("Error unmarshaling status notification request: %v", err)
		return
	}

	cp.mu.Lock()
	if cp.Connectors[request.ConnectorId] == nil {
		cp.Connectors[request.ConnectorId] = &Connector{
			ID:           request.ConnectorId,
			Status:       request.Status,
			Availability: "Operative",
		}
	} else {
		cp.Connectors[request.ConnectorId].Status = request.Status
	}
	cp.mu.Unlock()

	log.Printf("[%s] Status notification: Connector=%d, Status=%s, ErrorCode=%s",
		cp.ID, request.ConnectorId, request.Status, request.ErrorCode)

	cp.sendCallResult(messageID, struct{}{})
}

// handleAuthorize processes authorize requests
func (cp *ChargePoint) handleAuthorize(messageID string, payload json.RawMessage) {
	var request AuthorizeRequest
	if err := json.Unmarshal(payload, &request); err != nil {
		log.Printf("Error unmarshaling authorize request: %v", err)
		return
	}

	log.Printf("[%s] Authorize request: IdTag=%s", cp.ID, request.IdTag)

	// Check if charge point is authenticated
	cp.mu.RLock()
	isAuthenticated := cp.IsAuthenticated
	cp.mu.RUnlock()

	if !isAuthenticated {
		log.Printf("[%s] Authorization rejected - Charge point not authenticated", cp.ID)
		response := AuthorizeResponse{
			IdTagInfo: IdTagInfo{
				Status: AuthorizationStatusInvalid,
			},
		}
		cp.sendCallResult(messageID, response)
		return
	}

	// Check if the RFID tag is authorized
	server := cp.getServer()
	var status string
	var expiryDate string

	if server != nil {
		authorizedTag := server.authorizedTags[request.IdTag]
		if authorizedTag != nil {
			// Check if tag is expired
			if authorizedTag.ExpiryDate != nil && time.Now().After(*authorizedTag.ExpiryDate) {
				status = AuthorizationStatusExpired
				log.Printf("[%s] Tag %s is expired", cp.ID, request.IdTag)
			} else if authorizedTag.Status == "Active" {
				status = AuthorizationStatusAccepted
				if authorizedTag.ExpiryDate != nil {
					expiryDate = authorizedTag.ExpiryDate.Format(time.RFC3339)
				}
				log.Printf("[%s] Tag %s authorized successfully", cp.ID, request.IdTag)
			} else if authorizedTag.Status == "Blocked" {
				status = AuthorizationStatusBlocked
				log.Printf("[%s] Tag %s is blocked", cp.ID, request.IdTag)
			} else {
				status = AuthorizationStatusInvalid
				log.Printf("[%s] Tag %s has invalid status: %s", cp.ID, request.IdTag, authorizedTag.Status)
			}
		} else {
			status = AuthorizationStatusInvalid
			log.Printf("[%s] Tag %s not found in authorized list", cp.ID, request.IdTag)
		}
	} else {
		status = AuthorizationStatusInvalid
		log.Printf("[%s] Server not available for authorization", cp.ID)
	}

	response := AuthorizeResponse{
		IdTagInfo: IdTagInfo{
			Status:     status,
			ExpiryDate: expiryDate,
		},
	}

	cp.sendCallResult(messageID, response)
}

// handleStartTransaction processes start transaction requests
func (cp *ChargePoint) handleStartTransaction(messageID string, payload json.RawMessage) {
	var request StartTransactionRequest
	if err := json.Unmarshal(payload, &request); err != nil {
		log.Printf("Error unmarshaling start transaction request: %v", err)
		return
	}

	// Check if charge point is authenticated
	cp.mu.RLock()
	isAuthenticated := cp.IsAuthenticated
	cp.mu.RUnlock()

	if !isAuthenticated {
		log.Printf("[%s] StartTransaction rejected - Charge point not authenticated", cp.ID)
		response := StartTransactionResponse{
			TransactionId: 0,
			IdTagInfo: IdTagInfo{
				Status: AuthorizationStatusInvalid,
			},
		}
		cp.sendCallResult(messageID, response)
		return
	}

	// Check if the RFID tag is authorized
	server := cp.getServer()
	var status string

	if server != nil {
		authorizedTag := server.authorizedTags[request.IdTag]
		if authorizedTag != nil && authorizedTag.Status == "Active" {
			// Check if tag is expired
			if authorizedTag.ExpiryDate != nil && time.Now().After(*authorizedTag.ExpiryDate) {
				status = AuthorizationStatusExpired
				log.Printf("[%s] StartTransaction rejected - Tag %s is expired", cp.ID, request.IdTag)
			} else {
				status = AuthorizationStatusAccepted
				log.Printf("[%s] StartTransaction authorized for tag %s", cp.ID, request.IdTag)
			}
		} else {
			status = AuthorizationStatusInvalid
			log.Printf("[%s] StartTransaction rejected - Tag %s not authorized", cp.ID, request.IdTag)
		}
	} else {
		status = AuthorizationStatusInvalid
		log.Printf("[%s] StartTransaction rejected - Server not available", cp.ID)
	}

	transactionID := 0
	if status == AuthorizationStatusAccepted {
		transactionID = int(time.Now().Unix())

		cp.mu.Lock()
		if cp.Connectors[request.ConnectorId] != nil {
			cp.Connectors[request.ConnectorId].Transaction = &Transaction{
				ID:          transactionID,
				ConnectorID: request.ConnectorId,
				IdTag:       request.IdTag,
				StartTime:   time.Now(),
				MeterStart:  request.MeterStart,
				Status:      "Active",
			}
		}
		cp.mu.Unlock()

		log.Printf("[%s] Transaction started: Connector=%d, IdTag=%s, TransactionId=%d",
			cp.ID, request.ConnectorId, request.IdTag, transactionID)
	}

	response := StartTransactionResponse{
		TransactionId: transactionID,
		IdTagInfo: IdTagInfo{
			Status: status,
		},
	}

	cp.sendCallResult(messageID, response)
}

// handleStopTransaction processes stop transaction requests
func (cp *ChargePoint) handleStopTransaction(messageID string, payload json.RawMessage) {
	var request StopTransactionRequest
	if err := json.Unmarshal(payload, &request); err != nil {
		log.Printf("Error unmarshaling stop transaction request: %v", err)
		return
	}

	cp.mu.Lock()
	// Find and update transaction
	for _, connector := range cp.Connectors {
		if connector.Transaction != nil && connector.Transaction.ID == request.TransactionId {
			stopTime := time.Now()
			connector.Transaction.StopTime = &stopTime
			connector.Transaction.MeterStop = &request.MeterStop
			connector.Transaction.Reason = request.Reason
			connector.Transaction.Status = "Completed"
			break
		}
	}
	cp.mu.Unlock()

	log.Printf("[%s] Stop transaction: TransactionId=%d, Reason=%s",
		cp.ID, request.TransactionId, request.Reason)

	response := StopTransactionResponse{
		IdTagInfo: IdTagInfo{
			Status: AuthorizationStatusAccepted,
		},
	}

	cp.sendCallResult(messageID, response)
}

// handleMeterValues processes meter values requests
func (cp *ChargePoint) handleMeterValues(messageID string, payload json.RawMessage) {
	var request MeterValuesRequest
	if err := json.Unmarshal(payload, &request); err != nil {
		log.Printf("Error unmarshaling meter values request: %v", err)
		return
	}

	log.Printf("[%s] Meter values: Connector=%d, TransactionId=%d, Values=%d",
		cp.ID, request.ConnectorId, request.TransactionId, len(request.MeterValue))

	// Process meter values with detailed logging
	for i, meterValue := range request.MeterValue {
		log.Printf("[%s] MeterValue[%d]: Timestamp=%s, SampledValues=%d",
			cp.ID, i, meterValue.Timestamp, len(meterValue.SampledValue))

		for j, sampledValue := range meterValue.SampledValue {
			log.Printf("[%s]   SampledValue[%d]: Value=%s, Unit=%s, Measurand=%s, Context=%s, Format=%s, Location=%s, Phase=%s",
				cp.ID, j, sampledValue.Value, sampledValue.Unit, sampledValue.Measurand,
				sampledValue.Context, sampledValue.Format, sampledValue.Location, sampledValue.Phase)

			// Process specific measurands
			cp.processMeasurand(sampledValue, request.ConnectorId, request.TransactionId)
		}
	}

	cp.sendCallResult(messageID, struct{}{})
}

// processMeasurand processes individual measurand values
func (cp *ChargePoint) processMeasurand(sampledValue SampledValue, connectorId int, transactionId int) {
	switch sampledValue.Measurand {
	case MeasurandEnergyActiveImportRegister:
		log.Printf("[%s] ⚡ Energy Import: %s %s (Connector: %d, Transaction: %d)",
			cp.ID, sampledValue.Value, sampledValue.Unit, connectorId, transactionId)

	case MeasurandEnergyActiveExportRegister:
		log.Printf("[%s] ⚡ Energy Export: %s %s (Connector: %d, Transaction: %d)",
			cp.ID, sampledValue.Value, sampledValue.Unit, connectorId, transactionId)

	case MeasurandPowerActiveImport:
		log.Printf("[%s] 🔌 Power Import: %s %s (Connector: %d, Transaction: %d)",
			cp.ID, sampledValue.Value, sampledValue.Unit, connectorId, transactionId)

	case MeasurandPowerActiveExport:
		log.Printf("[%s] 🔌 Power Export: %s %s (Connector: %d, Transaction: %d)",
			cp.ID, sampledValue.Value, sampledValue.Unit, connectorId, transactionId)

	case MeasurandCurrentImport:
		log.Printf("[%s] 🔋 Current Import: %s %s (Connector: %d, Transaction: %d)",
			cp.ID, sampledValue.Value, sampledValue.Unit, connectorId, transactionId)

	case MeasurandVoltage:
		log.Printf("[%s] ⚡ Voltage: %s %s (Connector: %d, Transaction: %d)",
			cp.ID, sampledValue.Value, sampledValue.Unit, connectorId, transactionId)

	case MeasurandFrequency:
		log.Printf("[%s] 🔄 Frequency: %s %s (Connector: %d, Transaction: %d)",
			cp.ID, sampledValue.Value, sampledValue.Unit, connectorId, transactionId)

	case MeasurandTemperature:
		log.Printf("[%s] 🌡️ Temperature: %s %s (Connector: %d, Transaction: %d)",
			cp.ID, sampledValue.Value, sampledValue.Unit, connectorId, transactionId)

	case MeasurandSoC:
		log.Printf("[%s] 🔋 State of Charge: %s %s (Connector: %d, Transaction: %d)",
			cp.ID, sampledValue.Value, sampledValue.Unit, connectorId, transactionId)

	case MeasurandPowerFactor:
		log.Printf("[%s] 📊 Power Factor: %s %s (Connector: %d, Transaction: %d)",
			cp.ID, sampledValue.Value, sampledValue.Unit, connectorId, transactionId)

	case MeasurandRPM:
		log.Printf("[%s] 🔄 RPM: %s %s (Connector: %d, Transaction: %d)",
			cp.ID, sampledValue.Value, sampledValue.Unit, connectorId, transactionId)

	default:
		log.Printf("[%s] 📈 Other Measurand: %s = %s %s (Connector: %d, Transaction: %d)",
			cp.ID, sampledValue.Measurand, sampledValue.Value, sampledValue.Unit, connectorId, transactionId)
	}
}

// handleDataTransfer processes data transfer requests
func (cp *ChargePoint) handleDataTransfer(messageID string, payload json.RawMessage) {
	var request DataTransferRequest
	if err := json.Unmarshal(payload, &request); err != nil {
		log.Printf("Error unmarshaling data transfer request: %v", err)
		return
	}

	log.Printf("[%s] Data transfer: VendorId=%s, MessageId=%s, Data=%s",
		cp.ID, request.VendorId, request.MessageId, request.Data)

	response := DataTransferResponse{
		Status: "Accepted",
	}

	cp.sendCallResult(messageID, response)
}

// handleDiagnosticsStatusNotification processes diagnostics status notification
func (cp *ChargePoint) handleDiagnosticsStatusNotification(messageID string, payload json.RawMessage) {
	var request DiagnosticsStatusNotificationRequest
	if err := json.Unmarshal(payload, &request); err != nil {
		log.Printf("Error unmarshaling diagnostics status notification: %v", err)
		return
	}

	log.Printf("[%s] Diagnostics status: %s", cp.ID, request.Status)

	cp.sendCallResult(messageID, struct{}{})
}

// handleFirmwareStatusNotification processes firmware status notification
func (cp *ChargePoint) handleFirmwareStatusNotification(messageID string, payload json.RawMessage) {
	var request FirmwareStatusNotificationRequest
	if err := json.Unmarshal(payload, &request); err != nil {
		log.Printf("Error unmarshaling firmware status notification: %v", err)
		return
	}

	log.Printf("[%s] Firmware status: %s", cp.ID, request.Status)

	cp.sendCallResult(messageID, struct{}{})
}

// handleSecurityEventNotification processes security event notification
func (cp *ChargePoint) handleSecurityEventNotification(messageID string, payload json.RawMessage) {
	var request SecurityEventNotificationRequest
	if err := json.Unmarshal(payload, &request); err != nil {
		log.Printf("Error unmarshaling security event notification: %v", err)
		return
	}

	log.Printf("[%s] Security event: Type=%s, TechCode=%s, TechInfo=%s",
		cp.ID, request.Type, request.TechCode, request.TechInfo)

	cp.sendCallResult(messageID, struct{}{})
}

// handleCallResult processes OCPP call result messages
func (cp *ChargePoint) handleCallResult(messageID string, payload json.RawMessage) {
	log.Printf("[%s] Call result: %s", cp.ID, string(payload))
}

// handleCallError processes OCPP call error messages
func (cp *ChargePoint) handleCallError(messageID string, payload json.RawMessage) {
	log.Printf("[%s] Call error: %s", cp.ID, string(payload))
}

// sendCallResult sends a call result message
func (cp *ChargePoint) sendCallResult(messageID string, payload interface{}) {
	response := []interface{}{
		MessageTypeCallResult,
		messageID,
		payload,
	}

	responseBytes, err := json.Marshal(response)
	if err != nil {
		log.Printf("Error marshaling response: %v", err)
		return
	}

	log.Printf("[%s] Sending: %s", cp.ID, string(responseBytes))
	cp.Send <- responseBytes
}

// sendCall sends a call message to the charge point
func (cp *ChargePoint) sendCall(action string, payload interface{}) string {
	messageID := fmt.Sprintf("%d", time.Now().UnixNano())

	call := []interface{}{
		MessageTypeCall,
		messageID,
		action,
		payload,
	}

	callBytes, err := json.Marshal(call)
	if err != nil {
		log.Printf("Error marshaling call: %v", err)
		return ""
	}

	log.Printf("[%s] Sending call: %s", cp.ID, string(callBytes))
	cp.Send <- callBytes
	return messageID
}

// Central System to Charge Point methods

// ChangeAvailability sends a change availability request
func (cp *ChargePoint) ChangeAvailability(connectorId int, availabilityType string) {
	request := ChangeAvailabilityRequest{
		ConnectorId: connectorId,
		Type:        availabilityType,
	}

	cp.sendCall(ActionChangeAvailability, request)
}

// GetConfiguration sends a get configuration request
func (cp *ChargePoint) GetConfiguration(keys []string) {
	request := GetConfigurationRequest{
		Key: keys,
	}

	cp.sendCall(ActionGetConfiguration, request)
}

// ChangeConfiguration sends a change configuration request
func (cp *ChargePoint) ChangeConfiguration(key, value string) {
	request := ChangeConfigurationRequest{
		Key:   key,
		Value: value,
	}

	cp.sendCall(ActionChangeConfiguration, request)
}

// RemoteStartTransaction sends a remote start transaction request
func (cp *ChargePoint) RemoteStartTransaction(connectorId int, idTag string) {
	request := RemoteStartTransactionRequest{
		ConnectorId: connectorId,
		IdTag:       idTag,
	}

	cp.sendCall(ActionRemoteStartTransaction, request)
}

// RemoteStopTransaction sends a remote stop transaction request
func (cp *ChargePoint) RemoteStopTransaction(transactionId int) {
	request := RemoteStopTransactionRequest{
		TransactionId: transactionId,
	}

	cp.sendCall(ActionRemoteStopTransaction, request)
}

// Reset sends a reset request
func (cp *ChargePoint) Reset(resetType string) {
	request := ResetRequest{
		Type: resetType,
	}

	cp.sendCall(ActionReset, request)
}

// UnlockConnector sends an unlock connector request
func (cp *ChargePoint) UnlockConnector(connectorId int) {
	request := UnlockConnectorRequest{
		ConnectorId: connectorId,
	}

	cp.sendCall(ActionUnlockConnector, request)
}

// handleWebSocket handles incoming WebSocket connections
func (s *Server) handleWebSocket(w http.ResponseWriter, r *http.Request) {
	chargePointID := strings.Trim(r.URL.Path, "/")
	if chargePointID == "" {
		http.Error(w, "Charge point ID required", http.StatusBadRequest)
		return
	}

	// Handle CORS preflight
	if r.Method == "OPTIONS" {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		w.WriteHeader(http.StatusOK)
		return
	}

	// Check if this is a WebSocket upgrade request
	if !websocket.IsWebSocketUpgrade(r) {
		http.Error(w, "WebSocket upgrade required", http.StatusBadRequest)
		return
	}

	// Upgrade HTTP connection to WebSocket
	conn, err := s.upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("Error upgrading connection for %s: %v", chargePointID, err)
		return
	}

	// Check if subprotocol matches
	if conn.Subprotocol() != "ocpp1.6" {
		log.Printf("Protocol mismatch for %s: expected ocpp1.6, got %s",
			chargePointID, conn.Subprotocol())
		conn.WriteControl(websocket.CloseMessage,
			websocket.FormatCloseMessage(websocket.CloseProtocolError, "Invalid subprotocol"),
			time.Now().Add(time.Second))
		conn.Close()
		return
	}

	// Create charge point instance
	cp := NewChargePoint(chargePointID, conn)

	s.mu.Lock()
	s.clients[chargePointID] = cp
	s.mu.Unlock()

	log.Printf("New charge point connected: %s", chargePointID)

	// Start message handling
	cp.Start()

	// Wait for connection to close
	<-cp.Done

	// Clean up
	s.mu.Lock()
	delete(s.clients, chargePointID)
	s.mu.Unlock()

	log.Printf("Charge point disconnected: %s", chargePointID)
}

// GetChargePoint returns a charge point by ID
func (s *Server) GetChargePoint(id string) *ChargePoint {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.clients[id]
}

// GetAllChargePoints returns all connected charge points
func (s *Server) GetAllChargePoints() map[string]*ChargePoint {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make(map[string]*ChargePoint)
	for id, cp := range s.clients {
		result[id] = cp
	}
	return result
}

func main() {
	server := NewServer()
	globalServer = server // Set global server instance

	// Set up HTTP routes
	http.HandleFunc("/ocpp/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		server.handleWebSocket(w, r)
	})

	// Add a simple status endpoint
	http.HandleFunc("/status", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		chargePoints := server.GetAllChargePoints()

		status := struct {
			ServerTime     string            `json:"serverTime"`
			ChargePoints   int               `json:"chargePoints"`
			Connections    map[string]string `json:"connections"`
			AuthorizedCPs  int               `json:"authorizedCPs"`
			AuthorizedTags int               `json:"authorizedTags"`
		}{
			ServerTime:     time.Now().UTC().Format(time.RFC3339),
			ChargePoints:   len(chargePoints),
			Connections:    make(map[string]string),
			AuthorizedCPs:  len(server.authorizedCPs),
			AuthorizedTags: len(server.authorizedTags),
		}

		for id, cp := range chargePoints {
			cp.mu.RLock()
			status.Connections[id] = cp.Status
			cp.mu.RUnlock()
		}

		json.NewEncoder(w).Encode(status)
	})

	// Add authentication management endpoints
	http.HandleFunc("/auth/cps", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(server.authorizedCPs)
	})

	http.HandleFunc("/auth/tags", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(server.authorizedTags)
	})

	// Add measurands info endpoint
	http.HandleFunc("/measurands", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		measurands := struct {
			Energy    []string `json:"energy"`
			Power     []string `json:"power"`
			Current   []string `json:"current"`
			Voltage   []string `json:"voltage"`
			Other     []string `json:"other"`
			Contexts  []string `json:"contexts"`
			Locations []string `json:"locations"`
			Phases    []string `json:"phases"`
		}{
			Energy: []string{
				MeasurandEnergyActiveImportRegister,
				MeasurandEnergyActiveExportRegister,
				MeasurandEnergyReactiveImportRegister,
				MeasurandEnergyReactiveExportRegister,
				MeasurandEnergyApparentImportRegister,
				MeasurandEnergyApparentExportRegister,
			},
			Power: []string{
				MeasurandPowerActiveImport,
				MeasurandPowerActiveExport,
				MeasurandPowerReactiveImport,
				MeasurandPowerReactiveExport,
				MeasurandPowerApparentImport,
				MeasurandPowerApparentExport,
				MeasurandPowerFactor,
			},
			Current: []string{
				MeasurandCurrentImport,
				MeasurandCurrentExport,
				MeasurandCurrentOffered,
			},
			Voltage: []string{
				MeasurandVoltage,
			},
			Other: []string{
				MeasurandFrequency,
				MeasurandTemperature,
				MeasurandSoC,
				MeasurandRPM,
			},
			Contexts: []string{
				ContextInterruptionBegin,
				ContextInterruptionEnd,
				ContextOther,
				ContextSampleClock,
				ContextSamplePeriodic,
				ContextTransactionBegin,
				ContextTransactionEnd,
				ContextTrigger,
			},
			Locations: []string{
				LocationBody,
				LocationCable,
				LocationCharge,
				LocationEV,
				LocationInlet,
				LocationOutlet,
			},
			Phases: []string{
				PhaseL1, PhaseL2, PhaseL3, PhaseN,
				PhaseL1N, PhaseL2N, PhaseL3N,
				PhaseL1L2, PhaseL2L3, PhaseL3L1,
			},
		}

		json.NewEncoder(w).Encode(measurands)
	})

	port := ":9001"
	log.Printf("OCPP CSMS Server starting on port %s...", port)
	log.Printf("WebSocket endpoint: ws://localhost%s/ocpp/{chargePointId}", port)
	log.Printf("Status endpoint: http://localhost%s/status", port)
	log.Printf("Auth CPs endpoint: http://localhost%s/auth/cps", port)
	log.Printf("Auth Tags endpoint: http://localhost%s/auth/tags", port)
	log.Printf("Measurands endpoint: http://localhost%s/measurands", port)

	err := http.ListenAndServe(port, nil)
	if err != nil {
		log.Fatal("ListenAndServe: ", err)
	}
}
