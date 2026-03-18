import Foundation
import os.log

private let logger = Logger(subsystem: "com.ccjournal.app", category: "CLIBridge")

struct CLIBridge: Sendable {
    private let binaryPath: String

    init(binaryPath: String? = nil) {
        let resolved = binaryPath ?? Self.findBinary()
        self.binaryPath = resolved
        logger.info("CLIBridge initialized with binary: \(resolved)")
    }

    func fetchStats() async throws -> StatsResponse {
        try await run(["stats", "--json"])
    }

    func fetchToday() async throws -> EntriesResponse {
        try await run(["today", "--json"])
    }

    func fetchEntries(date: String) async throws -> EntriesResponse {
        try await run(["show", date, "--json"])
    }

    func fetchFileList() async throws -> FileListResponse {
        try await run(["list", "--json"])
    }

    func fetchVersion() async throws -> String {
        let data = try await runRaw(["--version"])
        return String(data: data, encoding: .utf8)?.trimmingCharacters(in: .whitespacesAndNewlines) ?? "unknown"
    }

    func copyStandup() async throws {
        _ = try await runRaw(["standup", "--copy"])
    }

    func copyWeekly() async throws {
        _ = try await runRaw(["weekly", "--copy"])
    }

    // MARK: - Private

    private func run<T: Decodable>(_ arguments: [String]) async throws -> T {
        let data = try await runRaw(arguments)
        return try JSONDecoder().decode(T.self, from: data)
    }

    private func runRaw(_ arguments: [String]) async throws -> Data {
        try await withCheckedThrowingContinuation { continuation in
            DispatchQueue.global(qos: .userInitiated).async {
                let argStr = arguments.joined(separator: " ")
                logger.debug("Running: cc-journal \(argStr)")
                do {
                    let process = Process()
                    process.executableURL = URL(fileURLWithPath: binaryPath)
                    process.arguments = arguments

                    let pipe = Pipe()
                    let errorPipe = Pipe()
                    process.standardOutput = pipe
                    process.standardError = errorPipe

                    try process.run()
                    process.waitUntilExit()

                    guard process.terminationStatus == 0 else {
                        let stderr = String(data: errorPipe.fileHandleForReading.readDataToEndOfFile(), encoding: .utf8) ?? "Unknown error"
                        logger.error("cc-journal \(argStr) failed (exit \(process.terminationStatus)): \(stderr)")
                        continuation.resume(throwing: CLIError.nonZeroExit(Int(process.terminationStatus), stderr))
                        return
                    }

                    let data = pipe.fileHandleForReading.readDataToEndOfFile()
                    logger.debug("cc-journal \(argStr) succeeded (\(data.count) bytes)")
                    continuation.resume(returning: data)
                } catch {
                    logger.error("Failed to launch cc-journal: \(error.localizedDescription)")
                    continuation.resume(throwing: error)
                }
            }
        }
    }

    /// Resolved path to the cc-journal binary, for sharing with ServerManager.
    static var resolvedBinaryPath: String { findBinary() }

    private static func findBinary() -> String {
        // User-configured path takes priority
        if let userPath = UserDefaults.standard.string(forKey: "binaryPath"),
           !userPath.isEmpty,
           FileManager.default.isExecutableFile(atPath: userPath) {
            logger.info("Using user-configured binary: \(userPath)")
            return userPath
        }

        let home = FileManager.default.homeDirectoryForCurrentUser.path

        // Check well-known locations including mise install paths
        // Order matters: prefer mise dev build, then shims, then homebrew
        let candidates = [
            "\(home)/.local/share/mise/installs/github-natefaerber-cc-journal/dev/cc-journal",
            "\(home)/.local/share/mise/shims/cc-journal",
            "/opt/homebrew/bin/cc-journal",
            "/usr/local/bin/cc-journal",
        ]

        for candidate in candidates {
            if FileManager.default.isExecutableFile(atPath: candidate) {
                logger.info("Found cc-journal at: \(candidate)")
                return candidate
            }
        }

        // Check PATH last (macOS apps have minimal PATH)
        if let pathBinary = findInPath("cc-journal") {
            logger.info("Found cc-journal in PATH: \(pathBinary)")
            return pathBinary
        }

        logger.warning("cc-journal binary not found — commands will fail")
        return "cc-journal" // fallback — will fail with a clear error
    }

    private static func findInPath(_ binary: String) -> String? {
        guard let path = ProcessInfo.processInfo.environment["PATH"] else { return nil }
        for dir in path.split(separator: ":") {
            let full = "\(dir)/\(binary)"
            if FileManager.default.isExecutableFile(atPath: full) {
                return full
            }
        }
        return nil
    }
}

enum CLIError: LocalizedError {
    case nonZeroExit(Int, String)

    var errorDescription: String? {
        switch self {
        case .nonZeroExit(let code, let message):
            return "cc-journal exited with code \(code): \(message)"
        }
    }
}
