import Foundation

class WebSocketClient {
    private let url: URL
    private var webSocketTask: URLSessionWebSocketTask?
    private var session: URLSession?
    private var isConnected = false
    private var reconnectTimer: Timer?

    private let reconnectInterval: TimeInterval = 5.0

    // Callbacks
    var onStatusChange: ((SystemStatus) -> Void)?
    var onTaskCountChange: ((Int) -> Void)?
    var onNotification: ((String, String) -> Void)?

    init(host: String = "localhost", port: Int = 4001) {
        self.url = URL(string: "ws://\(host):\(port)/shell")!
    }

    // MARK: - Connection

    func connect() {
        let session = URLSession(configuration: .default)
        self.session = session

        let task = session.webSocketTask(with: url)
        self.webSocketTask = task
        task.resume()

        isConnected = true
        listenForMessages()
        onStatusChange?(.healthy)

        stopReconnectTimer()
    }

    func disconnect() {
        stopReconnectTimer()
        isConnected = false
        webSocketTask?.cancel(with: .goingAway, reason: nil)
        webSocketTask = nil
        session?.invalidateAndCancel()
        session = nil
    }

    // MARK: - Sending

    func send(action: String, payload: [String: Any]? = nil) {
        var message: [String: Any] = ["action": action]
        if let payload = payload {
            message["payload"] = payload
        }

        guard let data = try? JSONSerialization.data(withJSONObject: message),
              let jsonString = String(data: data, encoding: .utf8) else {
            return
        }

        webSocketTask?.send(.string(jsonString)) { error in
            if let error = error {
                print("[FortShell] Send error: \(error.localizedDescription)")
            }
        }
    }

    // MARK: - Receiving

    private func listenForMessages() {
        webSocketTask?.receive { [weak self] result in
            guard let self = self else { return }

            switch result {
            case .success(let message):
                switch message {
                case .string(let text):
                    self.handleMessage(text)
                case .data(let data):
                    if let text = String(data: data, encoding: .utf8) {
                        self.handleMessage(text)
                    }
                @unknown default:
                    break
                }
                // Continue listening
                self.listenForMessages()

            case .failure(let error):
                print("[FortShell] WebSocket error: \(error.localizedDescription)")
                DispatchQueue.main.async {
                    self.onStatusChange?(.disconnected)
                    self.scheduleReconnect()
                }
            }
        }
    }

    private func handleMessage(_ text: String) {
        guard let data = text.data(using: .utf8),
              let json = try? JSONSerialization.jsonObject(with: data) as? [String: Any],
              let type = json["type"] as? String else {
            return
        }

        switch type {
        case "status":
            if let statusString = json["status"] as? String,
               let status = SystemStatus(rawValue: statusString) {
                DispatchQueue.main.async {
                    self.onStatusChange?(status)
                }
            }

        case "tasks":
            if let count = json["count"] as? Int {
                DispatchQueue.main.async {
                    self.onTaskCountChange?(count)
                }
            }

        case "notification":
            let title = json["title"] as? String ?? "Fort"
            let body = json["body"] as? String ?? ""
            self.onNotification?(title, body)

        default:
            break
        }
    }

    // MARK: - Reconnection

    private func scheduleReconnect() {
        stopReconnectTimer()
        DispatchQueue.main.async {
            self.reconnectTimer = Timer.scheduledTimer(
                withTimeInterval: self.reconnectInterval,
                repeats: true
            ) { [weak self] _ in
                guard let self = self else { return }
                print("[FortShell] Attempting reconnect...")
                self.connect()
            }
        }
    }

    private func stopReconnectTimer() {
        reconnectTimer?.invalidate()
        reconnectTimer = nil
    }
}
