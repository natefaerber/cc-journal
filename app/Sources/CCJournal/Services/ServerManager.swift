import AppKit
import Foundation
import os.log

private let logger = Logger(subsystem: "com.ccjournal.app", category: "ServerManager")

enum ServerState: Equatable {
    case stopped
    case starting
    case running
    case error(String)

    var label: String {
        switch self {
        case .stopped: "Stopped"
        case .starting: "Starting..."
        case .running: "Running"
        case .error(let msg): "Error: \(msg)"
        }
    }

    var isRunning: Bool {
        self == .running
    }
}

@Observable
final class ServerManager {
    var state: ServerState = .stopped
    var port: Int

    private var process: Process?
    private var healthCheckTimer: Timer?
    private let binaryPath: String

    init(binaryPath: String, port: Int = 8000) {
        self.binaryPath = binaryPath
        self.port = port
        logger.info("ServerManager initialized (binary: \(binaryPath), port: \(port))")

        // Stop server on app termination
        NotificationCenter.default.addObserver(
            forName: NSApplication.willTerminateNotification,
            object: nil,
            queue: .main
        ) { [weak self] _ in
            logger.info("App terminating — stopping server")
            self?.stopServer()
        }
    }

    deinit {
        stopServer()
    }

    func startServer() {
        guard state != .running && state != .starting else { return }
        logger.info("Starting server on port \(self.port)")
        state = .starting

        let proc = Process()
        proc.executableURL = URL(fileURLWithPath: binaryPath)
        proc.arguments = ["serve", "--port", "\(port)"]

        // Pipe server output to a log file
        let logDir = FileManager.default.homeDirectoryForCurrentUser
            .appendingPathComponent(".config/cc-journal/logs")
        try? FileManager.default.createDirectory(at: logDir, withIntermediateDirectories: true)
        let logFile = logDir.appendingPathComponent("server.log")
        if let handle = try? FileHandle(forWritingTo: logFile) {
            handle.seekToEndOfFile()
            proc.standardOutput = handle
            proc.standardError = handle
            logger.info("Server output logging to \(logFile.path)")
        } else {
            FileManager.default.createFile(atPath: logFile.path, contents: nil)
            if let handle = try? FileHandle(forWritingTo: logFile) {
                proc.standardOutput = handle
                proc.standardError = handle
                logger.info("Server output logging to \(logFile.path)")
            } else {
                logger.warning("Could not create server log file, output will be discarded")
                proc.standardOutput = FileHandle.nullDevice
                proc.standardError = FileHandle.nullDevice
            }
        }

        proc.terminationHandler = { [weak self] process in
            DispatchQueue.main.async {
                guard let self, self.process === process else { return }
                let code = process.terminationStatus
                if self.state == .running || self.state == .starting {
                    logger.error("Server exited unexpectedly with code \(code)")
                    self.state = .error("Server exited with code \(code)")
                } else {
                    logger.info("Server process ended (code \(code))")
                }
                self.stopHealthCheck()
                self.process = nil
            }
        }

        do {
            try proc.run()
            process = proc
            logger.info("Server process started (PID \(proc.processIdentifier))")
            // Give the server a moment to start, then begin health checks
            DispatchQueue.main.asyncAfter(deadline: .now() + 1.0) { [weak self] in
                self?.checkHealth()
                self?.startHealthCheck()
            }
        } catch {
            logger.error("Failed to start server: \(error.localizedDescription)")
            state = .error(error.localizedDescription)
        }
    }

    func stopServer() {
        stopHealthCheck()

        guard let proc = process, proc.isRunning else {
            process = nil
            state = .stopped
            return
        }

        let pid = proc.processIdentifier
        logger.info("Stopping server (PID \(pid)) — sending SIGTERM")
        proc.terminate()

        // SIGKILL after 3s if still running
        DispatchQueue.global().asyncAfter(deadline: .now() + 3.0) { [weak self] in
            guard let self, let proc = self.process, proc.isRunning else { return }
            logger.warning("Server still running after 3s — sending SIGKILL (PID \(pid))")
            kill(proc.processIdentifier, SIGKILL)
        }

        process = nil
        state = .stopped
    }

    func openDashboard(path: String = "") {
        if !state.isRunning && state != .starting {
            startServer()
        }

        // Small delay if we just started the server
        let delay: Double = state == .starting ? 2.0 : 0.0
        DispatchQueue.main.asyncAfter(deadline: .now() + delay) { [self] in
            if let url = URL(string: "http://localhost:\(port)\(path)") {
                NSWorkspace.shared.open(url)
            }
        }
    }

    func toggleServer() {
        if state.isRunning || state == .starting {
            stopServer()
        } else {
            startServer()
        }
    }

    // MARK: - Health Check

    private func startHealthCheck() {
        healthCheckTimer = Timer.scheduledTimer(withTimeInterval: 30.0, repeats: true) { [weak self] _ in
            self?.checkHealth()
        }
    }

    private func stopHealthCheck() {
        healthCheckTimer?.invalidate()
        healthCheckTimer = nil
    }

    private func checkHealth() {
        guard let url = URL(string: "http://localhost:\(port)/") else { return }

        let task = URLSession.shared.dataTask(with: url) { [weak self] _, response, error in
            DispatchQueue.main.async {
                guard let self else { return }
                if let http = response as? HTTPURLResponse, http.statusCode == 200 {
                    self.state = .running
                } else if let error {
                    // Only mark error if we expected it to be running
                    if self.state == .running {
                        self.state = .error(error.localizedDescription)
                    }
                }
            }
        }
        task.resume()
    }
}
