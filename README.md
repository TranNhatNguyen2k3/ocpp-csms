# OCPP 1.6J CSMS Server

Một Central System Management System (CSMS) chuẩn OCPP 1.6J được viết bằng Go, hỗ trợ đầy đủ các message types và WebSocket communication.

## Tính năng

### Charge Point to Central System Messages
- ✅ BootNotification
- ✅ Heartbeat
- ✅ StatusNotification
- ✅ Authorize
- ✅ StartTransaction
- ✅ StopTransaction
- ✅ MeterValues
- ✅ DataTransfer
- ✅ DiagnosticsStatusNotification
- ✅ FirmwareStatusNotification
- ✅ SecurityEventNotification

### Central System to Charge Point Messages
- ✅ ChangeAvailability
- ✅ GetConfiguration
- ✅ ChangeConfiguration
- ✅ RemoteStartTransaction
- ✅ RemoteStopTransaction
- ✅ Reset
- ✅ UnlockConnector

## Cài đặt

1. Đảm bảo bạn đã cài đặt Go (version 1.16+)
2. Clone repository này
3. Chạy lệnh:

```bash
go mod tidy
go run main.go
```

## Sử dụng

### Khởi động server

```bash
go run main.go
```

Server sẽ khởi động trên port 9001 với các endpoint:

- **WebSocket**: `ws://localhost:9001/ocpp/{chargePointId}`
- **Status API**: `http://localhost:9001/status`

### Kết nối Charge Point

Charge Point có thể kết nối qua WebSocket với URL:
```
ws://localhost:9001/ocpp/{chargePointId}
```

Trong đó `{chargePointId}` là ID duy nhất của charge point.

### Kiểm tra trạng thái

Truy cập `http://localhost:9001/status` để xem:
- Thời gian server
- Số lượng charge points đang kết nối
- Trạng thái của từng charge point

### Gửi lệnh đến Charge Point

```go
// Lấy charge point
cp := server.GetChargePoint("chargePointId")

// Thay đổi availability
cp.ChangeAvailability(1, "Operative")

// Lấy configuration
cp.GetConfiguration([]string{"HeartbeatInterval"})

// Thay đổi configuration
cp.ChangeConfiguration("HeartbeatInterval", "30")

// Remote start transaction
cp.RemoteStartTransaction(1, "CARD001")

// Remote stop transaction
cp.RemoteStopTransaction(12345)

// Reset charge point
cp.Reset("Soft")

// Unlock connector
cp.UnlockConnector(1)
```

## Cấu trúc dữ liệu

### ChargePoint
```go
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
    mu              sync.RWMutex
}
```

### Connector
```go
type Connector struct {
    ID           int
    Status       string
    Availability string
    Transaction  *Transaction
}
```

### Transaction
```go
type Transaction struct {
    ID            int
    ConnectorID   int
    IdTag         string
    StartTime     time.Time
    StopTime      *time.Time
    MeterStart    int
    MeterStop     *int
    Reason        string
    Status        string
}
```

## Logging

Server sẽ log tất cả các message OCPP với format:
```
[ChargePointID] OCPP: type=2, id=123, action=BootNotification, payload={...}
[ChargePointID] Sending: [3, "123", {...}]
```

## Ví dụ sử dụng với Charge Point

### Boot Notification
```json
[2, "123", "BootNotification", {
  "chargePointVendor": "VendorName",
  "chargePointModel": "ModelName",
  "chargePointSerialNumber": "SN123456",
  "firmwareVersion": "1.0.0"
}]
```

### Heartbeat
```json
[2, "124", "Heartbeat", {}]
```

### Status Notification
```json
[2, "125", "StatusNotification", {
  "connectorId": 1,
  "errorCode": "NoError",
  "status": "Available"
}]
```

### Start Transaction
```json
[2, "126", "StartTransaction", {
  "connectorId": 1,
  "idTag": "CARD001",
  "meterStart": 1000
}]
```

## Bảo mật

⚠️ **Lưu ý**: Server này được thiết kế cho mục đích development và testing. Trong môi trường production, bạn nên:

- Thêm authentication và authorization
- Sử dụng TLS/SSL cho WebSocket connections
- Validate và sanitize tất cả input
- Implement rate limiting
- Log security events
- Sử dụng environment variables cho configuration

## Đóng góp

Mọi đóng góp đều được chào đón! Hãy tạo issue hoặc pull request nếu bạn muốn cải thiện code này.

## License

MIT License
